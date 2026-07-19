package llm

import (
	"context"
	"encoding/json"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultModel = "claude-sonnet-5"

type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

type Turn struct {
	Text       string
	ToolCalls  []ToolCall
	StopReason string
}

type Conversation struct {
	System   string
	Messages []anthropic.MessageParam
}

func (c *Conversation) AddUser(text string) {
	c.Messages = append(c.Messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
}

func (c *Conversation) AddToolResults(results []ToolResult) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(results))
	for _, r := range results {
		blocks = append(blocks, anthropic.NewToolResultBlock(r.ToolCallID, r.Content, r.IsError))
	}
	c.Messages = append(c.Messages, anthropic.NewUserMessage(blocks...))
}

type Client interface {
	Converse(ctx context.Context, conv *Conversation, onDelta func(text string)) (Turn, error)
}

type Anthropic struct {
	client anthropic.Client
	model  anthropic.Model
}

func NewAnthropic() *Anthropic {
	model := defaultModel
	if m := os.Getenv("NINA_MODEL"); m != "" {
		model = m
	}
	return &Anthropic{client: anthropic.NewClient(), model: anthropic.Model(model)}
}

func (a *Anthropic) Converse(ctx context.Context, conv *Conversation, onDelta func(string)) (Turn, error) {
	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: 32000,
		System:    []anthropic.TextBlockParam{{Text: conv.System}},
		Messages:  conv.Messages,
		Tools:     toolDefinitions(),
	}
	stream := a.client.Messages.NewStreaming(ctx, params)
	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return Turn{}, err
		}
		if deltaEvent, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if textDelta, ok := deltaEvent.Delta.AsAny().(anthropic.TextDelta); ok && onDelta != nil {
				onDelta(textDelta.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Turn{}, err
	}
	conv.Messages = append(conv.Messages, message.ToParam())
	return turnFromMessage(message), nil
}

func turnFromMessage(message anthropic.Message) Turn {
	turn := Turn{StopReason: string(message.StopReason)}
	for _, block := range message.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			turn.Text += variant.Text
		case anthropic.ToolUseBlock:
			turn.ToolCalls = append(turn.ToolCalls, ToolCall{
				ID:    variant.ID,
				Name:  variant.Name,
				Input: json.RawMessage(variant.JSON.Input.Raw()),
			})
		}
	}
	return turn
}
