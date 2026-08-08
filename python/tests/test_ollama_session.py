from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest

from nina.agent.model import is_ollama, ollama_model_name
from nina.agent.ollama import OllamaAgentSession, classify
from nina.agent.session import TextDelta, TurnComplete
from nina.agent.tools import ToolResult


def _ndjson(*chunks: dict[str, object]) -> bytes:
    return b"".join((json.dumps(c) + "\n").encode() for c in chunks)


def _text_chunk(piece: str, done: bool = False) -> dict[str, object]:
    chunk: dict[str, object] = {"message": {"content": piece}, "done": done}
    if done:
        chunk["done_reason"] = "stop"
    return chunk


def _tool_call_chunk(name: str, arguments: dict[str, object]) -> dict[str, object]:
    return {
        "message": {
            "content": "",
            "tool_calls": [{"function": {"name": name, "arguments": arguments}}],
        },
        "done": True,
        "done_reason": "tool_calls",
    }


async def _write_ok(args: dict[str, object]) -> ToolResult:
    return ToolResult(f"wrote {args.get('path')}")


def test_is_ollama_and_name_parsing() -> None:
    assert is_ollama("ollama:gemma4") is True
    assert is_ollama("opus") is False
    assert is_ollama(None) is False
    assert ollama_model_name("ollama:gemma4:2b") == "gemma4:2b"


async def test_send_streams_text_deltas_and_completes(tmp_path: Path) -> None:
    responses = [
        httpx.Response(200, content=_ndjson(_text_chunk("hello "), _text_chunk("world", done=True)))
    ]

    def handler(request: httpx.Request) -> httpx.Response:
        return responses.pop(0)

    transport = httpx.MockTransport(handler)
    session = OllamaAgentSession(
        "sys prompt", str(tmp_path), {}, model="gemma4", transport=transport
    )

    events = [event async for event in session.send("hi")]
    assert events == [TextDelta("hello "), TextDelta("world"), TurnComplete()]
    assert session._messages[-1] == {"role": "assistant", "content": "hello world"}


async def test_send_runs_tool_call_then_final_answer(tmp_path: Path) -> None:
    first = httpx.Response(200, content=_ndjson(_tool_call_chunk("write_file", {"path": "a.py"})))
    second = httpx.Response(200, content=_ndjson(_text_chunk("done", done=True)))
    responses = [first, second]

    def handler(request: httpx.Request) -> httpx.Response:
        return responses.pop(0)

    transport = httpx.MockTransport(handler)
    session = OllamaAgentSession(
        "sys prompt",
        str(tmp_path),
        {"write_file": _write_ok},
        model="gemma4",
        transport=transport,
    )

    events = [event async for event in session.send("write it")]
    assert TextDelta("done") in events
    assert events[-1] == TurnComplete()
    tool_messages = [m for m in session._messages if m.get("role") == "tool"]
    assert tool_messages == [{"role": "tool", "tool_name": "write_file", "content": "wrote a.py"}]


async def test_send_stops_after_submit_review_even_with_more_calls(tmp_path: Path) -> None:
    chunk = {
        "message": {
            "content": "",
            "tool_calls": [
                {
                    "function": {
                        "name": "submit_review",
                        "arguments": {"verdict": "pass", "feedback": "ok"},
                    }
                },
                {"function": {"name": "write_file", "arguments": {"path": "b.py"}}},
            ],
        },
        "done": True,
    }
    responses = [httpx.Response(200, content=_ndjson(chunk))]

    def handler(request: httpx.Request) -> httpx.Response:
        return responses.pop(0)

    called = False

    async def write_file(args: dict[str, object]) -> ToolResult:
        nonlocal called
        called = True
        return ToolResult("wrote")

    async def submit_review(args: dict[str, object]) -> ToolResult:
        return ToolResult("verdict recorded")

    transport = httpx.MockTransport(handler)
    session = OllamaAgentSession(
        "sys prompt",
        str(tmp_path),
        {"submit_review": submit_review, "write_file": write_file},
        model="gemma4",
        transport=transport,
    )

    events = [event async for event in session.send("review it")]
    assert events == [TurnComplete()]
    assert called is True
    assert len(responses) == 0


async def test_send_raises_on_non_200(tmp_path: Path) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(500, json={"error": "model not found"})

    transport = httpx.MockTransport(handler)
    session = OllamaAgentSession("sys", str(tmp_path), {}, model="missing", transport=transport)

    with pytest.raises(RuntimeError, match="model not found"):
        async for _ in session.send("hi"):
            pass


async def test_send_raises_helpful_error_on_connect_failure(tmp_path: Path) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused", request=request)

    transport = httpx.MockTransport(handler)
    session = OllamaAgentSession("sys", str(tmp_path), {}, model="gemma4", transport=transport)

    with pytest.raises(RuntimeError, match="ollama serve"):
        async for _ in session.send("hi"):
            pass


def test_history_seeds_prior_messages(tmp_path: Path) -> None:
    history: list[dict[str, object]] = [
        {"role": "user", "text": "hello"},
        {"role": "assistant", "text": "hi there", "model": "gemma4"},
        {"role": "result", "session_id": "ignored"},
    ]
    session = OllamaAgentSession("sys prompt", str(tmp_path), {}, model="gemma4", history=history)
    assert session._messages == [
        {"role": "system", "content": "sys prompt"},
        {"role": "user", "content": "hello"},
        {"role": "assistant", "content": "hi there"},
    ]


def test_append_transcript_writes_to_workspace(tmp_path: Path) -> None:
    session = OllamaAgentSession("sys", str(tmp_path), {}, model="gemma4")
    session._append_transcript({"role": "user", "text": "hi"})
    path = tmp_path / ".nina" / "transcript.jsonl"
    assert json.loads(path.read_text().strip()) == {"role": "user", "text": "hi"}


async def test_classify_returns_message_content() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert json.loads(request.content)["stream"] is False
        return httpx.Response(200, json={"message": {"content": "LEAK"}})

    transport = httpx.MockTransport(handler)
    reply = await classify("system", "prompt", "gemma4", transport=transport)
    assert reply == "LEAK"


async def test_classify_raises_on_error_field() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"error": "boom"})

    transport = httpx.MockTransport(handler)
    with pytest.raises(RuntimeError, match="boom"):
        await classify("system", "prompt", "gemma4", transport=transport)


def test_host_and_num_ctx_from_env(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setenv("NINA_OLLAMA_HOST", "http://example.test:1234")
    monkeypatch.setenv("NINA_OLLAMA_NUM_CTX", "8192")
    session = OllamaAgentSession("sys", str(tmp_path), {}, model="gemma4")
    assert session._host == "http://example.test:1234"
    assert session._num_ctx == 8192


def test_host_and_num_ctx_defaults(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.delenv("NINA_OLLAMA_HOST", raising=False)
    monkeypatch.delenv("NINA_OLLAMA_NUM_CTX", raising=False)
    session = OllamaAgentSession("sys", str(tmp_path), {}, model="gemma4")
    assert session._host == "http://localhost:11434"
    assert session._num_ctx == 16384
