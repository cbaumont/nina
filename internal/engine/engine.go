package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/runner"
	"github.com/cbaumont/nina/internal/state"
	"github.com/cbaumont/nina/internal/workspace"
)

type State string

const (
	StateIdle     State = "idle"
	StateScaffold State = "scaffold"
	StateDrive    State = "drive"
	StateDone     State = "done"
)

type EventKind string

const (
	EventTextDelta   EventKind = "text_delta"
	EventInfo        EventKind = "info"
	EventPlanSet     EventKind = "plan_set"
	EventStepStarted EventKind = "step_started"
	EventReview      EventKind = "review"
	EventSessionDone EventKind = "session_done"
	EventConfirm     EventKind = "confirm"
	EventCommandRun  EventKind = "command_run"
)

type Event struct {
	Kind    EventKind
	Text    string
	Step    int
	Plan    *Plan
	Verdict string
	Confirm *ConfirmRequest
}

// ConfirmRequest asks the user to approve a command the model proposed.
// The engine blocks until an answer is sent on Reply.
type ConfirmRequest struct {
	Command string
	Reason  string
	Reply   chan ConfirmAnswer
}

type ConfirmAnswer struct {
	Approve bool
	Always  bool
}

type Plan struct {
	Title string
	Steps []llm.PlanStep
}

const DialLevel = 1

const (
	commandTimeout   = 2 * time.Minute
	maxReadFileBytes = 32 * 1024
)

type Engine struct {
	client      llm.Client
	ws          *workspace.Workspace
	dir         string
	emit        func(Event)
	conv        *llm.Conversation
	sessionID   string
	goal        string
	state       State
	plan        Plan
	stepIndex   int
	snapshots   int
	lastRef     string
	review      *llm.SubmitReviewInput
	autoApprove map[string]bool
}

func New(client llm.Client, ws *workspace.Workspace, dir string, emit func(Event)) *Engine {
	return &Engine{
		client:      client,
		ws:          ws,
		dir:         dir,
		emit:        emit,
		sessionID:   time.Now().Format("20060102-150405"),
		state:       StateIdle,
		conv:        &llm.Conversation{System: systemPrompt()},
		autoApprove: map[string]bool{},
	}
}

func (e *Engine) State() State   { return e.state }
func (e *Engine) Plan() Plan     { return e.plan }
func (e *Engine) StepIndex() int { return e.stepIndex }
func (e *Engine) Goal() string   { return e.goal }

// Restore rebuilds the engine from a saved session so it can continue
// where it left off. Call before any other engine method.
func (e *Engine) Restore(sess *state.Session, messages []llm.Message) {
	e.sessionID = sess.SessionID
	e.goal = sess.Goal
	e.state = State(sess.State)
	e.plan = Plan{Title: sess.PlanTitle, Steps: sess.Steps}
	e.stepIndex = sess.StepIndex
	e.snapshots = sess.Snapshots
	e.lastRef = sess.LastRef
	e.conv.Messages = messages
}

// persist saves the session and transcript to .nina/; failures are
// reported to the user but never interrupt the session.
func (e *Engine) persist() {
	sess := &state.Session{
		SessionID: e.sessionID,
		Goal:      e.goal,
		State:     string(e.state),
		PlanTitle: e.plan.Title,
		Steps:     e.plan.Steps,
		StepIndex: e.stepIndex,
		Snapshots: e.snapshots,
		LastRef:   e.lastRef,
	}
	if err := state.Save(e.dir, sess, e.conv.Messages); err != nil {
		e.emit(Event{Kind: EventInfo, Text: "Warning: could not save session state: " + err.Error()})
	}
}

func (e *Engine) Start(ctx context.Context, goal string) error {
	if e.state != StateIdle {
		return fmt.Errorf("session already started")
	}
	e.state = StateScaffold
	e.goal = goal
	e.conv.AddUser(startPrompt(goal))
	if _, err := e.converseLoop(ctx); err != nil {
		e.state = StateIdle
		return err
	}
	if len(e.plan.Steps) == 0 {
		e.state = StateIdle
		return fmt.Errorf("the model did not produce a task plan; try rephrasing the goal")
	}
	if err := e.snapshot(); err != nil {
		return err
	}
	e.state = StateDrive
	e.persist()
	e.emit(Event{Kind: EventStepStarted, Step: e.stepIndex})
	return nil
}

