package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/cbaumont/nina/internal/engine"
)

type engineEventMsg engine.Event

type opDoneMsg struct{ err error }

type model struct {
	eng      *engine.Engine
	events   chan engine.Event
	goal     string
	viewport viewport.Model
	input    textinput.Model
	renderer *glamour.TermRenderer
	style    string

	history        string
	streaming      strings.Builder
	busy           bool
	busyLabel      string
	pendingConfirm *engine.ConfirmRequest
	nudgedStep     int
	setup          *setupFlow
	awaitingGoal   bool
	setupAfterGoal bool
	planTitle      string
	stepIndex      int
	stepCount      int
	width          int
	height         int
	ready          bool
	suggestions    []commandInfo

	renderedHistory    string
	renderedHistorySrc string
	historyRenderCount int
}

func newModel(eng *engine.Engine, events chan engine.Event, goal string, needSetup, needGoal bool, style string) *model {
	input := textinput.New()
	input.Placeholder = "Ask Nina anything · /done when you finish a step · /help for commands (/quit to exit)"
	input.Focus()
	input.Cursor.SetMode(cursor.CursorStatic)
	m := &model{
		eng:        eng,
		events:     events,
		goal:       goal,
		input:      input,
		busy:       true,
		busyLabel:  "brainstorming project ideas",
		nudgedStep: -1,
		style:      style,
		viewport:   viewport.New(80, 20),
		ready:      true,
	}
	m.renderer, _ = glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(78))
	if needGoal {
		m.busy = false
		m.busyLabel = ""
		m.awaitingGoal = true
		m.setupAfterGoal = needSetup
		m.history = "## Welcome to Nina 👋\n\nWhat would you like to learn or build? Tell Nina your goal — you can always refine it later.\n"
	} else if needSetup {
		m.busy = false
		m.busyLabel = ""
		m.setup = &setupFlow{prof: eng.Profile()}
		m.history = "## Welcome to Nina 👋\n\nA quick minute of setup so Nina can teach at your level — press Enter to keep any default.\n" + setupQuestion(0, m.setup.prof)
	}
	if eng.State() != engine.StateIdle {
		plan := eng.Plan()
		m.planTitle = plan.Title
		m.stepCount = len(plan.Steps)
		m.stepIndex = eng.StepIndex()
		m.busy = false
		m.busyLabel = ""
		if m.stepIndex < len(plan.Steps) {
			step := plan.Steps[m.stepIndex]
			m.history = fmt.Sprintf("## %s\n\n▶️ **Session resumed** at step %d/%d: %s\n\nStep goal: %s\n\nKeep working in your editor and `/done` when ready — or ask Nina to remind you where you left off.\n",
				plan.Title, m.stepIndex+1, m.stepCount, step.Title, step.Goal)
		} else {
			m.history = "▶️ **Session resumed** — you were still choosing a project. Ask Nina to repeat the ideas, or tell it what you'd like to build.\n"
		}
	}
	return m
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.waitForEvent(), textinput.Blink}
	if m.eng.State() == engine.StateIdle && m.setup == nil && !m.awaitingGoal {
		cmds = append(cmds, m.runOp(func() error {
			return m.eng.Start(context.Background(), m.goal)
		}))
	}
	return tea.Batch(cmds...)
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
