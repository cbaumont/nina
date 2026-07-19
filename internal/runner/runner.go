package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultTimeout = 2 * time.Minute
	maxOutput      = 64 * 1024
)

type Result struct {
	ExitCode int
	Output   string
	TimedOut bool
}

// Run executes command through `sh -c` in dir, capturing combined
// stdout/stderr (tail-capped at maxOutput). The whole process group is
// killed when timeout elapses.
func Run(ctx context.Context, dir, command string, timeout time.Duration) (Result, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	out := &tailBuffer{limit: maxOutput}
	cmd.Stdout = out
	cmd.Stderr = out

	err := cmd.Run()
	result := Result{Output: out.String(), TimedOut: ctx.Err() == context.DeadlineExceeded}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		if result.TimedOut {
			result.ExitCode = -1
			return result, nil
		}
		return result, fmt.Errorf("running %q: %w", command, err)
	}
	return result, nil
}

// tailBuffer keeps only the last `limit` bytes written to it.
type tailBuffer struct {
	limit     int
	buf       []byte
	truncated bool
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.limit {
		b.buf = b.buf[len(b.buf)-b.limit:]
		b.truncated = true
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	s := strings.TrimSpace(string(b.buf))
	if b.truncated {
		return "[output truncated, showing the tail]\n" + s
	}
	return s
}
