package tui

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/cbaumont/nina/internal/engine"
	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/profile"
	"github.com/cbaumont/nina/internal/workspace"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

type fakeClient struct{}

func (fakeClient) Converse(_ context.Context, _ *llm.Conversation, _ func(string)) (llm.Turn, error) {
	return llm.Turn{Text: "ok", StopReason: "end_turn"}, nil
}

func newTestModel(t *testing.T) *model {
	t.Helper()
	dir := t.TempDir()
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(fakeClient{}, ws, dir, profile.Default(), func(engine.Event) {})
	return newModel(eng, make(chan engine.Event), "learn go", false, false, "dark")
}

func TestNewModelRendersBeforeWindowSizeMsg(t *testing.T) {
	m := newTestModel(t)
	if m.renderer == nil {
		t.Fatal("renderer must be set before any WindowSizeMsg arrives")
	}
	m.history = "**Idea 1:** a guessing game\n"
	m.refreshViewport()
	raw := m.viewport.View()
	if !ansiEscape.MatchString(raw) {
		t.Fatalf("markdown was not rendered before WindowSizeMsg (no ANSI styling found): %q", raw)
	}
	if strings.Contains(stripANSI(raw), "**Idea 1:**") {
		t.Fatalf("bold markers should be styled away, not shown literally: %q", raw)
	}
}

func TestRefreshViewportCachesUnchangedHistory(t *testing.T) {
	m := newTestModel(t)
	m.history = "# Step 1\n\nDo the thing.\n"
	m.refreshViewport()
	if m.historyRenderCount != 1 {
		t.Fatalf("first refresh should render once, got %d", m.historyRenderCount)
	}

	m.streaming.WriteString("still working")
	m.refreshViewport()
	if m.historyRenderCount != 1 {
		t.Fatalf("refresh with unchanged history should reuse cache, got %d renders", m.historyRenderCount)
	}
	if !strings.Contains(stripANSI(m.viewport.View()), "still working") {
		t.Fatal("streaming text should still appear in the viewport")
	}

	m.history += "\nMore.\n"
	m.refreshViewport()
	if m.historyRenderCount != 2 {
		t.Fatalf("refresh with changed history should render again, got %d", m.historyRenderCount)
	}
}

func TestRefreshViewportRendersStreamingMarkdown(t *testing.T) {
	m := newTestModel(t)
	m.streaming.WriteString("**bold**")
	m.refreshViewport()
	if strings.Contains(stripANSI(m.viewport.View()), "**bold**") {
		t.Fatalf("streaming text should be markdown-rendered while typing: %q", m.viewport.View())
	}
}

func TestNewModelWithoutGoalAwaitsGoalFirst(t *testing.T) {
	dir := t.TempDir()
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(fakeClient{}, ws, dir, profile.Default(), func(engine.Event) {})
	m := newModel(eng, make(chan engine.Event), "", false, true, "dark")
	if !m.awaitingGoal {
		t.Fatal("model should await a goal when none was provided")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should still return commands (event wait, blink)")
	}
	if m.eng.State() != engine.StateIdle {
		t.Fatal("engine should not start until a goal is supplied")
	}

	m2, _ := m.handleGoal("learn Go generics")
	got := m2.(*model)
	if got.awaitingGoal {
		t.Fatal("awaitingGoal should clear once a goal is entered")
	}
	if got.goal != "learn Go generics" {
		t.Fatalf("goal = %q, want %q", got.goal, "learn Go generics")
	}
}

func TestOpDoneFlushesStreamingOnSuccess(t *testing.T) {
	m := newTestModel(t)
	m.busy = true
	m.streaming.WriteString("Idea 1: a guessing game.")
	m.Update(opDoneMsg{})
	if m.streaming.Len() != 0 {
		t.Fatal("streaming buffer should be flushed once the op completes")
	}
	if !strings.Contains(m.history, "Idea 1: a guessing game.") {
		t.Fatalf("flushed streaming text should land in history, got %q", m.history)
	}
	if !strings.Contains(stripANSI(m.viewport.View()), "Idea") {
		t.Fatal("flushed content should be visible in the viewport without further input")
	}
}
