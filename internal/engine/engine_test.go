package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/profile"
	"github.com/cbaumont/nina/internal/state"
	"github.com/cbaumont/nina/internal/workspace"
)

type fakeClient struct {
	turns []llm.Turn
	calls int
}

func (f *fakeClient) Converse(_ context.Context, _ *llm.Conversation, _ func(string)) (llm.Turn, error) {
	if f.calls >= len(f.turns) {
		return llm.Turn{Text: "ok", StopReason: "end_turn"}, nil
	}
	turn := f.turns[f.calls]
	f.calls++
	return turn, nil
}

func toolCall(t *testing.T, name string, input any) llm.ToolCall {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return llm.ToolCall{ID: "toolu_" + name, Name: name, Input: raw}
}

func planCall(t *testing.T) llm.ToolCall {
	return toolCall(t, llm.ToolSetPlan, llm.SetPlanInput{
		Title: "Guessing Game",
		Steps: []llm.PlanStep{
			{Title: "Read input", Goal: "Program reads a number from the user"},
			{Title: "Compare", Goal: "Program says higher or lower"},
		},
	})
}

func newTestEngine(t *testing.T, turns []llm.Turn) (*Engine, string, *[]Event) {
	t.Helper()
	dir := t.TempDir()
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	events := &[]Event{}
	eng := New(&fakeClient{turns: turns}, ws, dir, profile.Default(), func(ev Event) {
		*events = append(*events, ev)
	})
	return eng, dir, events
}

func startedEngine(t *testing.T, extraTurns []llm.Turn) (*Engine, string, *[]Event) {
	t.Helper()
	turns := append([]llm.Turn{
		{ToolCalls: []llm.ToolCall{
			planCall(t),
			toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: "main.py", Content: "# stub\n"}),
		}, StopReason: "tool_use"},
		{Text: "Step 1: read input.", StopReason: "end_turn"},
	}, extraTurns...)
	eng, dir, events := newTestEngine(t, turns)
	if err := eng.Start(context.Background(), "learn python"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return eng, dir, events
}

func TestStartScaffoldsAndPlans(t *testing.T) {
	eng, dir, events := startedEngine(t, nil)

	if eng.State() != StateDrive {
		t.Errorf("state = %s", eng.State())
	}
	if eng.Plan().Title != "Guessing Game" || len(eng.Plan().Steps) != 2 {
		t.Errorf("plan = %+v", eng.Plan())
	}
	if _, err := os.Stat(filepath.Join(dir, "main.py")); err != nil {
		t.Errorf("scaffold file not written: %v", err)
	}
	kinds := map[EventKind]bool{}
	for _, ev := range *events {
		kinds[ev.Kind] = true
	}
	for _, want := range []EventKind{EventPlanSet, EventInfo, EventStepStarted} {
		if !kinds[want] {
			t.Errorf("missing event %s", want)
		}
	}
}

func TestDialRejectsWritesAfterScaffold(t *testing.T) {
	eng, dir, _ := startedEngine(t, nil)

	result := eng.execTool(context.Background(), toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: "solution.py", Content: "answer"}))
	if !result.IsError {
		t.Fatal("write_file allowed outside scaffold state")
	}
	if _, err := os.Stat(filepath.Join(dir, "solution.py")); !os.IsNotExist(err) {
		t.Error("file was created despite dial rejection")
	}
}

func TestDialPolicyMatrix(t *testing.T) {
	cases := []struct {
		dial    int
		state   State
		allowed bool
	}{
		{0, StateScaffold, false},
		{0, StateDrive, false},
		{1, StateScaffold, true},
		{1, StateDrive, false},
		{2, StateScaffold, true},
		{2, StateDrive, true},
		{3, StateDrive, true},
	}
	for _, tc := range cases {
		eng, dir, _ := newTestEngine(t, nil)
		eng.profile.Dial = tc.dial
		eng.state = tc.state
		result := eng.execTool(context.Background(), toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: "f.py", Content: "x"}))
		if got := !result.IsError; got != tc.allowed {
			t.Errorf("dial %d in %s: allowed = %v, want %v", tc.dial, tc.state, got, tc.allowed)
		}
		if _, err := os.Stat(filepath.Join(dir, "f.py")); (err == nil) != tc.allowed {
			t.Errorf("dial %d in %s: file existence mismatch", tc.dial, tc.state)
		}
	}
}

