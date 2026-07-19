package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

const (
	DefaultOllamaHost = "http://localhost:11434"
	defaultNumCtx     = 16384
)

type Ollama struct {
	host       string
	model      string
	numCtx     int
	httpClient *http.Client
}

func NewOllama(host, model string) *Ollama {
	if host == "" {
		host = DefaultOllamaHost
	}
	numCtx := defaultNumCtx
	if raw := os.Getenv("NINA_OLLAMA_NUM_CTX"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			numCtx = parsed
		}
	}
	return &Ollama{host: host, model: model, numCtx: numCtx, httpClient: http.DefaultClient}
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolName  string           `json:"tool_name,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaChunk struct {
	Message struct {
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls"`
	} `json:"message"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason"`
	Error      string `json:"error"`
}

func (o *Ollama) Converse(ctx context.Context, conv *Conversation, onDelta func(string)) (Turn, error) {
	payload := ollamaRequest{
		Model:    o.model,
		Messages: ollamaMessages(conv),
		Tools:    ollamaTools(),
		Stream:   true,
		Options:  map[string]any{"num_ctx": o.numCtx},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Turn{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, o.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Turn{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := o.httpClient.Do(request)
	if err != nil {
		return Turn{}, fmt.Errorf("calling ollama at %s: %w (is `ollama serve` running?)", o.host, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Turn{}, ollamaError(response)
	}

	turn := Turn{}
	callIndex := 0
	decoder := json.NewDecoder(response.Body)
	for {
		var chunk ollamaChunk
		if err := decoder.Decode(&chunk); err == io.EOF {
			break
		} else if err != nil {
			return Turn{}, fmt.Errorf("reading ollama stream: %w", err)
		}
		if chunk.Error != "" {
			return Turn{}, fmt.Errorf("ollama: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			turn.Text += chunk.Message.Content
			if onDelta != nil {
				onDelta(chunk.Message.Content)
			}
		}
		for _, call := range chunk.Message.ToolCalls {
			turn.ToolCalls = append(turn.ToolCalls, ToolCall{
				ID:    fmt.Sprintf("call_%d_%d", len(conv.Messages), callIndex),
				Name:  call.Function.Name,
				Input: call.Function.Arguments,
			})
			callIndex++
		}
		if chunk.Done {
			turn.StopReason = chunk.DoneReason
			break
		}
	}
	if len(turn.ToolCalls) > 0 {
		turn.StopReason = "tool_use"
	}
	conv.addAssistantTurn(turn)
	return turn, nil
}

func ollamaError(response *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &body) == nil && body.Error != "" {
		return fmt.Errorf("ollama: %s", body.Error)
	}
	return fmt.Errorf("ollama: unexpected status %s: %s", response.Status, raw)
}

func ollamaMessages(conv *Conversation) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(conv.Messages)+1)
	if conv.System != "" {
		out = append(out, ollamaMessage{Role: "system", Content: conv.System})
	}
	for _, message := range conv.Messages {
		if len(message.ToolResults) > 0 {
			for _, result := range message.ToolResults {
				content := result.Content
				if result.IsError {
					content = "Error: " + content
				}
				out = append(out, ollamaMessage{
					Role:     "tool",
					Content:  content,
					ToolName: conv.toolNameFor(result.ToolCallID),
				})
			}
			continue
		}
		converted := ollamaMessage{Role: message.Role, Content: message.Text}
		for _, call := range message.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, ollamaToolCall{
				Function: ollamaFunctionCall{Name: call.Name, Arguments: call.Input},
			})
		}
		out = append(out, converted)
	}
	return out
}

func ollamaTools() []ollamaTool {
	defs := ToolDefs()
	out := make([]ollamaTool, 0, len(defs))
	for _, def := range defs {
		out = append(out, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters: map[string]any{
					"type":       "object",
					"properties": def.Properties,
					"required":   def.Required,
				},
			},
		})
	}
	return out
}
