package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startWatcher(t *testing.T, dir string) chan struct{} {
	t.Helper()
	nudges := make(chan struct{}, 8)
	w, err := Start(dir, 100*time.Millisecond, func() {
		nudges <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return nudges
}

func expectNudge(t *testing.T, nudges chan struct{}) {
	t.Helper()
	select {
	case <-nudges:
	case <-time.After(3 * time.Second):
		t.Fatal("no nudge after relevant file activity")
	}
}

func expectQuiet(t *testing.T, nudges chan struct{}) {
	t.Helper()
	select {
	case <-nudges:
		t.Fatal("nudge fired for ignored activity")
	case <-time.After(400 * time.Millisecond):
	}
}

func TestNudgeAfterEditThenIdle(t *testing.T) {
	dir := t.TempDir()
	nudges := startWatcher(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectNudge(t, nudges)
}

func TestIgnoredPathsStayQuiet(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{".git", ".nina"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	nudges := startWatcher(t, dir)

	os.WriteFile(filepath.Join(dir, ".git", "index"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, ".nina", "session.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.py.swp"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "backup~"), []byte("x"), 0o644)
	expectQuiet(t, nudges)
}

func TestWatchesNewSubdirectories(t *testing.T) {
	dir := t.TempDir()
	nudges := startWatcher(t, dir)

	sub := filepath.Join(dir, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(sub, "app.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectNudge(t, nudges)
}