func TestUpdateProfileRebuildsSystemPrompt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	eng, _, _ := startedEngine(t, nil)
	prof := eng.Profile()
	prof.Dial = 0
	if err := eng.UpdateProfile(prof); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(eng.conv.System, "level 0") {
		t.Error("system prompt not rebuilt after profile change")
	}
	result := eng.execTool(context.Background(), toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: "f.py", Content: "x"}))
	if !result.IsError {
		t.Error("write allowed at dial 0 after update")
	}
}

func TestUpdatePlanReplacesRemainingSteps(t *testing.T) {
	eng, _, events := startedEngine(t, nil)

	result := eng.execTool(context.Background(), toolCall(t, llm.ToolUpdatePlan, llm.UpdatePlanInput{
		Steps: []llm.PlanStep{{Title: "New step 2", Goal: "different goal"}, {Title: "New step 3", Goal: "extra"}},
	}))
	if result.IsError {
		t.Fatalf("result = %+v", result)
	}
	steps := eng.Plan().Steps
	if len(steps) != 3 || steps[0].Title != "Read input" || steps[1].Title != "New step 2" {
		t.Errorf("steps = %+v", steps)
	}
	last := (*events)[len(*events)-1]
	if last.Kind != EventPlanSet {
		t.Errorf("expected plan_set event, got %+v", last)
	}
}

func TestSkipAdvancesWithoutReview(t *testing.T) {
	eng, dir, events := startedEngine(t, []llm.Turn{
		{Text: "Step 2 instructions.", StopReason: "end_turn"},
	})
	writeWorkspaceFile(t, dir, "main.py", "half-finished\n")

	if err := eng.Skip(context.Background()); err != nil {
		t.Fatal(err)
	}
	if eng.StepIndex() != 1 || eng.State() != StateDrive {
		t.Errorf("step = %d, state = %s", eng.StepIndex(), eng.State())
	}
	for _, ev := range *events {
		if ev.Kind == EventReview {
			t.Error("skip must not produce a review")
		}
	}
	// The skipped work is the new baseline: an immediate /done sees no diff.
	if err := eng.Done(context.Background()); err != nil {
		t.Fatal(err)
	}
	if eng.StepIndex() != 1 {
		t.Errorf("baseline not reset; step = %d", eng.StepIndex())
	}
}

func TestWriteFileRejectsEscapingPaths(t *testing.T) {
	eng, _, _ := newTestEngine(t, nil)
	eng.state = StateScaffold
	for _, path := range []string{"../evil.txt", "/etc/passwd", ""} {
		result := eng.execTool(context.Background(), toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: path, Content: "x"}))
		if !result.IsError {
			t.Errorf("path %q accepted", path)
		}
	}
}

func TestDonePassAdvancesStep(t *testing.T) {
	eng, dir, events := startedEngine(t, []llm.Turn{
		{ToolCalls: []llm.ToolCall{
			toolCall(t, llm.ToolSubmitReview, llm.SubmitReviewInput{Verdict: "pass", Feedback: "Nice use of input()."}),
		}, StopReason: "tool_use"},
		{Text: "Verdict recorded.", StopReason: "end_turn"},
		{Text: "Step 2: compare the numbers.", StopReason: "end_turn"},
	})
	writeWorkspaceFile(t, dir, "main.py", "n = int(input())\n")

	if err := eng.Done(context.Background()); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if eng.StepIndex() != 1 {
		t.Errorf("step index = %d", eng.StepIndex())
	}
	if verdict := lastReview(events); verdict != "pass" {
		t.Errorf("review verdict = %q", verdict)
	}
}

