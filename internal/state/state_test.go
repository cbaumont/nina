package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cbaumont/nina/internal/llm"
)

func TestLoadWithoutSession(t *testing.T) {
	sess, messages, err := Load(t.TempDir())
	if sess != nil || messages != nil || err != nil {
		t.Errorf("got %+v, %+v, %v", sess, messages, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sess := &Session{
		SessionID: "20260719-120000",
		Goal:      "learn python",
		State:     "drive",
		PlanTitle: "Guessing Game",
		Steps: []llm.PlanStep{
			{Title: "Read input", Goal: "reads a number"},
			{Title: "Compare", Goal: "says higher or lower"},
		},
		StepIndex: 1,
		Snapshots: 2,
		LastRef:   "refs/nina/20260719-120000/1",
	}
	messages := []llm.Message{
		{Role: "user", Text: "start"},
		{Role: "assistant", Text: "plan", ToolCalls: []llm.ToolCall{
			{ID: "toolu_1", Name: "set_plan", Input: json.RawMessage(`{"title":"Guessing Game"}`)},
		}},
		{Role: "user", ToolResults: []llm.ToolResult{
			{ToolCallID: "toolu_1", Content: "plan set with 2 steps"},
		}},
	}

	if err := Save(dir, sess, messages); err != nil {
		t.Fatal(err)
	}
	gotSess, gotMessages, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSess, sess) {
		t.Errorf("session = %+v, want %+v", gotSess, sess)
	}
	if !reflect.DeepEqual(gotMessages, messages) {
		t.Errorf("messages = %+v, want %+v", gotMessages, messages)
	}
}

func TestLoadHistoryWithoutFile(t *testing.T) {
	history, err := LoadHistory(t.TempDir())
	if history != "" || err != nil {
		t.Errorf("got %q, %v", history, err)
	}
}

func TestSaveLoadHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	text := "## Guessing Game\n\nStep 1: read input\n"
	if err := SaveHistory(dir, text); err != nil {
		t.Fatal(err)
	}
	got, err := LoadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != text {
		t.Errorf("history = %q, want %q", got, text)
	}

	if err := SaveHistory(dir, "replaced"); err != nil {
		t.Fatal(err)
	}
	got, err = LoadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "replaced" {
		t.Errorf("history = %q, want %q", got, "replaced")
	}
}

func TestSaveOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, &Session{StepIndex: 0}, []llm.Message{{Role: "user", Text: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, &Session{StepIndex: 1}, []llm.Message{{Role: "user", Text: "a"}, {Role: "assistant", Text: "b"}}); err != nil {
		t.Fatal(err)
	}
	sess, messages, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sess.StepIndex != 1 || len(messages) != 2 {
		t.Errorf("session = %+v, messages = %d", sess, len(messages))
	}
	if _, err := os.Stat(filepath.Join(dir, ".nina", "session.json.tmp")); !os.IsNotExist(err) {
		t.Error("tmp file left behind")
	}
}
