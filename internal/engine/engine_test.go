package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cbaumont/nina/internal/llm"
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
	eng := New(&fakeClient{turns: turns}, ws, dir, func(ev Event) {
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

	result := eng.execTool(toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: "solution.py", Content: "answer"}))
	if !result.IsError {
		t.Fatal("write_file allowed outside scaffold state")
	}
	if _, err := os.Stat(filepath.Join(dir, "solution.py")); !os.IsNotExist(err) {
		t.Error("file was created despite dial rejection")
	}
}

func TestWriteFileRejectsEscapingPaths(t *testing.T) {
	eng, _, _ := newTestEngine(t, nil)
	eng.state = StateScaffold
	for _, path := range []string{"../evil.txt", "/etc/passwd", ""} {
		result := eng.execTool(toolCall(t, llm.ToolWriteFile, llm.WriteFileInput{Path: path, Content: "x"}))
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
