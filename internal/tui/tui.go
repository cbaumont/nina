package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/cbaumont/nina/internal/engine"
	"github.com/cbaumont/nina/internal/llm"
	"github.com/cbaumont/nina/internal/profile"
	"github.com/cbaumont/nina/internal/state"
	"github.com/cbaumont/nina/internal/watcher"
	"github.com/cbaumont/nina/internal/workspace"
)

const watcherIdle = 25 * time.Second

// Run starts a session. With an empty goal it resumes the session saved
// in dir; with a goal it starts fresh, refusing if a live session exists.
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
	needSetup := !profileFound && sess == nil
	// Watch failure is non-fatal: worst case Nina behaves as if watching
	// were off and the learner drives reviews with /done alone.
	if w, err := watcher.Start(dir, watcherIdle, func() {
		select {
		case events <- engine.Event{Kind: engine.EventNudge}:
		default:
		}
	}); err == nil {
		defer w.Close()
	}
	program := tea.NewProgram(newModel(eng, events, goal, needSetup), tea.WithAltScreen())
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

	history        string
	streaming      strings.Builder
	busy           bool
	busyLabel      string
	pendingConfirm *engine.ConfirmRequest
	nudgedStep     int
	setup          *setupFlow
	planTitle string
	stepIndex int
	stepCount int
	width     int
	ready     bool
}

func newModel(eng *engine.Engine, events chan engine.Event, goal string, needSetup bool) *model {
	input := textinput.New()
	input.Placeholder = "Ask Nina anything · /done when you finish a step · /help for commands (/quit to exit)"
	input.Focus()
	m := &model{
		eng:        eng,
		events:     events,
		goal:       goal,
		input:      input,
		busy:       true,
		busyLabel:  "setting up your learning project",
		nudgedStep: -1,
	}
	if needSetup {
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
		step := plan.Steps[m.stepIndex]
		m.history = fmt.Sprintf("## %s\n\n▶️ **Session resumed** at step %d/%d: %s\n\nStep goal: %s\n\nKeep working in your editor and `/done` when ready — or ask Nina to remind you where you left off.\n",
			plan.Title, m.stepIndex+1, m.stepCount, step.Title, step.Goal)
	}
	return m
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.waitForEvent(), textinput.Blink}
	if m.eng.State() == engine.StateIdle && m.setup == nil {
		cmds = append(cmds, m.runOp(func() error {
			return m.eng.Start(context.Background(), m.goal)
		}))
	}
	return tea.Batch(cmds...)
}

// setupFlow walks the profile questions, either at first run (then the
// session starts once done) or when re-opened with /profile mid-session.
type setupFlow struct {
	index   int
	prof    profile.Profile
	editing bool
}

const setupQuestions = 5

func setupQuestion(index int, prof profile.Profile) string {
	switch index {
	case 0:
		return fmt.Sprintf("\n**1/5 — Your general programming experience?** `none` · `beginner` · `intermediate` · `professional` *(default: %s)*\n", prof.Experience)
	case 1:
		return fmt.Sprintf("\n**2/5 — How familiar are you with the stack you're learning?** `none` · `beginner` · `intermediate` · `professional` *(default: %s)*\n", prof.StackFamiliar)
	case 2:
		known := strings.Join(prof.KnownStacks, ", ")
		if known == "" {
			known = "none"
		}
		return fmt.Sprintf("\n**3/5 — Languages or stacks you already know?** comma-separated, so Nina can use analogies *(default: %s)*\n", known)
	case 3:
		return fmt.Sprintf("\n**4/5 — How much may Nina type?** `0` full manual · `1` scaffold only · `2` + boilerplate · `3` collaborative *(default: %d)*\n", prof.Dial)
	default:
		return fmt.Sprintf("\n**5/5 — How fast should hints escalate when you're stuck?** `slow` · `medium` · `fast` *(default: %s)*\n", prof.HintEscalation)
	}
}

func (m *model) handleSetup(text string) (tea.Model, tea.Cmd) {
	m.input.Reset()
	s := m.setup
	if answer := strings.ToLower(strings.TrimSpace(text)); answer != "" {
		var err error
		switch s.index {
		case 0:
			s.prof.Experience, err = profile.ParseLevel(answer)
		case 1:
			s.prof.StackFamiliar, err = profile.ParseLevel(answer)
		case 2:
			s.prof.KnownStacks = nil
			if answer != "none" {
				for stack := range strings.SplitSeq(answer, ",") {
					if stack = strings.TrimSpace(stack); stack != "" {
						s.prof.KnownStacks = append(s.prof.KnownStacks, stack)
					}
				}
			}
		case 3:
			s.prof.Dial, err = profile.ParseDial(answer)
		case 4:
			s.prof.HintEscalation, err = profile.ParseHintSpeed(answer)
		}
		if err != nil {
			m.history += "\n> " + err.Error() + "\n" + setupQuestion(s.index, s.prof)
			m.refreshViewport()
			return m, nil
		}
	}
	s.index++
	if s.index < setupQuestions {
		m.history += setupQuestion(s.index, s.prof)
		m.refreshViewport()
		return m, nil
	}

	m.setup = nil
	if err := m.eng.UpdateProfile(s.prof); err != nil {
		m.history += "\n> **Error:** could not save your profile: " + err.Error() + "\n"
	}
	if s.editing {
		m.history += "\n> ✅ Profile updated — Nina adapts from the next message.\n"
		m.refreshViewport()
		return m, nil
	}
	m.busy = true
	m.busyLabel = "setting up your learning project"
	m.refreshViewport()
	return m, m.runOp(func() error {
		return m.eng.Start(context.Background(), m.goal)
	})
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
		m.history += "\n> **Commands:** `/done` review the step · `/why` zoom out · `/stuck` get help · `/skip` next step · `/recap` session recap · `/run [cmd]` run code · `/dial <0-3>` typing dial · `/profile` adjust profile · `/quit`\n"
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

func (m *model) handleEvent(ev engine.Event) {
	switch ev.Kind {
	case engine.EventTextDelta:
		m.streaming.WriteString(ev.Text)
	case engine.EventInfo:
		m.flushStreaming()
		m.history += fmt.Sprintf("\n> %s\n", ev.Text)
	case engine.EventCommandRun:
		m.flushStreaming()
		m.history += "\n" + ev.Text + "\n"
	case engine.EventConfirm:
		m.flushStreaming()
		m.pendingConfirm = ev.Confirm
		reason := ""
		if ev.Confirm.Reason != "" {
			reason = " — " + ev.Confirm.Reason
		}
		m.history += fmt.Sprintf("\n> ⚡ Nina wants to run `%s`%s\n>\n> **y** run once · **a** always this session · **n** skip\n", ev.Confirm.Command, reason)
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
	case engine.EventNudge:
		// Hint only, and at most once per step: /done stays authoritative.
		if m.busy || m.eng.State() != engine.StateDrive || m.nudgedStep == m.stepIndex {
			return
		}
		m.nudgedStep = m.stepIndex
		m.history += "\n> 👀 Looks like you've made changes and paused — `/done` whenever you want a review.\n"
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
	parts = append(parts, fmt.Sprintf("dial %d", m.eng.Profile().Dial))
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
