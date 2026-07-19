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
