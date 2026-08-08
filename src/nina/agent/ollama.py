from __future__ import annotations

import contextlib
import json
import os
from collections.abc import AsyncIterator

import httpx

from nina.agent.session import AgentEvent, TextDelta, ToolHandler, TurnComplete
from nina.agent.tools import TOOL_SPECS, TOOL_SUBMIT_REVIEW
from nina.system import state

DEFAULT_HOST = "http://localhost:11434"
DEFAULT_NUM_CTX = 16384
MAX_TURNS = 40


def _resolve_host(host: str | None) -> str:
    return host or os.environ.get("NINA_OLLAMA_HOST") or DEFAULT_HOST


def _resolve_num_ctx(num_ctx: int | None) -> int:
    if num_ctx is not None:
        return num_ctx
    raw = os.environ.get("NINA_OLLAMA_NUM_CTX", "")
    if raw:
        with contextlib.suppress(ValueError):
            parsed = int(raw)
            if parsed > 0:
                return parsed
    return DEFAULT_NUM_CTX


def _connect_error(host: str, err: Exception) -> RuntimeError:
    return RuntimeError(f"calling ollama at {host}: {err} (is `ollama serve` running?)")


async def _status_error(response: httpx.Response) -> RuntimeError:
    raw = await response.aread()
    with contextlib.suppress(json.JSONDecodeError):
        body = json.loads(raw)
        message = body.get("error")
        if message:
            return RuntimeError(f"ollama: {message}")
    text = raw.decode(errors="replace")
    return RuntimeError(f"ollama: unexpected status {response.status_code}: {text}")


def _ollama_tools() -> list[dict[str, object]]:
    return [
        {
            "type": "function",
            "function": {
                "name": spec.name,
                "description": spec.description,
                "parameters": spec.schema,
            },
        }
        for spec in TOOL_SPECS
    ]


async def classify(
    system_prompt: str,
    prompt: str,
    model: str,
    host: str | None = None,
    transport: httpx.AsyncBaseTransport | None = None,
) -> str:
    resolved_host = _resolve_host(host)
    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": prompt},
        ],
        "stream": False,
    }
    async with httpx.AsyncClient(timeout=None, transport=transport) as client:
        try:
            response = await client.post(f"{resolved_host}/api/chat", json=payload)
        except httpx.HTTPError as err:
            raise _connect_error(resolved_host, err) from err
        if response.status_code != 200:
            raise await _status_error(response)
        body = response.json()
    error = body.get("error")
    if error:
        raise RuntimeError(f"ollama: {error}")
    return str(body.get("message", {}).get("content", ""))


class OllamaAgentSession:
    def __init__(
        self,
        system_prompt: str,
        cwd: str,
        handlers: dict[str, ToolHandler],
        model: str,
        host: str | None = None,
        num_ctx: int | None = None,
        history: list[dict[str, object]] | None = None,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self._cwd = cwd
        self._handlers = handlers
        self._model = model
        self._host = _resolve_host(host)
        self._num_ctx = _resolve_num_ctx(num_ctx)
        self._transport = transport
        self._messages: list[dict[str, object]] = [{"role": "system", "content": system_prompt}]
        for entry in history or []:
            role, text = entry.get("role"), entry.get("text")
            if role in ("user", "assistant") and text:
                self._messages.append({"role": role, "content": text})
        self.session_id: str | None = None

    async def send(self, text: str) -> AsyncIterator[AgentEvent]:
        state.append_transcript_safe(self._cwd, {"role": "user", "text": text})
        self._messages.append({"role": "user", "content": text})
        async with httpx.AsyncClient(timeout=None, transport=self._transport) as client:
            for _ in range(MAX_TURNS):
                assistant_text = ""
                tool_calls: list[dict[str, object]] = []
                async for event in self._stream_turn(client):
                    if isinstance(event, TextDelta):
                        assistant_text += event.text
                        yield event
                    else:
                        tool_calls.append(event)
                self._messages.append(
                    {
                        "role": "assistant",
                        "content": assistant_text,
                        **({"tool_calls": tool_calls} if tool_calls else {}),
                    }
                )
                if assistant_text:
                    state.append_transcript_safe(
                        self._cwd,
                        {"role": "assistant", "text": assistant_text, "model": self._model},
                    )
                if not tool_calls:
                    break
                reviewed = await self._run_tool_calls(tool_calls)
                if reviewed:
                    break
        yield TurnComplete()

    async def _stream_turn(
        self, client: httpx.AsyncClient
    ) -> AsyncIterator[TextDelta | dict[str, object]]:
        payload = {
            "model": self._model,
            "messages": self._messages,
            "tools": _ollama_tools(),
            "stream": True,
            "options": {"num_ctx": self._num_ctx},
        }
        try:
            async with client.stream("POST", f"{self._host}/api/chat", json=payload) as response:
                if response.status_code != 200:
                    raise await _status_error(response)
                async for line in response.aiter_lines():
                    if not line.strip():
                        continue
                    chunk = json.loads(line)
                    if chunk.get("error"):
                        raise RuntimeError(f"ollama: {chunk['error']}")
                    message = chunk.get("message", {})
                    piece = message.get("content", "")
                    if piece:
                        yield TextDelta(piece)
                    for call in message.get("tool_calls", []) or []:
                        yield call
                    if chunk.get("done"):
                        break
        except httpx.HTTPError as err:
            raise _connect_error(self._host, err) from err

    async def _run_tool_calls(self, tool_calls: list[dict[str, object]]) -> bool:
        reviewed = False
        for call in tool_calls:
            function = call.get("function", {})
            if not isinstance(function, dict):
                function = {}
            name = str(function.get("name", ""))
            raw_args = function.get("arguments", {})
            args = raw_args if isinstance(raw_args, dict) else json.loads(raw_args or "{}")
            handler = self._handlers.get(name)
            if handler is None:
                content, is_error = f"unknown tool {name!r}", True
            else:
                result = await handler(args)
                content, is_error = result.content, result.is_error
            self._messages.append(
                {
                    "role": "tool",
                    "tool_name": name,
                    "content": f"Error: {content}" if is_error else content,
                }
            )
            if name == TOOL_SUBMIT_REVIEW:
                reviewed = True
        return reviewed

    async def close(self) -> None:
        return None
