package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace struct {
	dir string
}

func Open(dir string) (*Workspace, error) {
	w := &Workspace{dir: dir}
	if _, err := w.git(nil, "rev-parse", "--git-dir"); err != nil {
		if _, err := w.git(nil, "init"); err != nil {
			return nil, fmt.Errorf("initializing git repository: %w", err)
		}
	}
	if err := w.excludeFromSnapshots("/.nina/"); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Workspace) excludeFromSnapshots(pattern string) error {
	gitDir, err := w.git(nil, "rev-parse", "--git-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(w.dir, gitDir)
	}
	path := filepath.Join(gitDir, "info", "exclude")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for line := range strings.SplitSeq(string(existing), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content+pattern+"\n"), 0o644)
}

func SnapshotRef(session string, step int) string {
	return fmt.Sprintf("refs/nina/%s/%d", session, step)
}

func (w *Workspace) Snapshot(ref string) (string, error) {
	indexDir, err := os.MkdirTemp("", "nina-index-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(indexDir)
	indexEnv := []string{"GIT_INDEX_FILE=" + filepath.Join(indexDir, "index")}

	if _, err := w.git(indexEnv, "add", "-A", "."); err != nil {
		return "", fmt.Errorf("staging snapshot: %w", err)
	}
	tree, err := w.git(indexEnv, "write-tree")
	if err != nil {
		return "", fmt.Errorf("writing snapshot tree: %w", err)
	}
	commit, err := w.git(nil, "commit-tree", tree, "-m", "nina snapshot")
	if err != nil {
		return "", fmt.Errorf("writing snapshot commit: %w", err)
	}
	if _, err := w.git(nil, "update-ref", ref, commit); err != nil {
		return "", fmt.Errorf("updating snapshot ref: %w", err)
	}
	return commit, nil
}

func (w *Workspace) Diff(refA, refB string) (string, error) {
	return w.git(nil, "diff", refA, refB)
}

func (w *Workspace) git(extraEnv []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=nina",
		"GIT_AUTHOR_EMAIL=nina@localhost",
		"GIT_COMMITTER_NAME=nina",
		"GIT_COMMITTER_EMAIL=nina@localhost",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
