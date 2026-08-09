from __future__ import annotations

import sys

from nina.tui.run import run as tui_run

VERSION = "0.0.1-python-skeleton"


def main() -> None:
    run_args(sys.argv[1:])


def run_args(args: list[str]) -> None:
    if not args:
        _print_usage()
        return
    command = args[0]
    if command == "version":
        print(f"nina {VERSION}")
        return
    if command == "start":
        rest, model = _extract_model_flag(args[1:])
        rest, auto = _extract_auto_flag(rest)
        goal = " ".join(rest)
        tui_run(goal, ".", True, model, auto)
        return
    if command == "resume":
        tui_run("", ".", False)
        return
    _print_usage()
    print(f"nina: unknown command {command!r}", file=sys.stderr)
    sys.exit(1)


def _extract_model_flag(args: list[str]) -> tuple[list[str], str | None]:
    rest: list[str] = []
    model: str | None = None
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--model":
            if i + 1 >= len(args):
                print("nina: --model requires a value", file=sys.stderr)
                sys.exit(1)
            model = args[i + 1]
            i += 2
            continue
        if arg.startswith("--model="):
            model = arg.split("=", 1)[1]
            i += 1
            continue
        rest.append(arg)
        i += 1
    return rest, model


def _extract_auto_flag(args: list[str]) -> tuple[list[str], bool]:
    rest: list[str] = []
    auto = False
    for arg in args:
        if arg == "--auto":
            auto = True
            continue
        rest.append(arg)
    return rest, auto


def _print_usage() -> None:
    print(
        "nina — AI pair programming companion\n\n"
        "Usage:\n"
        "  nina start                     begin a guided session; Nina asks for your goal first\n"
        '  nina start "<learning goal>"   begin a guided session with a goal already in hand\n'
        "  nina start --model opus ...    use a specific model (sonnet, opus, haiku, or a full\n"
        "                                  model ID); remembered for nina resume\n"
        "  nina start --model ollama:<m>  run entirely on a local model via `ollama serve`\n"
        "                                  (set NINA_OLLAMA_HOST to override localhost:11434)\n"
        "  nina start --auto ...          skip approval prompts before Nina runs commands;\n"
        "                                  remembered for nina resume, toggle anytime with /auto\n"
        "  nina resume                    continue the session saved in .nina/\n"
        "  nina version                   print version\n\n"
        "Nina runs on top of your local Claude Code install and uses whatever\n"
        "credential it already has — including a Claude subscription login. Use\n"
        "--model ollama:<model> to skip Claude entirely and run on a local Ollama model."
    )


if __name__ == "__main__":
    main()
