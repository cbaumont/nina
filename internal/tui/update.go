package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/cbaumont/nina/internal/engine"
	"github.com/cbaumont/nina/internal/profile"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-4)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
		}
		m.updateViewportHeight()
		m.renderer, _ = glamour.NewTermRenderer(glamour.WithStandardStyle(m.style), glamour.WithWordWrap(msg.Width-2))
		m.renderedHistorySrc = ""
		m.refreshViewport()
		return m, nil

	case engineEventMsg:
		m.handleEvent(engine.Event(msg))
		m.refreshViewport()
		if engine.Event(msg).Kind == engine.EventSessionDone {
			m.busy = true
			m.busyLabel = "writing your session summary"
			return m, tea.Batch(m.waitForEvent(), m.runOp(func() error {
				return m.eng.Summarize(context.Background())
			}))
		}
		return m, m.waitForEvent()

	case opDoneMsg:
		m.busy = false
		m.busyLabel = ""
		m.flushStreaming()
		if msg.err != nil {
			m.history += fmt.Sprintf("\n> **Error:** %s\n", msg.err)
		}
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleInput(strings.TrimSpace(m.input.Value()))
		}
	}

	var inputCmd, viewportCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	m.updateSuggestions()
	if keyMsg, ok := msg.(tea.KeyMsg); !ok || keyMsg.Type != tea.KeyRunes {
		m.viewport, viewportCmd = m.viewport.Update(msg)
	}
	return m, tea.Batch(inputCmd, viewportCmd)
}

func (m *model) updateSuggestions() {
	text := m.input.Value()
	if m.setup != nil || m.pendingConfirm != nil ||
		!strings.HasPrefix(text, "/") || strings.ContainsAny(text, " \t") {
		m.suggestions = nil
	} else {
		m.suggestions = matchCommands(text)
	}
	m.updateViewportHeight()
}

func (m *model) updateViewportHeight() {
	if !m.ready {
		return
	}
	h := m.height - 4
	if len(m.suggestions) > 0 {
		h -= len(m.suggestions) + 1
	}
	if h < 1 {
		h = 1
	}
	m.viewport.Height = h
}

func (m *model) handleInput(text string) (tea.Model, tea.Cmd) {
	defer m.updateSuggestions()
	if m.pendingConfirm != nil {
		return m.handleConfirm(strings.ToLower(text))
	}
	if m.setup != nil {
		return m.handleSetup(text)
	}
	if text == "/quit" || text == "/exit" {
		return m, tea.Quit
	}
	if text == "" || m.busy {
		return m, nil
	}
	m.input.Reset()
	if command, ok := strings.CutPrefix(text, "/run"); ok && (command == "" || command[0] == ' ') {
		command = strings.TrimSpace(command)
		message := "Please run the project (or its tests) now and walk me through the output."
		if command != "" {
			message = fmt.Sprintf("Please run `%s` now and walk me through the output.", command)
		}
		return m.sendToNina("`"+text+"`", "running", message)
	}
	if value, ok := strings.CutPrefix(text, "/dial "); ok {
		dial, err := profile.ParseDial(strings.TrimSpace(value))
		if err != nil {
			m.history += "\n> " + err.Error() + "\n"
			m.refreshViewport()
			return m, nil
		}
		prof := m.eng.Profile()
		prof.Dial = dial
		if err := m.eng.UpdateProfile(prof); err != nil {
			m.history += "\n> **Error:** " + err.Error() + "\n"
		} else {
			m.history += fmt.Sprintf("\n> 🎚️ Typing dial set to %d.\n", dial)
		}
		m.refreshViewport()
		return m, nil
	}
	switch text {
	case "/done":
		m.busy = true
		m.busyLabel = "reviewing your changes"
		m.history += "\n---\n\n`/done`\n"
		m.refreshViewport()
		return m, m.runOp(func() error { return m.eng.Done(context.Background()) })
	case "/skip":
		m.busy = true
		m.busyLabel = "skipping to the next step"
		m.history += "\n---\n\n`/skip`\n"
		m.refreshViewport()
		return m, m.runOp(func() error { return m.eng.Skip(context.Background()) })
	case "/why":
		return m.sendToNina("`"+text+"`", "thinking",
			"Why this step? Zoom out: explain how it fits into the bigger picture of what we're building and why it comes now.")
	case "/stuck":
		return m.sendToNina("`"+text+"`", "thinking",
			"I'm stuck on the current step. Help me get moving again, escalating per my hint settings — start with your next-strongest hint, not the full solution.")
	case "/recap":
		return m.sendToNina("`"+text+"`", "recapping",
			"Recap the session so far: what we've built, the concepts covered, and how the pieces fit together.")
	case "/summary":
		m.busy = true
		m.busyLabel = "writing your session summary"
		m.history += "\n---\n\n`/summary`\n"
		m.refreshViewport()
		return m, m.runOp(func() error { return m.eng.Summarize(context.Background()) })
	case "/copy":
		if err := clipboard.WriteAll(m.history); err != nil {
			m.history += fmt.Sprintf("\n> **Error:** could not copy: %s\n", err)
		} else {
			m.history += "\n> 📋 Session copied to clipboard.\n"
		}
		m.refreshViewport()
		return m, nil
	case "/profile":
		m.setup = &setupFlow{prof: m.eng.Profile(), editing: true}
		m.history += "\n> Adjust your profile — press Enter to keep any current value.\n" + setupQuestion(0, m.setup.prof)
		m.refreshViewport()
		return m, nil
	case "/dial":
		m.history += fmt.Sprintf("\n> The typing dial is at %d. Change it with `/dial <0-3>`.\n", m.eng.Profile().Dial)
		m.refreshViewport()
		return m, nil
	case "/help":
		var b strings.Builder
		b.WriteString("\n> **Commands:**\n")
		for _, c := range commands {
			fmt.Fprintf(&b, "> - `%s` — %s\n", c.display(), c.Desc)
		}
		m.history += b.String()
		m.refreshViewport()
		return m, nil
	default:
		if strings.HasPrefix(text, "/") {
			m.history += fmt.Sprintf("\n> Unknown command `%s` — `/help` lists the commands.\n", text)
			m.refreshViewport()
			return m, nil
		}
		return m.sendToNina("**You:** "+text, "thinking", text)
	}
}

func (m *model) sendToNina(display, label, message string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = label
	m.history += fmt.Sprintf("\n---\n\n%s\n", display)
	m.refreshViewport()
	return m, m.runOp(func() error { return m.eng.UserMessage(context.Background(), message) })
}

func (m *model) handleConfirm(answer string) (tea.Model, tea.Cmd) {
	defer m.updateSuggestions()
	req := m.pendingConfirm
	var reply engine.ConfirmAnswer
	switch answer {
	case "y", "yes":
		reply = engine.ConfirmAnswer{Approve: true}
	case "a", "always":
		reply = engine.ConfirmAnswer{Approve: true, Always: true}
	case "n", "no":
		reply = engine.ConfirmAnswer{}
	default:
		m.history += "\n> Please answer **y** (run once), **a** (always this session), or **n** (skip).\n"
		m.refreshViewport()
		m.input.Reset()
		return m, nil
	}
	m.pendingConfirm = nil
	m.input.Reset()
	label := "skipped"
	if reply.Approve {
		label = "approved"
		if reply.Always {
			label = "approved for this session"
		}
	}
	m.history += fmt.Sprintf("\n> `%s` %s\n", req.Command, label)
	m.refreshViewport()
	req.Reply <- reply
	return m, nil
}
