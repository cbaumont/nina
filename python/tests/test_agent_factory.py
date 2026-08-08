from __future__ import annotations

from pathlib import Path

from nina.agent.claude_sdk import ClaudeSdkAgentSession
from nina.agent.factory import session_factory
from nina.agent.ollama import OllamaAgentSession
from nina.agent.tools import TOOL_SPECS, ToolResult
from nina.system import state


async def _handler(args: dict[str, object]) -> ToolResult:
    return ToolResult("ok")


def _handlers() -> dict[str, object]:
    return {spec.name: _handler for spec in TOOL_SPECS}


def test_factory_builds_claude_session_for_non_ollama_model(tmp_path: Path) -> None:
    factory = session_factory(str(tmp_path), "opus", resume="sdk-123")
    session = factory("system prompt", _handlers())
    assert isinstance(session, ClaudeSdkAgentSession)
    assert session._client.options.model == "opus"
    assert session._client.options.resume == "sdk-123"


def test_factory_builds_claude_session_for_none_model(tmp_path: Path) -> None:
    factory = session_factory(str(tmp_path), None)
    session = factory("system prompt", _handlers())
    assert isinstance(session, ClaudeSdkAgentSession)


def test_factory_builds_ollama_session_for_ollama_model(tmp_path: Path) -> None:
    factory = session_factory(str(tmp_path), "ollama:gemma4")
    session = factory("system prompt", {})
    assert isinstance(session, OllamaAgentSession)
    assert session._model == "gemma4"


def test_factory_replays_transcript_history_for_ollama(tmp_path: Path) -> None:
    state.append_transcript(str(tmp_path), {"role": "user", "text": "earlier question"})
    factory = session_factory(str(tmp_path), "ollama:gemma4", replay_history=True)
    session = factory("system prompt", {})
    assert isinstance(session, OllamaAgentSession)
    assert {"role": "user", "content": "earlier question"} in session._messages


def test_factory_does_not_replay_history_by_default(tmp_path: Path) -> None:
    state.append_transcript(str(tmp_path), {"role": "user", "text": "earlier question"})
    factory = session_factory(str(tmp_path), "ollama:gemma4")
    session = factory("system prompt", {})
    assert isinstance(session, OllamaAgentSession)
    assert session._messages == [{"role": "system", "content": "system prompt"}]
