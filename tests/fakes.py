from __future__ import annotations

from collections.abc import AsyncIterator
from dataclasses import dataclass, field

from nina.agent.session import AgentEvent, RateLimited, TextDelta, ToolHandler, TurnComplete
from nina.agent.tools import TOOL_SUBMIT_REVIEW


@dataclass
class ScriptedToolCall:
    name: str
    input: dict[str, object]


@dataclass
class ScriptedTurn:
    text: str = ""
    tool_calls: list[ScriptedToolCall] = field(default_factory=list)
    rate_limited: float | None | bool = False


class FakeAgentSession:
    def __init__(
        self, handlers: dict[str, ToolHandler], turns: list[ScriptedTurn] | None = None
    ) -> None:
        self._handlers = handlers
        self._turns = turns or []
        self._cursor = 0
        self.calls = 0
        self.session_id: str | None = None

    async def send(self, text: str) -> AsyncIterator[AgentEvent]:
        self.calls += 1
        while True:
            if self._cursor < len(self._turns):
                turn = self._turns[self._cursor]
            else:
                turn = ScriptedTurn(text="ok")
            self._cursor += 1
            if turn.text:
                yield TextDelta(turn.text)
            if turn.rate_limited is not False:
                resets_at = turn.rate_limited if isinstance(turn.rate_limited, float) else None
                yield RateLimited(rate_limit_type="unknown", resets_at=resets_at, fatal=True)
                return
            if not turn.tool_calls:
                break
            reviewed = False
            for call in turn.tool_calls:
                await self._handlers[call.name](call.input)
                if call.name == TOOL_SUBMIT_REVIEW:
                    reviewed = True
            if reviewed:
                break
        yield TurnComplete()

    async def close(self) -> None:
        pass
