from __future__ import annotations

from dataclasses import dataclass


@dataclass
class CommandInfo:
    name: str
    desc: str
    usage: str = ""

    def display(self) -> str:
        return self.usage or self.name


COMMANDS: list[CommandInfo] = [
    CommandInfo("/done", "review the step"),
    CommandInfo("/why", "zoom out"),
    CommandInfo("/stuck", "get help"),
    CommandInfo("/skip", "next step"),
    CommandInfo("/recap", "session recap"),
    CommandInfo("/run", "run code", usage="/run [cmd]"),
    CommandInfo("/summary", "session summary"),
    CommandInfo("/copy", "copy session to clipboard"),
    CommandInfo("/dial", "typing dial", usage="/dial <0-3>"),
    CommandInfo("/auto", "auto-approve commands", usage="/auto <on|off>"),
    CommandInfo("/profile", "adjust profile"),
    CommandInfo("/help", "list commands"),
    CommandInfo("/quit", "exit"),
]


def match_commands(text: str) -> list[CommandInfo]:
    if text == "/":
        return list(COMMANDS)
    return [c for c in COMMANDS if c.name.startswith(text)]
