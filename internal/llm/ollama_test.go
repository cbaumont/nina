package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ollamaServer(t *testing.T, chunks []string, lastRequest *ollamaRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(lastRequest); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
		}
	}))
}

func TestOllamaStreamsTextAndBuildsRequest(t *testing.T) {
	var request ollamaRequest
	server := ollamaServer(t, []string{
		`{"message":{"role":"assistant","content":"Hel"},"done":false}`,
		`{"message":{"role":"assistant","content":"lo"},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
	}, &request)
	defer server.Close()

	client := NewOllama(server.URL, "gemma4")
	conv := &Conversation{System: "be helpful"}
	conv.AddUser("hi")

	var deltas []string
	turn, err := client.Converse(context.Background(), conv, func(text string) {
		deltas = append(deltas, text)
	})
	if err != nil {
		t.Fatalf("Converse: %v", err)
	}

	if turn.Text != "Hello" {
		t.Errorf("text = %q", turn.Text)
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Errorf("deltas = %v", deltas)
	}
	if turn.StopReason != "stop" {
		t.Errorf("stop reason = %q", turn.StopReason)
	}

	if request.Model != "gemma4" || !request.Stream {
		t.Errorf("request = %+v", request)
	}
	if len(request.Messages) != 2 || request.Messages[0].Role != "system" || request.Messages[0].Content != "be helpful" {
		t.Errorf("messages = %+v", request.Messages)
	}
	if len(request.Tools) != len(ToolDefs()) || request.Tools[0].Type != "function" {
		t.Errorf("tools = %+v", request.Tools)
	}
	if request.Options["num_ctx"] == nil {
		t.Error("num_ctx not set")
	}
	if last := conv.Messages[len(conv.Messages)-1]; last.Role != "assistant" || last.Text != "Hello" {
		t.Errorf("assistant turn not appended: %+v", last)
	}
}

func TestOllamaToolCallsAndResults(t *testing.T) {
	var request ollamaRequest
	server := ollamaServer(t, []string{
		`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"set_plan","arguments":{"title":"Game","steps":[]}}}]},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
	}, &request)
	defer server.Close()

	client := NewOllama(server.URL, "gemma4")
	conv := testConversation()

	turn, err := client.Converse(context.Background(), conv, nil)
	if err != nil {
		t.Fatalf("Converse: %v", err)
	}

	if len(turn.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", turn.ToolCalls)
	}
	call := turn.ToolCalls[0]
	if call.ID == "" || call.Name != ToolSetPlan {
		t.Errorf("call = %+v", call)
	}
	if turn.StopReason != "tool_use" {
		t.Errorf("stop reason = %q", turn.StopReason)
	}
	var input SetPlanInput
	if err := json.Unmarshal(call.Input, &input); err != nil || input.Title != "Game" {
		t.Errorf("input = %s (err %v)", call.Input, err)
	}

	var toolMessage *ollamaMessage
	var assistantWithCalls *ollamaMessage
	for i := range request.Messages {
		m := &request.Messages[i]
		if m.Role == "tool" {
			toolMessage = m
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			assistantWithCalls = m
		}
	}
	if assistantWithCalls == nil || assistantWithCalls.ToolCalls[0].Function.Name != ToolWriteFile {
		t.Errorf("assistant tool_calls missing: %+v", request.Messages)
	}
	if toolMessage == nil {
		t.Fatalf("tool result message missing: %+v", request.Messages)
	}
	if toolMessage.ToolName != ToolWriteFile || !strings.HasPrefix(toolMessage.Content, "Error: ") {
		t.Errorf("tool message = %+v", toolMessage)
	}
}

func TestOllamaSurfacesServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"gemma3 does not support tools"}`))
	}))
	defer server.Close()

	client := NewOllama(server.URL, "gemma3")
	conv := &Conversation{}
	conv.AddUser("hi")

	_, err := client.Converse(context.Background(), conv, nil)
	if err == nil || !strings.Contains(err.Error(), "does not support tools") {
		t.Errorf("err = %v", err)
	}
}

func TestNewOllamaDefaults(t *testing.T) {
	client := NewOllama("", "gemma4")
	if client.host != DefaultOllamaHost {
		t.Errorf("host = %q", client.host)
	}
	if client.numCtx != defaultNumCtx {
		t.Errorf("numCtx = %d", client.numCtx)
	}

	t.Setenv("NINA_OLLAMA_NUM_CTX", "4096")
	client = NewOllama("", "gemma4")
	if client.numCtx != 4096 {
		t.Errorf("numCtx override = %d", client.numCtx)
	}
}
