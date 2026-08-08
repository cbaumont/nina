from __future__ import annotations

from pathlib import Path

from nina.agent.claude_sdk import ClaudeSdkAgentSession
from nina.agent.tools import TOOL_SPECS, ToolResult


async def _handler(args: dict[str, object]) -> ToolResult:
    return ToolResult("ok")


def _handlers() -> dict[str, object]:
    return {spec.name: _handler for spec in TOOL_SPECS}


def test_resume_none_by_default(tmp_path: Path) -> None:
    session = ClaudeSdkAgentSession("system", str(tmp_path), _handlers())
    assert session._client.options.resume is None


def test_resume_passed_through_to_options(tmp_path: Path) -> None:
    session = ClaudeSdkAgentSession("system", str(tmp_path), _handlers(), resume="sdk-session-123")
    assert session._client.options.resume == "sdk-session-123"


def test_model_none_by_default(tmp_path: Path) -> None:
    session = ClaudeSdkAgentSession("system", str(tmp_path), _handlers())
    assert session._client.options.model is None


def test_model_passed_through_to_options(tmp_path: Path) -> None:
    session = ClaudeSdkAgentSession("system", str(tmp_path), _handlers(), model="opus")
    assert session._client.options.model == "opus"


def test_append_transcript_writes_to_workspace(tmp_path: Path) -> None:
    session = ClaudeSdkAgentSession("system", str(tmp_path), _handlers())
    session._append_transcript({"role": "user", "text": "hi"})
    path = tmp_path / ".nina" / "transcript.jsonl"
    assert path.exists()
    assert "hi" in path.read_text()
