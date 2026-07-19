package llm

import (
	"os"
	"strings"
)

func New() (Client, error) {
	model := os.Getenv("NINA_MODEL")
	if name, ok := strings.CutPrefix(model, "ollama:"); ok {
		return NewOllama(os.Getenv("NINA_OLLAMA_HOST"), name), nil
	}
	return NewAnthropic(model)
}

const fastAnthropicModel = "claude-haiku-4-5"

// NewScreener returns the fast-tier client used for lightweight
// classification (dial screening): Haiku on the Anthropic backend, the
// same local model on Ollama (a second local model is rarely loaded).
func NewScreener() (Client, error) {
	model := os.Getenv("NINA_MODEL")
	if name, ok := strings.CutPrefix(model, "ollama:"); ok {
		return NewOllama(os.Getenv("NINA_OLLAMA_HOST"), name), nil
	}
	return NewAnthropic(fastAnthropicModel)
}