func (e *Engine) Done(ctx context.Context) error {
	if e.state != StateDrive {
		return fmt.Errorf("no step in progress")
	}
	previousRef := e.lastRef
	if err := e.snapshot(); err != nil {
		return err
	}
	diff, err := e.ws.Diff(previousRef, e.lastRef)
	if err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		e.emit(Event{Kind: EventInfo, Text: "No changes since the last snapshot — edit the files in your editor, then /done."})
		return nil
	}

	e.review = nil
	step := e.plan.Steps[e.stepIndex]
	e.conv.AddUser(reviewPrompt(step, diff))
	if _, err := e.converseLoop(ctx); err != nil {
		return err
	}
	if e.review == nil {
		e.emit(Event{Kind: EventInfo, Text: "The navigator did not submit a verdict; treating this step as still in progress."})
		return nil
	}
	e.emit(Event{Kind: EventReview, Verdict: e.review.Verdict, Text: e.review.Feedback, Step: e.stepIndex})
	if e.review.Verdict != "pass" {
		e.persist()
		return nil
	}

	e.stepIndex++
	if e.stepIndex >= len(e.plan.Steps) {
		e.state = StateDone
		e.persist()
		e.emit(Event{Kind: EventSessionDone})
		return nil
	}
	e.conv.AddUser(instructPrompt(e.stepIndex, e.plan.Steps[e.stepIndex]))
	if _, err := e.converseLoop(ctx); err != nil {
		return err
	}
	e.persist()
	e.emit(Event{Kind: EventStepStarted, Step: e.stepIndex})
	return nil
}

func (e *Engine) UserMessage(ctx context.Context, text string) error {
	if e.state == StateIdle {
		return fmt.Errorf("session not started")
	}
	e.conv.AddUser(text)
	_, err := e.converseLoop(ctx)
	if err == nil {
		e.persist()
	}
	return err
}

func (e *Engine) converseLoop(ctx context.Context) (llm.Turn, error) {
	for {
		turn, err := e.client.Converse(ctx, e.conv, func(text string) {
			e.emit(Event{Kind: EventTextDelta, Text: text})
		})
		if err != nil {
			return llm.Turn{}, err
		}
		if len(turn.ToolCalls) == 0 {
			return turn, nil
		}
		results := make([]llm.ToolResult, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			results = append(results, e.execTool(ctx, call))
		}
		e.conv.AddToolResults(results)
	}
}

func (e *Engine) execTool(ctx context.Context, call llm.ToolCall) llm.ToolResult {
	switch call.Name {
	case llm.ToolWriteFile:
		return e.execWriteFile(call)
	case llm.ToolSetPlan:
		return e.execSetPlan(call)
	case llm.ToolSubmitReview:
		return e.execSubmitReview(call)
	case llm.ToolRunCommand:
		return e.execRunCommand(ctx, call)
	case llm.ToolReadFile:
		return e.execReadFile(call)
	default:
		return llm.ToolResult{ToolCallID: call.ID, Content: fmt.Sprintf("unknown tool %q", call.Name), IsError: true}
	}
}

func (e *Engine) execRunCommand(ctx context.Context, call llm.ToolCall) llm.ToolResult {
	var input llm.RunCommandInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: "invalid run_command input: " + err.Error(), IsError: true}
	}
	if strings.TrimSpace(input.Command) == "" {
		return llm.ToolResult{ToolCallID: call.ID, Content: "run_command needs a non-empty command", IsError: true}
	}
	if !e.autoApprove[input.Command] {
		reply := make(chan ConfirmAnswer, 1)
		e.emit(Event{Kind: EventConfirm, Confirm: &ConfirmRequest{Command: input.Command, Reason: input.Reason, Reply: reply}})
		var answer ConfirmAnswer
		select {
		case answer = <-reply:
		case <-ctx.Done():
			return llm.ToolResult{ToolCallID: call.ID, Content: "cancelled", IsError: true}
		}
		if answer.Always && answer.Approve {
			e.autoApprove[input.Command] = true
		}
		if !answer.Approve {
			return llm.ToolResult{ToolCallID: call.ID, Content: "The learner declined to run this command. Continue without it, or propose an alternative and explain why it is needed."}
		}
	}
	result, err := runner.Run(ctx, e.dir, input.Command, commandTimeout)
	if err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	e.emit(Event{Kind: EventCommandRun, Text: commandOutputMarkdown(input.Command, result)})
	return llm.ToolResult{ToolCallID: call.ID, Content: commandToolContent(result)}
}

