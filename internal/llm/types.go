package llm

import (
	"context"
	"encoding/json"
)

type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

type Turn struct {
	Text       string
	ToolCalls  []ToolCall
	StopReason string
}

type Message struct {
	Role        string       `json:"role"`
	Text        string       `json:"text,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

type Conversation struct {
	System   string
	Messages []Message
}

func (c *Conversation) AddUser(text string) {
	c.Messages = append(c.Messages, Message{Role: "user", Text: text})
}

func (c *Conversation) AddToolResults(results []ToolResult) {
	c.Messages = append(c.Messages, Message{Role: "user", ToolResults: results})
}

func (c *Conversation) addAssistantTurn(turn Turn) {
	c.Messages = append(c.Messages, Message{Role: "assistant", Text: turn.Text, ToolCalls: turn.ToolCalls})
}

func (c *Conversation) toolNameFor(toolCallID string) string {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		for _, call := range c.Messages[i].ToolCalls {
			if call.ID == toolCallID {
				return call.Name
			}
		}
	}
	return ""
}

type Client interface {
	Converse(ctx context.Context, conv *Conversation, onDelta func(text string)) (Turn, error)
}
