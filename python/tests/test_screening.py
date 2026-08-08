from __future__ import annotations

import pytest

from nina.engine import screening
from nina.engine.events import STATE_DRIVE, STATE_SCAFFOLD


def test_is_active_only_at_low_dial_in_drive() -> None:
    assert screening.is_active(0, STATE_DRIVE)
    assert screening.is_active(1, STATE_DRIVE)
    assert not screening.is_active(2, STATE_DRIVE)
    assert not screening.is_active(1, STATE_SCAFFOLD)


async def test_leaks_skips_classification_without_fenced_code(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def fail_if_called(*args: object, **kwargs: object) -> None:
        raise AssertionError("query() should not be called for text with no code fence")

    monkeypatch.setattr(screening, "query", fail_if_called)
    assert await screening.leaks("goal", "just guidance, no code") is False


async def test_leaks_skips_classification_for_single_line_fragment(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def fail_if_called(*args: object, **kwargs: object) -> None:
        raise AssertionError("query() should not be called for a single-line fragment")

    monkeypatch.setattr(screening, "query", fail_if_called)
    assert await screening.leaks("goal", "```py\nimport os\n```") is False


async def test_leaks_skips_classification_for_blank_fenced_block(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def fail_if_called(*args: object, **kwargs: object) -> None:
        raise AssertionError("query() should not be called for a blank fenced block")

    monkeypatch.setattr(screening, "query", fail_if_called)
    assert await screening.leaks("goal", "```py\n\n   \n```") is False


async def test_leaks_classifies_multi_line_fragment(monkeypatch: pytest.MonkeyPatch) -> None:
    from claude_agent_sdk import AssistantMessage, TextBlock

    calls = []

    async def fake_query(*, prompt: str, options: object) -> object:
        calls.append(prompt)
        yield AssistantMessage(content=[TextBlock(text="OK")], model="test")

    monkeypatch.setattr(screening, "query", fake_query)
    result = await screening.leaks("goal", "```py\nx = 1\ny = 2\n```")
    assert result is False
    assert len(calls) == 1


async def test_screen_text_returns_original_when_not_leaking(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def not_leaking(step_goal: str, text: str) -> bool:
        return False

    monkeypatch.setattr(screening, "leaks", not_leaking)

    async def rewrite_should_not_be_called(instruction: str) -> str:
        raise AssertionError("rewrite should not be called when text does not leak")

    result = await screening.screen_text("goal", "```py\nx = 1\n```", rewrite_should_not_be_called)
    assert result == "```py\nx = 1\n```"


async def test_screen_text_rewrites_when_leaking(monkeypatch: pytest.MonkeyPatch) -> None:
    calls = iter([True, False])

    async def fake_leaks(step_goal: str, text: str) -> bool:
        return next(calls)

    monkeypatch.setattr(screening, "leaks", fake_leaks)

    async def rewrite(instruction: str) -> str:
        assert instruction == screening.REWRITE_NOTE
        return "here are the constructs to use"

    result = await screening.screen_text("goal", "```py\nsolution\n```", rewrite)
    assert result == "here are the constructs to use"


async def test_screen_text_warns_when_rewrite_still_leaks(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def always_leaks(step_goal: str, text: str) -> bool:
        return True

    monkeypatch.setattr(screening, "leaks", always_leaks)

    async def rewrite(instruction: str) -> str:
        return "still has the answer"

    result = await screening.screen_text("goal", "```py\nsolution\n```", rewrite)
    assert result.startswith(screening.LEAK_WARNING)
    assert "still has the answer" in result


async def test_screen_text_falls_back_to_original_on_empty_rewrite(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def always_leaks(step_goal: str, text: str) -> bool:
        return True

    monkeypatch.setattr(screening, "leaks", always_leaks)

    async def empty_rewrite(instruction: str) -> str:
        return "   "

    result = await screening.screen_text("goal", "```py\nsolution\n```", empty_rewrite)
    assert result == "```py\nsolution\n```"