func commandOutputMarkdown(command string, result runner.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "```console\n$ %s\n", command)
	if result.Output != "" {
		b.WriteString(result.Output + "\n")
	}
	b.WriteString("```\n")
	switch {
	case result.TimedOut:
		fmt.Fprintf(&b, "*timed out after %s and was killed*", commandTimeout)
	case result.ExitCode == 0:
		b.WriteString("*exit code 0*")
	default:
		fmt.Fprintf(&b, "*exit code %d*", result.ExitCode)
	}
	return b.String()
}

func commandToolContent(result runner.Result) string {
	content := fmt.Sprintf("exit code: %d", result.ExitCode)
	if result.TimedOut {
		content = fmt.Sprintf("command timed out after %s and was killed\n%s", commandTimeout, content)
	}
	if result.Output == "" {
		return content + "\n(no output)"
	}
	return content + "\n" + result.Output
}

func (e *Engine) execReadFile(call llm.ToolCall) llm.ToolResult {
	var input llm.ReadFileInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: "invalid read_file input: " + err.Error(), IsError: true}
	}
	path, err := e.safePath(input.Path)
	if err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	if len(data) > maxReadFileBytes {
		return llm.ToolResult{ToolCallID: call.ID, Content: string(data[:maxReadFileBytes]) + "\n[file truncated]"}
	}
	return llm.ToolResult{ToolCallID: call.ID, Content: string(data)}
}

func (e *Engine) execWriteFile(call llm.ToolCall) llm.ToolResult {
	if e.state != StateScaffold {
		return llm.ToolResult{
			ToolCallID: call.ID,
			Content:    "Denied by the typing dial policy: at dial level 1 you may only write files while scaffolding the project. Instruct the learner to write this code themselves.",
			IsError:    true,
		}
	}
	var input llm.WriteFileInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: "invalid write_file input: " + err.Error(), IsError: true}
	}
	path, err := e.safePath(input.Path)
	if err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	e.emit(Event{Kind: EventInfo, Text: "Nina wrote `" + input.Path + "`"})
	return llm.ToolResult{ToolCallID: call.ID, Content: "wrote " + input.Path}
}

func (e *Engine) execSetPlan(call llm.ToolCall) llm.ToolResult {
	var input llm.SetPlanInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: "invalid set_plan input: " + err.Error(), IsError: true}
	}
	if len(input.Steps) == 0 {
		return llm.ToolResult{ToolCallID: call.ID, Content: "a plan needs at least one step", IsError: true}
	}
	e.plan = Plan{Title: input.Title, Steps: input.Steps}
	e.emit(Event{Kind: EventPlanSet, Plan: &e.plan})
	return llm.ToolResult{ToolCallID: call.ID, Content: fmt.Sprintf("plan set with %d steps", len(input.Steps))}
}

func (e *Engine) execSubmitReview(call llm.ToolCall) llm.ToolResult {
	var input llm.SubmitReviewInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return llm.ToolResult{ToolCallID: call.ID, Content: "invalid submit_review input: " + err.Error(), IsError: true}
	}
	if input.Verdict != "pass" && input.Verdict != "retry" {
		return llm.ToolResult{ToolCallID: call.ID, Content: "verdict must be pass or retry", IsError: true}
	}
	e.review = &input
	return llm.ToolResult{ToolCallID: call.ID, Content: "verdict recorded"}
}

func (e *Engine) safePath(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative to the workspace: %q", rel)
	}
	root := filepath.Clean(e.dir)
	path := filepath.Join(root, rel)
	if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes the workspace: %q", rel)
	}
	return path, nil
}

func (e *Engine) snapshot() error {
	ref := workspace.SnapshotRef(e.sessionID, e.snapshots)
	if _, err := e.ws.Snapshot(ref); err != nil {
		return err
	}
	e.lastRef = ref
	e.snapshots++
	return nil
}
