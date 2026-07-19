package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/profile"
	"github.com/cbaumont/nina/internal/workspace"
)

func TestEndToEndAgainstRealModel(t *testing.T) {
	if os.Getenv("NINA_E2E") == "" {
		t.Skip("set NINA_E2E=1 (and NINA_MODEL, e.g. ollama:gemma4:e2b) to run the live end-to-end test")
	}
	client, err := llm.New()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	var events []Event
	eng := New(client, ws, dir, profile.Default(), func(ev Event) {
		events = append(events, ev)
		if ev.Kind == EventConfirm {
			t.Logf("auto-approving command: %s", ev.Confirm.Command)
			ev.Confirm.Reply <- ConfirmAnswer{Approve: true}
			return
		}
		if ev.Kind != EventTextDelta {
			t.Logf("event %s: step=%d verdict=%s text=%.120s", ev.Kind, ev.Step, ev.Verdict, ev.Text)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := eng.Start(ctx, "learn Python basics with a tiny number guessing game"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if eng.State() != StatePropose {
		t.Fatalf("state after Start = %s, want propose", eng.State())
	}
	if err := eng.UserMessage(ctx, "The first idea sounds good — let's do that one."); err != nil {
		t.Fatalf("choosing a project: %v", err)
	}
	if eng.State() != StateDrive || len(eng.Plan().Steps) == 0 {
		t.Fatalf("state=%s plan=%+v", eng.State(), eng.Plan())
	}
	t.Logf("plan: %s (%d steps)", eng.Plan().Title, len(eng.Plan().Steps))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	scaffolded := false
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != ".git" {
			scaffolded = true
		}
	}
	if !scaffolded {
		t.Log("warning: model scaffolded no files (allowed, but unusual)")
	}

	learnerFile := filepath.Join(dir, "answer.py")
	if err := os.WriteFile(learnerFile, []byte("n = int(input('guess: '))\nprint(n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := eng.Done(ctx); err != nil {
		t.Fatalf("Done: %v", err)
	}

	reviewed := false
	for _, ev := range events {
		if ev.Kind == EventReview || (ev.Kind == EventInfo && strings.Contains(ev.Text, "verdict")) {
			reviewed = true
		}
	}
	if !reviewed {
		t.Error("no review outcome after /done")
	}
}