func TestDoneRetryKeepsStep(t *testing.T) {
	eng, dir, events := startedEngine(t, []llm.Turn{
		{ToolCalls: []llm.ToolCall{
			toolCall(t, llm.ToolSubmitReview, llm.SubmitReviewInput{Verdict: "retry", Feedback: "What happens if the input is not a number?"}),
		}, StopReason: "tool_use"},
		{Text: "Verdict recorded.", StopReason: "end_turn"},
	})
	writeWorkspaceFile(t, dir, "main.py", "n = input()\n")

	if err := eng.Done(context.Background()); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if eng.StepIndex() != 0 {
		t.Errorf("step index = %d", eng.StepIndex())
	}
	if verdict := lastReview(events); verdict != "retry" {
		t.Errorf("review verdict = %q", verdict)
	}
}

func TestDoneWithNoChanges(t *testing.T) {
	eng, _, events := startedEngine(t, nil)

	if err := eng.Done(context.Background()); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if eng.StepIndex() != 0 {
		t.Errorf("step index = %d", eng.StepIndex())
	}
	last := (*events)[len(*events)-1]
	if last.Kind != EventInfo {
		t.Errorf("expected info event, got %+v", last)
	}
}

func TestSessionCompletesAfterLastStep(t *testing.T) {
	passTurns := []llm.Turn{
		{ToolCalls: []llm.ToolCall{
			toolCall(t, llm.ToolSubmitReview, llm.SubmitReviewInput{Verdict: "pass", Feedback: "Good."}),
		}, StopReason: "tool_use"},
		{Text: "Verdict recorded.", StopReason: "end_turn"},
		{Text: "Step 2 instructions.", StopReason: "end_turn"},
		{ToolCalls: []llm.ToolCall{
			toolCall(t, llm.ToolSubmitReview, llm.SubmitReviewInput{Verdict: "pass", Feedback: "Done!"}),
		}, StopReason: "tool_use"},
		{Text: "Verdict recorded.", StopReason: "end_turn"},
	}
	eng, dir, events := startedEngine(t, passTurns)

	writeWorkspaceFile(t, dir, "main.py", "step one\n")
	if err := eng.Done(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, dir, "main.py", "step two\n")
	if err := eng.Done(context.Background()); err != nil {
		t.Fatal(err)
	}

	if eng.State() != StateDone {
		t.Errorf("state = %s", eng.State())
	}
	found := false
	for _, ev := range *events {
		if ev.Kind == EventSessionDone {
			found = true
		}
	}
	if !found {
		t.Error("missing session_done event")
	}
}

func TestPersistAndRestoreContinuesSession(t *testing.T) {
	eng, dir, _ := startedEngine(t, nil)
	sess, messages, err := state.Load(dir)
	if err != nil || sess == nil {
		t.Fatalf("no session persisted after Start: %+v, %v", sess, err)
	}
	if sess.State != string(StateDrive) || sess.Goal != "learn python" || len(messages) == 0 {
		t.Errorf("session = %+v, messages = %d", sess, len(messages))
	}

	// A fresh engine in the same workspace picks up where the first left off.
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	events := &[]Event{}
	restored := New(&fakeClient{turns: []llm.Turn{
		{ToolCalls: []llm.ToolCall{
			toolCall(t, llm.ToolSubmitReview, llm.SubmitReviewInput{Verdict: "pass", Feedback: "Nice."}),
		}, StopReason: "tool_use"},
		{Text: "Verdict recorded.", StopReason: "end_turn"},
		{Text: "Step 2 instructions.", StopReason: "end_turn"},
	}}, ws, dir, profile.Default(), func(ev Event) { *events = append(*events, ev) })
	restored.Restore(sess, messages)

	if restored.State() != StateDrive || restored.Plan().Title != eng.Plan().Title {
		t.Errorf("restored state = %s, plan = %+v", restored.State(), restored.Plan())
	}
	writeWorkspaceFile(t, dir, "main.py", "n = int(input())\n")
	if err := restored.Done(context.Background()); err != nil {
		t.Fatalf("Done after restore: %v", err)
	}
	if restored.StepIndex() != 1 {
		t.Errorf("step index = %d", restored.StepIndex())
	}
}

