package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultAnthropicModel = "claude-sonnet-5"

type Anthropic struct {
	client anthropic.Client
	model  anthropic.Model
}

func NewAnthropic(model string) (*Anthropic, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set; export your API key, or use a local model with NINA_MODEL=ollama:<model>")
	}
	if model == "" {
		model = defaultAnthropicModel
	}
	return &Anthropic{client: anthropic.NewClient(), model: anthropic.Model(model)}, nil
}

func (a *Anthropic) Converse(ctx context.Context, conv *Conversation, onDelta func(string)) (Turn, error) {
	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: 32000,
		System:    []anthropic.TextBlockParam{{Text: conv.System}},
		Messages:  anthropicMessages(conv),
		Tools:     anthropicTools(),
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
	turn := turnFromMessage(message)
	conv.addAssistantTurn(turn)
	return turn, nil
}

func anthropicMessages(conv *Conversation) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(conv.Messages))
	for _, message := range conv.Messages {
		blocks := []anthropic.ContentBlockParamUnion{}
		if message.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(message.Text))
		}
		for _, call := range message.ToolCalls {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{ID: call.ID, Name: call.Name, Input: call.Input},
			})
		}
		for _, result := range message.ToolResults {
			blocks = append(blocks, anthropic.NewToolResultBlock(result.ToolCallID, result.Content, result.IsError))
		}
		if message.Role == "assistant" {
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		} else {
			out = append(out, anthropic.NewUserMessage(blocks...))
		}
	}
	return out
}

func anthropicTools() []anthropic.ToolUnionParam {
	defs := ToolDefs()
	out := make([]anthropic.ToolUnionParam, 0, len(defs))
	for _, def := range defs {
		tool := anthropic.ToolParam{
			Name:        def.Name,
			Description: anthropic.String(def.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: def.Properties,
				Required:   def.Required,
			},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return out
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
