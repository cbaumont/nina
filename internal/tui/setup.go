package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cbaumont/nina/internal/profile"
)

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

func (m *model) handleGoal(text string) (tea.Model, tea.Cmd) {
	defer m.updateSuggestions()
	m.input.Reset()
	text = strings.TrimSpace(text)
	if text == "" {
		m.history += "\n> Please tell Nina what you'd like to learn or build.\n"
		m.refreshViewport()
		return m, nil
	}
	m.goal = text
	m.awaitingGoal = false
	if m.setupAfterGoal {
		m.setupAfterGoal = false
		m.setup = &setupFlow{prof: m.eng.Profile()}
		m.history += "\n> A quick minute of setup so Nina can teach at your level — press Enter to keep any default.\n" + setupQuestion(0, m.setup.prof)
		m.refreshViewport()
		return m, nil
	}
	m.busy = true
	m.busyLabel = "brainstorming project ideas"
	m.refreshViewport()
	return m, m.runOp(func() error {
		return m.eng.Start(context.Background(), m.goal)
	})
}

func (m *model) handleSetup(text string) (tea.Model, tea.Cmd) {
	defer m.updateSuggestions()
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
	m.busyLabel = "brainstorming project ideas"
	m.refreshViewport()
	return m, m.runOp(func() error {
		return m.eng.Start(context.Background(), m.goal)
	})
}
