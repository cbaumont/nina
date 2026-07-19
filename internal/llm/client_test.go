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

func TestConversationBuilding(t *testing.T) {
	conv := &Conversation{System: "system prompt"}
	conv.AddUser("hello")
	conv.AddToolResults([]ToolResult{{ToolCallID: "toolu_1", Content: "denied", IsError: true}})

	if len(conv.Messages) != 2 {
		t.Fatalf("messages = %d", len(conv.Messages))
	}
	raw, err := json.Marshal(conv.Messages[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tool_result", "toolu_1", "denied"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("tool result message missing %q:\n%s", want, raw)
		}
	}
}
