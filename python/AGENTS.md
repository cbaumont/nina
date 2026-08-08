# AGENTS.md

### Project

Nina is a CLI AI pair-programming companion for people learning to code: the AI navigates and reviews, the human types. This is the Python port, built on the Claude Agent SDK so Nina can run on a Claude subscription login (not just an API key) by spawning the `claude` CLI as a subprocess. See `../nina-design-doc.md` for the full design and `README.md` for usage. The Go implementation at the repo root is frozen and being removed once this port reaches parity.

* Stack: Python 3.13, `claude-agent-sdk`, `httpx` for the Ollama backend, Textual TUI, `watchdog` for file-idle detection, git plumbing for workspace snapshots.
* Layout: `src/nina/cli.py` (entry point), `src/nina/driver.py` (headless CLI driver wiring engine to stdio); `src/nina/engine/` — step state machine, typing-dial policy, tool handlers (`__init__.py`), system/turn prompts (`prompts.py`), haiku leak classifier (`screening.py`), and engine event/state types (`events.py`); `src/nina/agent/` — the `AgentSession` protocol (`session.py`), its Claude Agent SDK implementation (`claude_sdk.py`) and Ollama HTTP implementation (`ollama.py`, its own tool-call loop since Ollama has no subprocess-side one), the `session_factory()` dispatch on the `--model` prefix (`factory.py`), the `ollama:` prefix helpers (`model.py`), and tool specs (`tools.py`); `src/nina/system/` — supporting infra: profile storage (`profile.py`), session persistence (`state.py`), git workspace snapshots/diffs under `refs/nina/*` (`workspace.py`), subprocess command runner (`runner.py`), file-idle watcher (`watcher.py`), and `claude` CLI credential check (`credentials.py`); `src/nina/tui/` (Textual UI).
* The typing dial is enforced inside tool handler bodies in `engine/__init__.py` (never in the prompt alone) — keep it that way. Same for the leak-screening rewrite gate (`Engine.rewriting`).
* Build/check: `uv run ruff format --check . && uv run ruff check . && uv run mypy --strict src && uv run pytest` (all offline; no credentials needed). CI runs the same checks on every push/PR to `main`.
* Live end-to-end check (needs a logged-in `claude` CLI, draws on your subscription window): `NINA_E2E=1 uv run pytest -k e2e`.
* Console script is `nina`, same name as the Go build — during development, invoke it as `uv run nina` to avoid `PATH` ambiguity.

### Working pattern

* Read nearby source and tests first.
* Make the smallest correct change.
* Run relevant tests.
* Before committing, run the full check line above and fix anything it flags.
* Ensure to always commit your changes.
* Keep commit messages short, in imperative mood, and without prefixes (e.g. `Add user profile validation`).

### Phased Development Workflow

* When creating an implementation plan, break the work into small, independently deliverable phases. Each phase should result in a working, testable increment.
* Commit all completed changes at the end of every phase using a clear, descriptive commit message before moving on to the next phase.

### Testing

* Add or update tests for non-trivial behaviour changes.
* Use a test-first approach whenever possible.
* Prefer `FakeAgentSession` (`tests/fakes.py`) over the real SDK for engine-level tests — it's network-free and scriptable. Reserve real-session checks for manual verification, not the test suite.

### Comments and docstrings

* Zero comments and zero docstrings. Full type annotations and clear naming carry the load instead — if a name needs a comment to be understood, rename it.

### Naming

* Use clear, consistent, descriptive names; avoid unnecessary abbreviations unless they are widely understood.
