from __future__ import annotations

from collections.abc import AsyncIterator, Awaitable, Callable

from claude_agent_sdk import (
    AssistantMessage,
    ClaudeAgentOptions,
    ClaudeSDKClient,
    RateLimitEvent,
    ResultMessage,
    StreamEvent,
    TextBlock,
    create_sdk_mcp_server,
    tool,
)

from nina.agent.session import AgentEvent, RateLimited, TextDelta, ToolHandler, TurnComplete
from nina.agent.tools import TOOL_SPECS
from nina.system import state

MAX_TURNS = 40


def _wrap_tool(handler: ToolHandler) -> Callable[[dict[str, object]], Awaitable[dict[str, object]]]:
    async def run(args: dict[str, object]) -> dict[str, object]:
        result = await handler(args)
        return {
            "content": [{"type": "text", "text": result.content}],
            "is_error": result.is_error,
        }

    return run


class ClaudeSdkAgentSession:
    def __init__(
        self,
        system_prompt: str,
        cwd: str,
        handlers: dict[str, ToolHandler],
        resume: str | None = None,
        model: str | None = None,
    ) -> None:
        sdk_tools = [
            tool(spec.name, spec.description, spec.schema)(_wrap_tool(handlers[spec.name]))
            for spec in TOOL_SPECS
        ]
        server = create_sdk_mcp_server(name="nina", version="0.1.0", tools=sdk_tools)
        options = ClaudeAgentOptions(
            system_prompt=system_prompt,
            tools=[],
            setting_sources=[],
            env={"CLAUDE_CODE_DISABLE_AUTO_MEMORY": "1"},
            mcp_servers={"nina": server},
            allowed_tools=[f"mcp__nina__{spec.name}" for spec in TOOL_SPECS],
            permission_mode="dontAsk",
            include_partial_messages=True,
            max_turns=MAX_TURNS,
            cwd=cwd,
            resume=resume,
            model=model,
        )
        self._client = ClaudeSDKClient(options=options)
        self._connected = False
        self._cwd = cwd
        self.session_id: str | None = None

    async def send(self, text: str) -> AsyncIterator[AgentEvent]:
        state.append_transcript_safe(self._cwd, {"role": "user", "text": text})
        if not self._connected:
            await self._client.connect()
            self._connected = True
        await self._client.query(text)
        async for message in self._client.receive_response():
            if isinstance(message, StreamEvent):
                if message.event.get("type") == "content_block_delta":
                    delta = message.event.get("delta", {})
                    piece = delta.get("text")
                    if piece:
                        yield TextDelta(piece)
            elif isinstance(message, AssistantMessage):
                text_out = "".join(
                    block.text for block in message.content if isinstance(block, TextBlock)
                )
                if text_out:
                    state.append_transcript_safe(
                        self._cwd,
                        {
                            "role": "assistant",
                            "text": text_out,
                            "session_id": message.session_id,
                            "model": message.model,
                        },
                    )
                if message.error == "rate_limit":
                    yield RateLimited(
                        rate_limit_type="unknown",
                        resets_at=None,
                        fatal=True,
                    )
            elif isinstance(message, RateLimitEvent):
                info = message.rate_limit_info
                yield RateLimited(
                    rate_limit_type=info.rate_limit_type or "unknown",
                    resets_at=info.resets_at,
                )
            elif isinstance(message, ResultMessage):
                self.session_id = message.session_id
                state.append_transcript_safe(
                    self._cwd,
                    {
                        "role": "result",
                        "subtype": message.subtype,
                        "session_id": message.session_id,
                    },
                )
        yield TurnComplete()

    async def close(self) -> None:
        if self._connected:
            await self._client.disconnect()
            self._connected = False
