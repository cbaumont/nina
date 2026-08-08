from __future__ import annotations

from collections.abc import AsyncIterator, Awaitable, Callable
from dataclasses import dataclass
from typing import Protocol

from nina.agent.tools import ToolResult

ToolHandler = Callable[[dict[str, object]], Awaitable[ToolResult]]


@dataclass
class TextDelta:
    text: str


@dataclass
class TurnComplete:
    pass


@dataclass
class RateLimited:
    rate_limit_type: str
    resets_at: float | None
    fatal: bool = False


AgentEvent = TextDelta | TurnComplete | RateLimited


class AgentSession(Protocol):
    session_id: str | None

    def send(self, text: str) -> AsyncIterator[AgentEvent]: ...

    async def close(self) -> None: ...
