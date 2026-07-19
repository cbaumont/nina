# Nina

Nina is an AI pair-programming companion for people who want to *learn* to code, not just have code written for them. Nina is the navigator: she plans a small learning project, explains what to do and why, and reviews what you wrote. You are the driver: you type the code in your own editor. A "typing dial" enforced by the engine — not by asking the model nicely — guarantees Nina can't just do it for you.

## Install

Requires Go 1.25+ and git.

```sh
git clone https://github.com/cbaumont/nina
cd nina
go install ./cmd/nina
```

This puts `nina` in `$(go env GOPATH)/bin` (usually `~/go/bin`) — make sure that's in your `PATH`.

## Pick a model

**Claude (default, best quality):**

```sh
export ANTHROPIC_API_KEY=sk-ant-...
```

**Local via Ollama (free, offline):**

```sh
ollama pull gemma4        # or gemma4:e2b for low-RAM machines
export NINA_MODEL=ollama:gemma4
```

The Ollama model must support tool calling (`gemma4`, `qwen3`, `llama3.x` do; plain `gemma3` does not).

## Use

Start in an **empty directory** — Nina scaffolds the project there:

```sh
mkdir learn-python && cd learn-python
nina start "learn Python basics"
```

Nina proposes a tiny project, scaffolds it, and gives you the first step. Then loop:

1. Write the code in your own editor.
2. Type `/done` — Nina diffs what you wrote and reviews it against the step's goal.
3. Pass → next step. Not quite → a Socratic nudge, try again.

Type anything else to ask Nina a question mid-step. `/quit` exits.

## Configuration

| Env var | Meaning | Default |
|---|---|---|
| `NINA_MODEL` | Model: a Claude model name, or `ollama:<model>` | `claude-sonnet-5` |
| `ANTHROPIC_API_KEY` | Claude API key (Claude backend only) | — |
| `NINA_OLLAMA_HOST` | Ollama server URL | `http://localhost:11434` |
| `NINA_OLLAMA_NUM_CTX` | Ollama context window in tokens | `16384` |

Nina snapshots your progress under hidden git refs (`refs/nina/*`) — your `git log` and branches stay untouched.
