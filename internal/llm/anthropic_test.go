package llm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

const messageFixture = `{
	"id": "msg_test",
	"type": "message",
	"role": "assistant",
	"model": "claude-sonnet-5",
	"stop_reason": "tool_use",
	"content": [
		{"type": "text", "text": "Scaffolding the project."},
		{"type": "tool_use", "id": "toolu_1", "name": "write_file",
		 "input": {"path": "main.py", "content": "print('hi')\n"}}
	],
	"usage": {"input_tokens": 10, "output_tokens": 20}
}`

func TestTurnFromMessage(t *testing.T) {
	var message anthropic.Message
	if err := json.Unmarshal([]byte(messageFixture), &message); err != nil {
		t.Fatal(err)
	}
	turn := turnFromMessage(message)

	if turn.Text != "Scaffolding the project." {
		t.Errorf("text = %q", turn.Text)
	}
	if turn.StopReason != "tool_use" {
		t.Errorf("stop reason = %q", turn.StopReason)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(turn.ToolCalls))
	}
	call := turn.ToolCalls[0]
	if call.ID != "toolu_1" || call.Name != ToolWriteFile {
		t.Errorf("call = %+v", call)
	}
	var input WriteFileInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if input.Path != "main.py" {
		t.Errorf("path = %q", input.Path)
	}
}

func testConversation() *Conversation {
	conv := &Conversation{System: "system prompt"}
	conv.AddUser("hello")
	conv.addAssistantTurn(Turn{
		Text:      "Writing a file.",
		ToolCalls: []ToolCall{{ID: "toolu_1", Name: ToolWriteFile, Input: json.RawMessage(`{"path":"main.py","content":"x"}`)}},
	})
	conv.AddToolResults([]ToolResult{{ToolCallID: "toolu_1", Content: "denied", IsError: true}})
	return conv
}

func TestAnthropicMessageConversion(t *testing.T) {
	params := anthropicMessages(testConversation())
	if len(params) != 3 {
		t.Fatalf("messages = %d", len(params))
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(raw)
	for _, want := range []string{
		`"hello"`,
		`"tool_use"`, `"toolu_1"`, `"write_file"`, `"main.py"`,
		`"tool_result"`, `"denied"`, `"is_error":true`,
		`"role":"assistant"`,
	} {
		if !strings.Contains(wire, want) {
			t.Errorf("wire missing %s:\n%s", want, wire)
		}
	}
}

func TestAnthropicToolConversion(t *testing.T) {
	tools := anthropicTools()
	if len(tools) != 3 {
		t.Fatalf("tools = %d", len(tools))
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(raw)
	for _, want := range []string{ToolWriteFile, ToolSetPlan, ToolSubmitReview, `"required":["path","content"]`, `"input_schema"`} {
		if !strings.Contains(wire, want) {
			t.Errorf("wire missing %s:\n%s", want, wire)
		}
	}
}

func TestToolNameFor(t *testing.T) {
	conv := testConversation()
	if name := conv.toolNameFor("toolu_1"); name != ToolWriteFile {
		t.Errorf("toolNameFor = %q", name)
	}
	if name := conv.toolNameFor("missing"); name != "" {
		t.Errorf("toolNameFor(missing) = %q", name)
	}
}
