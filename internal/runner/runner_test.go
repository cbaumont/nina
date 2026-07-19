package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCapturesOutputAndExitCode(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), "echo out; echo err >&2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.TimedOut {
		t.Errorf("result = %+v", res)
	}
	if !strings.Contains(res.Output, "out") || !strings.Contains(res.Output, "err") {
		t.Errorf("output = %q", res.Output)
	}
}

func TestRunNonZeroExit(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), "exit 3", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d", res.ExitCode)
	}
}

func TestRunTimeoutKillsCommand(t *testing.T) {
	start := time.Now()
	res, err := Run(context.Background(), t.TempDir(), "sleep 10", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut {
		t.Error("expected TimedOut")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("kill took too long: %s", elapsed)
	}
}

func TestRunRunsInDir(t *testing.T) {
	dir := t.TempDir()
	res, err := Run(context.Background(), dir, "pwd", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, dir) {
		t.Errorf("pwd = %q, want dir %q", res.Output, dir)
	}
}

func TestRunCapsOutputToTail(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), "seq 1 100000", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Output) > maxOutput+64 {
		t.Errorf("output length = %d", len(res.Output))
	}
	if !strings.HasPrefix(res.Output, "[output truncated") {
		t.Error("missing truncation marker")
	}
	if !strings.Contains(res.Output, "100000") {
		t.Error("tail of output missing")
	}
}
