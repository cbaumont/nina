package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher observes a workspace and calls nudge once the user has edited
// files and then gone idle. It is a hint source only: file events never
// trigger reviews directly, /done stays authoritative, and any watch
// failure degrades to silence.
type Watcher struct {
	fs    *fsnotify.Watcher
	root  string
	idle  time.Duration
	nudge func()
}

var ignoredDirs = map[string]bool{
	".git":         true,
	".nina":        true,
	"node_modules": true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".idea":        true,
	".vscode":      true,
}

func Start(dir string, idle time.Duration, nudge func()) (*Watcher, error) {
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{fs: fs, root: dir, idle: idle, nudge: nudge}
	if err := w.addRecursive(dir); err != nil {
		fs.Close()
		return nil, err
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Close() error {
	return w.fs.Close()
}

func (w *Watcher) loop() {
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case event, ok := <-w.fs.Events:
			if !ok {
				return
			}
			if w.ignored(event.Name) {
				continue
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.addRecursive(event.Name)
					continue
				}
			}
			if timer == nil {
				timer = time.NewTimer(w.idle)
				timerC = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timerC:
					default:
					}
				}
				timer.Reset(w.idle)
			}
		case <-timerC:
			timer = nil
			timerC = nil
			w.nudge()
		case _, ok := <-w.fs.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) addRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if path != w.root && ignoredDirs[entry.Name()] {
			return filepath.SkipDir
		}
		return w.fs.Add(path)
	})
}

func (w *Watcher) ignored(path string) bool {
	rel, err := filepath.Rel(w.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return true
	}
	for part := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		if ignoredDirs[part] {
			return true
		}
	}
	base := filepath.Base(path)
	// Editor noise: vim/emacs swap, temp, and lock files.
	return strings.HasSuffix(base, "~") ||
		strings.HasSuffix(base, ".swp") ||
		strings.HasSuffix(base, ".swo") ||
		strings.HasSuffix(base, ".tmp") ||
		strings.HasPrefix(base, ".#") ||
		strings.HasPrefix(base, "#")
}
