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

func NewScreener() (Client, error) {
	model := os.Getenv("NINA_MODEL")
	if name, ok := strings.CutPrefix(model, "ollama:"); ok {
		return NewOllama(os.Getenv("NINA_OLLAMA_HOST"), name), nil
	}
	return NewAnthropic(fastAnthropicModel)
}
