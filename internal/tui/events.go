package tui

import (
	"fmt"
	"strings"

	"github.com/cbaumont/nina/internal/engine"
)

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
