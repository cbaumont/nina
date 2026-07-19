package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/cbaumont/nina/internal/llm"
)

// Message screening is the engineered control against over-helping in
// chat: at dial levels 0-1 a fast-tier model classifies each outgoing
// navigator message during the drive/review loop, and flagged messages
// are regenerated once. Screened messages are not streamed; they arrive
// as a single block after passing.

// SetScreener installs the fast-tier client. Without one (or at dial
// levels 2-3) messages stream through unscreened.
func (e *Engine) SetScreener(client llm.Client) { e.screener = client }

func (e *Engine) screeningActive() bool {
	return e.screener != nil && e.profile.Dial <= 1 && e.state == StateDrive
}

const screenSystemPrompt = `You are a strict classifier inside a pair-programming teaching tool. The learner must type the step's code themselves; the navigator may guide but not hand over the solution. Decide whether the navigator message contains complete code that solves the learner's current step — code they could paste to finish the step without writing it themselves. Guidance, construct names, and small fragments (an import, a signature, one short illustrative line) are fine. Reply with exactly one word: LEAK if the message hands over the step's solution, otherwise OK.`

func (e *Engine) leaks(ctx context.Context, text string) bool {
	goal := ""
	if e.stepIndex < len(e.plan.Steps) {
		goal = e.plan.Steps[e.stepIndex].Goal
	}
	conv := &llm.Conversation{System: screenSystemPrompt}
	conv.AddUser(fmt.Sprintf("Current step goal:\n%s\n\nNavigator message:\n%s", goal, text))
	turn, err := e.screener.Converse(ctx, conv, nil)
	if err != nil {
		// Fail open: screening is a control, not a gate on the session.
		return false
	}
	return strings.Contains(strings.ToUpper(turn.Text), "LEAK")
}

// screenText returns the text to show the learner: unchanged when clean,
// regenerated once when flagged, and delivered with a visible caution if
// the regeneration is flagged again.
func (e *Engine) screenText(ctx context.Context, text string) string {
	if !e.leaks(ctx, text) {
		return text
	}
	e.conv.AddUser("System note: your previous message contained complete code solving the current step, which the typing dial forbids. Rewrite the message now: keep the teaching content and name the constructs to use, but let the learner write the code. Reply with the rewritten message only.")
	turn, err := e.client.Converse(ctx, e.conv, nil)
	if err != nil {
		return text
	}
	if len(turn.ToolCalls) > 0 {
		results := make([]llm.ToolResult, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			results = append(results, llm.ToolResult{ToolCallID: call.ID, Content: "Not executed: this turn only rewrites your previous message. Call the tool again in your next turn if needed."})
		}
		e.conv.AddToolResults(results)
	}
	rewritten := turn.Text
	if strings.TrimSpace(rewritten) == "" {
		return text
	}
	if e.leaks(ctx, rewritten) {
		return "> ⚠️ *Heads-up: this may include more of the solution than intended — try writing your own version before reading closely.*\n\n" + rewritten
	}
	return rewritten
}
