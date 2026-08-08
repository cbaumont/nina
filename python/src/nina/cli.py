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
        goal = " ".join(args[1:])
        tui_run(goal, ".", True)
        return
    if command == "resume":
        tui_run("", ".", False)
        return
    _print_usage()
    print(f"nina: unknown command {command!r}", file=sys.stderr)
    sys.exit(1)


def _print_usage() -> None:
    print(
        "nina — AI pair programming companion\n\n"
        "Usage:\n"
        "  nina start                     begin a guided session; Nina asks for your goal first\n"
        '  nina start "<learning goal>"   begin a guided session with a goal already in hand\n'
        "  nina resume                    continue the session saved in .nina/\n"
        "  nina version                   print version\n\n"
        "Nina runs on top of your local Claude Code install and uses whatever\n"
        "credential it already has — including a Claude subscription login."
    )


if __name__ == "__main__":
    main()
