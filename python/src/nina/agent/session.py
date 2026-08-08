from __future__ import annotations

from collections.abc import AsyncIterator, Awaitable, Callable
from dataclasses import dataclass, field
from typing import Protocol

from nina.agent.tools import ToolResult

ToolHandler = Callable[[dict[str, object]], Awaitable[ToolResult]]


@dataclass
class TextDelta:
    text: str


@dataclass
class MessageComplete:
    text: str


@dataclass
class TurnComplete:
    pass


@dataclass
class RateLimited:
    rate_limit_type: str
    resets_at: float | None
    raw: dict[str, object] = field(default_factory=dict)
    fatal: bool = False


AgentEvent = TextDelta | MessageComplete | TurnComplete | RateLimited


class AgentSession(Protocol):
    session_id: str | None

    def send(self, text: str) -> AsyncIterator[AgentEvent]: ...

    async def close(self) -> None: ...
