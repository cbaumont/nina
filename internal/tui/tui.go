package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/cbaumont/nina/internal/engine"
	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/workspace"
)

func Run(goal, dir string) error {
	ws, err := workspace.Open(dir)
	if err != nil {
		return err
	}
	client, err := llm.New()
	if err != nil {
		return err
	}
	events := make(chan engine.Event, 64)
	eng := engine.New(client, ws, dir, func(ev engine.Event) {
		events <- ev
	})
	program := tea.NewProgram(newModel(eng, events, goal), tea.WithAltScreen())
	_, err = program.Run()
	return err
}

type engineEventMsg engine.Event

type opDoneMsg struct{ err error }

type model struct {
	eng      *engine.Engine
	events   chan engine.Event
	goal     string
	viewport viewport.Model
	input    textinput.Model
	renderer *glamour.TermRenderer

	history   string
	streaming strings.Builder
	busy      bool
	busyLabel string
	planTitle string
	stepIndex int
	stepCount int
	width     int
	ready     bool
}

func newModel(eng *engine.Engine, events chan engine.Event, goal string) *model {
	input := textinput.New()
	input.Placeholder = "Ask Nina anything, or /done when you finish a step (/quit to exit)"
	input.Focus()
	return &model{
		eng:       eng,
		events:    events,
		goal:      goal,
		input:     input,
		busy:      true,
		busyLabel: "setting up your learning project",
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.waitForEvent(), m.runOp(func() error {
		return m.eng.Start(context.Background(), m.goal)
	}), textinput.Blink)
}

func (m *model) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		return engineEventMsg(<-m.events)
	}
}

func (m *model) runOp(op func() error) tea.Cmd {
	return func() tea.Msg {
		return opDoneMsg{err: op()}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		viewportHeight := msg.Height - 4
		if viewportHeight < 1 {
			viewportHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}
		m.renderer, _ = glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(msg.Width-2))
		m.refreshViewport()
		return m, nil

	case engineEventMsg:
		m.handleEvent(engine.Event(msg))
		m.refreshViewport()
		return m, m.waitForEvent()

	case opDoneMsg:
		m.busy = false
		m.busyLabel = ""
		if msg.err != nil {
			m.flushStreaming()
			m.history += fmt.Sprintf("\n> **Error:** %s\n", msg.err)
			m.refreshViewport()
		}
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
	m.viewport, viewportCmd = m.viewport.Update(msg)
	return m, tea.Batch(inputCmd, viewportCmd)
}

func (m *model) handleInput(text string) (tea.Model, tea.Cmd) {
	if text == "/quit" || text == "/exit" {
		return m, tea.Quit
	}
	if text == "" || m.busy {
		return m, nil
	}
	m.input.Reset()
	switch text {
	case "/done":
		m.busy = true
		m.busyLabel = "reviewing your changes"
		m.history += "\n---\n\n`/done`\n"
		m.refreshViewport()
		return m, m.runOp(func() error { return m.eng.Done(context.Background()) })
	default:
		m.busy = true
		m.busyLabel = "thinking"
		m.history += fmt.Sprintf("\n---\n\n**You:** %s\n", text)
		m.refreshViewport()
		return m, m.runOp(func() error { return m.eng.UserMessage(context.Background(), text) })
	}
}

func (m *model) handleEvent(ev engine.Event) {
	switch ev.Kind {
	case engine.EventTextDelta:
		m.streaming.WriteString(ev.Text)
	case engine.EventInfo:
		m.flushStreaming()
		m.history += fmt.Sprintf("\n> %s\n", ev.Text)
	case engine.EventPlanSet:
		m.flushStreaming()
		m.planTitle = ev.Plan.Title
		m.stepCount = len(ev.Plan.Steps)
		var steps strings.Builder
		for i, step := range ev.Plan.Steps {
			fmt.Fprintf(&steps, "%d. %s\n", i+1, step.Title)
		}
		m.history += fmt.Sprintf("\n## %s\n\n%s", ev.Plan.Title, steps.String())
	case engine.EventStepStarted:
		m.flushStreaming()
		m.stepIndex = ev.Step
	case engine.EventReview:
		m.flushStreaming()
		icon := "✅"
		if ev.Verdict != "pass" {
			icon = "🔄"
		}
		m.history += fmt.Sprintf("\n%s **Review (%s):** %s\n", icon, ev.Verdict, ev.Text)
	case engine.EventSessionDone:
		m.flushStreaming()
		m.history += "\n🎉 **Session complete!** You worked through every step. `/quit` when you're ready.\n"
	}
}

func (m *model) flushStreaming() {
	if m.streaming.Len() == 0 {
		return
	}
	m.history += "\n" + m.streaming.String() + "\n"
	m.streaming.Reset()
}

func (m *model) refreshViewport() {
	if !m.ready {
		return
	}
	content := m.history
	if m.renderer != nil {
		if rendered, err := m.renderer.Render(m.history); err == nil {
			content = rendered
		}
	}
	if m.streaming.Len() > 0 {
		content += "\n" + m.streaming.String()
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

var statusStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("230")).
	Background(lipgloss.Color("62")).
	Padding(0, 1)

func (m *model) statusLine() string {
	title := m.planTitle
	if title == "" {
		title = m.goal
	}
	parts := []string{"nina", title}
	if m.stepCount > 0 {
		step := m.stepIndex + 1
		if step > m.stepCount {
			step = m.stepCount
		}
		parts = append(parts, fmt.Sprintf("step %d/%d", step, m.stepCount))
	}
	parts = append(parts, fmt.Sprintf("dial %d", engine.DialLevel))
	if m.busy {
		parts = append(parts, "⋯ "+m.busyLabel)
	}
	line := statusStyle.Render(strings.Join(parts, "  •  "))
	if m.width > 0 {
		line = lipgloss.PlaceHorizontal(m.width, lipgloss.Left, line, lipgloss.WithWhitespaceBackground(lipgloss.Color("62")))
	}
	return line
}

func (m *model) View() string {
	if !m.ready {
		return "starting nina..."
	}
	return m.statusLine() + "\n" + m.viewport.View() + "\n" + m.input.View()
}
