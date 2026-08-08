# Nina

Nina is an AI pair-programming companion for people who want to *learn* to code, not just have code written for them. Nina is the navigator: she plans a small learning project, explains what to do and why, and reviews what you wrote. You are the driver: you type the code in your own editor. A "typing dial" enforced by the engine (not by asking the model nicely) guarantees Nina can't just do it for you.

Nina is built on the [Claude Agent SDK](https://docs.claude.com/en/api/agent-sdk/overview). It runs on top of your local `claude` CLI install and uses whatever credential that CLI already has — including a Claude subscription login, not just an API key. If you're logged into Claude Code with `claude login`, Nina just works; API key, Amazon Bedrock, and Google Vertex auth are equally supported, since Nina is simply the credential the CLI resolves. A local-model option via Ollama is also available (see below) for when you don't want to use Claude at all.

## Install

Requires [uv](https://docs.astral.sh/uv/) and the [Claude Code CLI](https://docs.claude.com/en/docs/claude-code) (`claude`) logged in or otherwise authenticated.

```sh
git clone https://github.com/cbaumont/nina
cd nina
uv sync
```

Run it with `uv run nina` (the console script is named `nina`), or install it as a standalone command on your `PATH`:

```sh
uv tool install --editable .
```

`--editable` keeps `nina` pointed at this checkout, so `git pull` or local edits take effect immediately with no reinstall — only add/remove/version changes to dependencies need `uv tool upgrade nina`. Make sure uv's tool bin directory is on your `PATH` (`uv tool update-shell` sets this up if it isn't).

## Use

Start in an **empty directory**: Nina scaffolds the project there:

```sh
mkdir learn-python && cd learn-python
nina start
```

If you don't pass a goal, Nina asks for one first thing. You can still give it up front the old way:

```sh
nina start "learn Python basics"
```

(If you skipped `uv tool install` and only ran `uv sync`, use `uv run --project /path/to/nina nina start` instead, from any directory.)

On first run Nina then asks a few quick profile questions (experience, how much she may type, how fast hints escalate). She then proposes 2–3 project ideas; pick one and she checks your environment, scaffolds the project, and gives you the first step. Then loop:

1. Write the code in your own editor.
2. Type `/done`: Nina diffs what you wrote, runs it or its tests where possible, and reviews it against the step's goal.
3. Pass → next step. Not quite → a Socratic nudge, try again.

Type anything else to ask Nina a question mid-step. Sessions save to `.nina/`; `nina resume` continues an interrupted session, including its actual conversation memory — not just Nina's plan/step bookkeeping — by resuming the underlying Claude Agent SDK session. Finishing (or `/summary`) writes a learning recap to `.nina/` too.

Pass `--model` to pick which model Nina runs on, e.g. `nina start --model opus "learn Python basics"` (accepts `sonnet`, `opus`, `haiku`, `inherit`, or a full model ID; defaults to the Claude CLI's default). The choice is remembered in `.nina/session.json`, so `nina resume` continues on the same model.

## Local models via Ollama

Pass `--model ollama:<model>` to skip Claude entirely and run Nina against a local model served by [Ollama](https://ollama.com), e.g. `nina start --model ollama:qwen3:8b "learn Python basics"`. This needs `ollama serve` running and the model already pulled; no Claude credential is required or touched. The model needs tool-calling support (Ollama's `tools` API) to drive Nina's plan/write/review loop — most recent instruction-tuned models qualify.

Set `NINA_OLLAMA_HOST` to point at a non-default Ollama host (default `http://localhost:11434`) and `NINA_OLLAMA_NUM_CTX` to override the context window size (default `16384`). The dial-enforced screening classifier also runs on the same local model when the session is on Ollama, so no Claude call happens at any point in an Ollama-backed session.

`nina resume` on an Ollama session replays the saved `.nina/transcript.jsonl` back into a fresh conversation to restore context — there's no server-side session to resume against, unlike the Claude Agent SDK path.

**Commands:** `/done` · `/why` · `/stuck` · `/skip` · `/recap` · `/run [cmd]` · `/summary` · `/dial <0-3>` · `/profile` · `/help` · `/quit`

**The typing dial** is a ceiling on what Nina may write, enforced by the engine: `0` nothing, `1` project scaffold only (default), `2` + boilerplate, `3` collaborative. At levels 0–1 a fast model additionally screens Nina's messages so she doesn't paste the solution into chat.

If you sit idle with uncommitted changes in your editor for a while, Nina will nudge you to run `/done` when you're ready for a review.

Nina snapshots your progress under hidden git refs (`refs/nina/*`); your `git log` and branches stay untouched.

## Credentials

Nina never touches your credentials directly — it spawns the `claude` CLI as a subprocess and inherits whatever auth that CLI already resolved (subscription login, `ANTHROPIC_API_KEY`, Bedrock, or Vertex). On startup Nina runs `claude auth status` and tells you which one is in play. If `ANTHROPIC_API_KEY` is set in your environment, be aware it takes precedence over a subscription login — Nina will warn you when that's the case, since the key silently wins and draws from your API balance instead of your subscription.

## License

[GPLv3](LICENSE).
