package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cbaumont/nina/internal/tui"
)

const version = "0.0.1-skeleton"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "nina:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println("nina", version)
		return nil
	case "start":
		if len(args) < 2 {
			return fmt.Errorf("start requires a learning goal, e.g. nina start \"learn Python basics\"")
		}
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		return tui.Run(strings.Join(args[1:], " "), dir)
	case "resume":
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		return tui.Run("", dir)
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println(`nina — AI pair programming companion

Usage:
  nina start "<learning goal>"   begin a guided session in the current directory
  nina resume                    continue the session saved in .nina/
  nina version                   print version

Models:
  Claude (default)   requires ANTHROPIC_API_KEY; pick a model with NINA_MODEL
  Ollama (local)     NINA_MODEL=ollama:<model>, e.g. ollama:gemma4
                     host via NINA_OLLAMA_HOST (default http://localhost:11434)
                     the model must support tool calling (gemma4, qwen3,
                     llama3.x do; plain gemma3 does not)`)
}