func TestSnapshotsExcludeNinaDir(t *testing.T) {
	eng, dir, _ := startedEngine(t, []llm.Turn{
		{ToolCalls: []llm.ToolCall{
			toolCall(t, llm.ToolSubmitReview, llm.SubmitReviewInput{Verdict: "retry", Feedback: "keep going"}),
		}, StopReason: "tool_use"},
	})
	// Only .nina content changed => diff must be empty, so Done reports
	// "no changes" instead of sending the session state to review.
	if err := eng.Done(context.Background()); err != nil {
		t.Fatal(err)
	}
	if eng.review != nil {
		t.Error(".nina changes leaked into the review diff")
	}
	if _, err := os.Stat(filepath.Join(dir, ".nina", "session.json")); err != nil {
		t.Fatalf("expected session state on disk: %v", err)
	}
}

func confirmingEngine(t *testing.T, answer ConfirmAnswer) (*Engine, string, *[]Event) {
	t.Helper()
	dir := t.TempDir()
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	events := &[]Event{}
	eng := New(&fakeClient{}, ws, dir, profile.Default(), func(ev Event) {
		*events = append(*events, ev)
		if ev.Kind == EventConfirm {
			ev.Confirm.Reply <- answer
		}
	})
	eng.state = StateDrive
	return eng, dir, events
}

func runCall(t *testing.T, command string) llm.ToolCall {
	t.Helper()
	return toolCall(t, llm.ToolRunCommand, llm.RunCommandInput{Command: command, Reason: "verify"})
}

func TestRunCommandApproved(t *testing.T) {
	eng, _, events := confirmingEngine(t, ConfirmAnswer{Approve: true})

	result := eng.execTool(context.Background(), runCall(t, "echo hello"))
	if result.IsError {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Content, "exit code: 0") || !strings.Contains(result.Content, "hello") {
		t.Errorf("content = %q", result.Content)
	}
	kinds := map[EventKind]int{}
	for _, ev := range *events {
		kinds[ev.Kind]++
	}
	if kinds[EventConfirm] != 1 || kinds[EventCommandRun] != 1 {
		t.Errorf("events = %v", kinds)
	}
}

func TestRunCommandDeclined(t *testing.T) {
	eng, dir, events := confirmingEngine(t, ConfirmAnswer{Approve: false})

	result := eng.execTool(context.Background(), runCall(t, "touch declined.txt"))
	if result.IsError {
		t.Fatalf("declined command should not be a tool error: %+v", result)
	}
	if !strings.Contains(result.Content, "declined") {
		t.Errorf("content = %q", result.Content)
	}
	if _, err := os.Stat(filepath.Join(dir, "declined.txt")); !os.IsNotExist(err) {
		t.Error("command ran despite decline")
	}
	for _, ev := range *events {
		if ev.Kind == EventCommandRun {
			t.Error("unexpected command_run event")
		}
	}
}

func TestRunCommandAlwaysSkipsSecondConfirm(t *testing.T) {
	eng, _, events := confirmingEngine(t, ConfirmAnswer{Approve: true, Always: true})

	for range 2 {
		if result := eng.execTool(context.Background(), runCall(t, "true")); result.IsError {
			t.Fatalf("result = %+v", result)
		}
	}
	confirms := 0
	for _, ev := range *events {
		if ev.Kind == EventConfirm {
			confirms++
		}
	}
	if confirms != 1 {
		t.Errorf("confirm events = %d, want 1", confirms)
	}
}

func TestReadFile(t *testing.T) {
	eng, dir, _ := confirmingEngine(t, ConfirmAnswer{})
	writeWorkspaceFile(t, dir, "main.py", "print('hi')\n")

	result := eng.execTool(context.Background(), toolCall(t, llm.ToolReadFile, llm.ReadFileInput{Path: "main.py"}))
	if result.IsError || result.Content != "print('hi')\n" {
		t.Errorf("result = %+v", result)
	}

	result = eng.execTool(context.Background(), toolCall(t, llm.ToolReadFile, llm.ReadFileInput{Path: "../secret"}))
	if !result.IsError {
		t.Error("escaping path accepted")
	}
}

func writeWorkspaceFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lastReview(events *[]Event) string {
	verdict := ""
	for _, ev := range *events {
		if ev.Kind == EventReview {
			verdict = ev.Verdict
		}
	}
	return verdict
}
