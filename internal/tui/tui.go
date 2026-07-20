package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cbaumont/nina/internal/engine"
	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/profile"
	"github.com/cbaumont/nina/internal/state"
	"github.com/cbaumont/nina/internal/watcher"
	"github.com/cbaumont/nina/internal/workspace"
)

const watcherIdle = 25 * time.Second

func Run(goal, dir string) error {
	sess, messages, err := state.Load(dir)
	if err != nil {
		return err
	}
	if goal == "" {
		if sess == nil {
			return fmt.Errorf("no session to resume here; start one with nina start \"<learning goal>\"")
		}
		if sess.State == string(engine.StateDone) {
			return fmt.Errorf("the last session is complete; start a new one with nina start \"<learning goal>\"")
		}
	} else if sess != nil && sess.State != string(engine.StateDone) {
		return fmt.Errorf("a session is already in progress here (%s); continue it with nina resume, or delete .nina/ to start over", sess.PlanTitle)
	}

	ws, err := workspace.Open(dir)
	if err != nil {
		return err
	}
	client, err := llm.New()
	if err != nil {
		return err
	}
	prof, profileFound, err := profile.Load(dir)
	if err != nil {
		return err
	}
	events := make(chan engine.Event, 64)
	eng := engine.New(client, ws, dir, prof, func(ev engine.Event) {
		events <- ev
	})
	if goal == "" {
		eng.Restore(sess, messages)
		goal = sess.Goal
	}
	if screener, err := llm.NewScreener(); err == nil {
		eng.SetScreener(screener)
	}
	needSetup := !profileFound && sess == nil
	if w, err := watcher.Start(dir, watcherIdle, func() {
		select {
		case events <- engine.Event{Kind: engine.EventNudge}:
		default:
		}
	}); err == nil {
		defer w.Close()
	}
	style := "dark"
	if !lipgloss.HasDarkBackground() {
		style = "light"
	}
	program := tea.NewProgram(newModel(eng, events, goal, needSetup, style), tea.WithAltScreen())
	_, err = program.Run()
	return err
}
