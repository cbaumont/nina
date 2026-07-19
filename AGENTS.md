# AGENTS.md

### Project

Nina is a CLI AI pair-programming companion for people learning to code: the AI navigates and reviews, the human types. See `nina-design-doc.md` for the full design and `README.md` for usage.

* Stack: Go (module `github.com/cbaumont/nina`), Bubble Tea + Glamour TUI, git plumbing for workspace snapshots.
* Layout: `cmd/nina` (entry point), `internal/engine` (step state machine, typing-dial policy, prompts), `internal/llm` (provider-neutral conversation + Anthropic and Ollama backends), `internal/workspace` (git snapshots/diffs under `refs/nina/*`), `internal/tui`.
* The typing dial is enforced in `internal/engine` (tool-call filter), never in the prompt alone — keep it that way.
* Build/check: `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` (all offline; no API key needed).
* Live end-to-end test (needs a running model): `NINA_E2E=1 NINA_MODEL=ollama:gemma4:e2b go test ./internal/engine/ -run TestEndToEndAgainstRealModel -v`.

### Working pattern

* Read nearby source and tests first.
* Make the smallest correct change.
* Run relevant tests.
* Ensure to always commit your changes.
* Keep commit messages short, in imperative mood, and without prefixes (e.g. `Add user profile validation`).

### Phased Development Workflow

* When creating an implementation plan, break the work into small, independently deliverable phases. Each phase should result in a working, testable increment.
* Commit all completed changes at the end of every phase using a clear, descriptive commit message before moving on to the next phase.

### Testing

* Add or update tests for non-trivial behaviour changes.
* Use a test-first approach whenever possible.

### Comments

* Real programmers (and agents) don't use comments. The code should be obvious.

### Naming

* Use clear, consistent, descriptive names; avoid unnecessary abbreviations unless they are widely understood.
