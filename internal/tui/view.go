package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var statusStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("230")).
	Background(lipgloss.Color("62")).
	Padding(0, 1)

var (
	suggestionCmdStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	suggestionDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (m *model) refreshViewport() {
	if !m.ready {
		return
	}
	if m.history != m.renderedHistorySrc {
		content := m.history
		if m.renderer != nil {
			if rendered, err := m.renderer.Render(m.history); err == nil {
				content = rendered
			}
		}
		m.renderedHistory = content
		m.renderedHistorySrc = m.history
		m.historyRenderCount++
	}
	content := m.renderedHistory
	if m.streaming.Len() > 0 {
		streamed := m.streaming.String()
		if m.renderer != nil {
			if rendered, err := m.renderer.Render(streamed); err == nil {
				streamed = rendered
			}
		}
		content += "\n" + streamed
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m *model) suggestionsView() string {
	lines := make([]string, len(m.suggestions))
	for i, c := range m.suggestions {
		lines[i] = "  " + suggestionCmdStyle.Render(c.display()) + "  " + suggestionDescStyle.Render(c.Desc)
	}
	return strings.Join(lines, "\n")
}

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
	view := m.statusLine() + "\n" + m.viewport.View() + "\n"
	if len(m.suggestions) > 0 {
		view += m.suggestionsView() + "\n"
	}
	return view + m.input.View()
}
