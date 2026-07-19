package llm

import (
	"strings"
	"testing"
)

func TestNewSelectsOllama(t *testing.T) {
	t.Setenv("NINA_MODEL", "ollama:gemma4")
	t.Setenv("NINA_OLLAMA_HOST", "http://example.test:1234")

	client, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ollama, ok := client.(*Ollama)
	if !ok {
		t.Fatalf("client = %T", client)
	}
	if ollama.model != "gemma4" || ollama.host != "http://example.test:1234" {
		t.Errorf("ollama = %+v", ollama)
	}
}

func TestNewSelectsAnthropic(t *testing.T) {
	t.Setenv("NINA_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")

	client, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := client.(*Anthropic); !ok {
		t.Fatalf("client = %T", client)
	}
}

func TestNewAnthropicRequiresKey(t *testing.T) {
	t.Setenv("NINA_MODEL", "claude-sonnet-5")
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := New()
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("err = %v", err)
	}
}
