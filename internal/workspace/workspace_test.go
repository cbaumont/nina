package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenInitsRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("expected .git directory: %v", err)
	}
}

func TestOpenUsesExistingRepo(t *testing.T) {
	dir := t.TempDir()
	gitOutput(t, dir, "init")
	if _, err := Open(dir); err != nil {
		t.Fatalf("Open: %v", err)
	}
}

func TestSnapshotAndDiff(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "a.txt", "one\n")
	ref1 := SnapshotRef("s1", 0)
	if _, err := w.Snapshot(ref1); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	writeFile(t, dir, "a.txt", "two\n")
	writeFile(t, dir, "b.txt", "new file\n")
	ref2 := SnapshotRef("s1", 1)
	if _, err := w.Snapshot(ref2); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	diff, err := w.Diff(ref1, ref2)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, want := range []string{"-one", "+two", "b.txt", "+new file"} {
		if !strings.Contains(diff, want) {
			t.Errorf("diff missing %q:\n%s", want, diff)
		}
	}
}

func TestSnapshotDoesNotTouchIndexOrHistory(t *testing.T) {
	dir := t.TempDir()
	gitOutput(t, dir, "init")
	gitOutput(t, dir, "-c", "user.name=test", "-c", "user.email=test@test", "commit", "--allow-empty", "-m", "user commit")

	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.txt", "hello\n")
	statusBefore := gitOutput(t, dir, "status", "--porcelain")

	if _, err := w.Snapshot(SnapshotRef("s1", 0)); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if statusAfter := gitOutput(t, dir, "status", "--porcelain"); statusAfter != statusBefore {
		t.Errorf("git status changed:\nbefore: %s\nafter: %s", statusBefore, statusAfter)
	}
	if log := gitOutput(t, dir, "log", "--oneline", "HEAD"); strings.Contains(log, "nina") {
		t.Errorf("snapshot leaked into branch history:\n%s", log)
	}
	if refs := gitOutput(t, dir, "for-each-ref", "refs/nina"); !strings.Contains(refs, "refs/nina/s1/0") {
		t.Errorf("expected snapshot ref, got:\n%s", refs)
	}
}

func TestSnapshotRespectsGitignore(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".gitignore", "ignored.txt\n")
	writeFile(t, dir, "ignored.txt", "secret\n")
	ref1 := SnapshotRef("s1", 0)
	if _, err := w.Snapshot(ref1); err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "ignored.txt", "changed secret\n")
	ref2 := SnapshotRef("s1", 1)
	if _, err := w.Snapshot(ref2); err != nil {
		t.Fatal(err)
	}

	diff, err := w.Diff(ref1, ref2)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Errorf("ignored file leaked into diff:\n%s", diff)
	}
}
