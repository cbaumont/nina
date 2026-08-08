from __future__ import annotations

from collections.abc import Sequence
from pathlib import Path

from textual.widgets import Static

from nina.agent.tools import TOOL_RUN_COMMAND, TOOL_SET_PLAN, TOOL_WRITE_FILE
from nina.engine import Engine, new_engine
from nina.engine.events import STATE_DRIVE, STATE_PROPOSE
from nina.system.profile import default as default_profile
from nina.system.workspace import open_workspace
from nina.tui.app import NinaApp
from tests.fakes import FakeAgentSession, ScriptedToolCall, ScriptedTurn


def build_app(
    tmp_path: Path,
    turns: Sequence[ScriptedTurn],
    goal: str = "learn python",
    need_setup: bool = False,
    need_goal: bool = False,
    prior_history: str = "",
) -> NinaApp:
    dir = str(tmp_path)
    ws = open_workspace(dir)
    engine: Engine = new_engine(
        ws, dir, default_profile(), lambda ev: None, lambda sp, h: FakeAgentSession(h, list(turns))
    )
    return NinaApp(engine, goal, dir, need_setup, need_goal, prior_history)


async def test_start_flow_proposes_ideas(tmp_path: Path) -> None:
    turns = [ScriptedTurn(text="Idea 1: guessing game. Idea 2: dice roller. Which one?")]
    app = build_app(tmp_path, turns)
    async with app.run_test() as pilot:
        await pilot.pause()
        assert app.engine.state == STATE_PROPOSE
        assert "Idea 1" in app.history
        assert not app.busy


async def test_choosing_idea_scaffolds_and_starts_drive(tmp_path: Path) -> None:
    turns = [
        ScriptedTurn(text="Idea 1 or idea 2?"),
        ScriptedTurn(
            tool_calls=[
                ScriptedToolCall(
                    TOOL_SET_PLAN,
                    {
                        "title": "Guessing Game",
                        "steps": [{"title": "Read input", "goal": "reads a number"}],
                    },
                ),
                ScriptedToolCall(TOOL_WRITE_FILE, {"path": "main.py", "content": "# stub\n"}),
            ]
        ),
        ScriptedTurn(text="Step 1: read input."),
    ]
    app = build_app(tmp_path, turns)
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.press(*"the guessing game", "enter")
        await pilot.pause()
        assert app.engine.state == STATE_DRIVE
        assert app.plan_title == "Guessing Game"


async def test_setup_flow_then_starts(tmp_path: Path) -> None:
    turns = [ScriptedTurn(text="Ideas here")]
    app = build_app(tmp_path, turns, need_setup=True)
    async with app.run_test() as pilot:
        await pilot.pause()
        assert app.setup is not None
        for _ in range(5):
            await pilot.press("enter")
            await pilot.pause()
        assert app.setup is None
        assert app.engine.state == STATE_PROPOSE


async def test_awaiting_goal_then_starts(tmp_path: Path) -> None:
    turns = [ScriptedTurn(text="Ideas here")]
    app = build_app(tmp_path, turns, goal="", need_goal=True)
    async with app.run_test() as pilot:
        await pilot.pause()
        assert app.awaiting_goal
        await pilot.press(*"learn python", "enter")
        await pilot.pause()
        assert not app.awaiting_goal
        assert app.engine.state == STATE_PROPOSE


async def test_confirm_flow_approves_command(tmp_path: Path) -> None:
    turns = [
        ScriptedTurn(text="Idea 1 or idea 2?"),
        ScriptedTurn(
            tool_calls=[ScriptedToolCall(TOOL_RUN_COMMAND, {"command": "true", "reason": "check"})]
        ),
    ]
    app = build_app(tmp_path, turns)
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.press(*"go ahead", "enter")
        await pilot.pause()
        assert app.pending_confirm is not None
        await pilot.press("y", "enter")
        await pilot.pause()
        assert app.pending_confirm is None


async def test_dial_command_updates_profile(tmp_path: Path) -> None:
    turns = [ScriptedTurn(text="Idea 1 or idea 2?")]
    app = build_app(tmp_path, turns)
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.press(*"/dial 0", "enter")
        await pilot.pause()
        assert app.engine.profile.dial == 0


async def test_slash_suggestions_appear(tmp_path: Path) -> None:
    turns = [ScriptedTurn(text="Idea 1 or idea 2?")]
    app = build_app(tmp_path, turns)
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.press("/")
        await pilot.pause()
        panel = app.query_one("#suggestions", Static)
        assert "/done" in str(panel.content)


async def test_quit_command_exits(tmp_path: Path) -> None:
    turns = [ScriptedTurn(text="Idea 1 or idea 2?")]
    app = build_app(tmp_path, turns)
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.press(*"/quit", "enter")
        await pilot.pause()
        assert not app.is_running
