from __future__ import annotations

from pathlib import Path

import pytest

from nina.agent.tools import TOOL_SET_PLAN, TOOL_SUBMIT_REVIEW, TOOL_WRITE_FILE
from nina.engine import Engine, RateLimitExceeded, new_engine
from nina.engine import screening as screening_module
from nina.engine.events import (
    EVENT_COMMAND_RUN,
    EVENT_CONFIRM,
    EVENT_INFO,
    EVENT_PLAN_SET,
    EVENT_REVIEW,
    EVENT_SESSION_DONE,
    EVENT_STEP_STARTED,
    EVENT_TEXT_DELTA,
    STATE_DONE,
    STATE_DRIVE,
    STATE_PROPOSE,
    STATE_SCAFFOLD,
    ConfirmAnswer,
    Event,
)
from nina.system import state
from nina.system.profile import default as default_profile
from nina.system.workspace import open_workspace
from tests.fakes import FakeAgentSession, ScriptedToolCall, ScriptedTurn


def new_test_engine(
    tmp_path: Path, turns: list[ScriptedTurn] | None = None
) -> tuple[Engine, str, list[Event]]:
    dir = str(tmp_path)
    ws = open_workspace(dir)
    events: list[Event] = []
    engine = new_engine(
        ws, dir, default_profile(), events.append, lambda sp, h: FakeAgentSession(h, turns)
    )
    return engine, dir, events


def plan_call() -> ScriptedToolCall:
    return ScriptedToolCall(
        TOOL_SET_PLAN,
        {
            "title": "Guessing Game",
            "steps": [
                {"title": "Read input", "goal": "Program reads a number from the user"},
                {"title": "Compare", "goal": "Program says higher or lower"},
            ],
        },
    )


async def started_engine(
    tmp_path: Path, extra_turns: list[ScriptedTurn] | None = None
) -> tuple[Engine, str, list[Event]]:
    turns = [
        ScriptedTurn(text="Idea 1: a guessing game. Idea 2: a dice roller. Which one?"),
        ScriptedTurn(
            tool_calls=[
                plan_call(),
                ScriptedToolCall(TOOL_WRITE_FILE, {"path": "main.py", "content": "# stub\n"}),
            ]
        ),
        ScriptedTurn(text="Step 1: read input."),
        *(extra_turns or []),
    ]
    engine, dir, events = new_test_engine(tmp_path, turns)
    await engine.start("learn python")
    await engine.user_message("the guessing game")
    return engine, dir, events


def confirming_engine(tmp_path: Path, answer: ConfirmAnswer) -> tuple[Engine, str, list[Event]]:
    dir = str(tmp_path)
    ws = open_workspace(dir)
    events: list[Event] = []

    def emit(ev: Event) -> None:
        events.append(ev)
        if ev.kind == EVENT_CONFIRM:
            assert ev.confirm is not None
            ev.confirm.reply.set_result(answer)

    engine = new_engine(ws, dir, default_profile(), emit, lambda sp, h: FakeAgentSession(h))
    engine.state = STATE_DRIVE
    return engine, dir, events


async def test_start_proposes_before_scaffolding(tmp_path: Path) -> None:
    eng, dir, _ = new_test_engine(tmp_path, [ScriptedTurn(text="Idea 1 or idea 2?")])
    await eng.start("learn python")
    assert eng.state == STATE_PROPOSE
    assert len(eng.plan.steps) == 0
    for entry in Path(dir).iterdir():
        assert entry.name in (".git", ".nina"), f"file scaffolded during propose: {entry.name}"

    await eng.user_message("something else?")
    assert eng.state == STATE_PROPOSE


async def test_start_scaffolds_and_plans(tmp_path: Path) -> None:
    eng, dir, events = await started_engine(tmp_path)

    assert eng.state == STATE_DRIVE
    assert eng.plan.title == "Guessing Game"
    assert len(eng.plan.steps) == 2
    assert (Path(dir) / "main.py").exists()
    kinds = {ev.kind for ev in events}
    for want in (EVENT_PLAN_SET, EVENT_INFO, EVENT_STEP_STARTED):
        assert want in kinds


async def test_dial_rejects_writes_after_scaffold(tmp_path: Path) -> None:
    eng, dir, _ = await started_engine(tmp_path)
    result = await eng._write_file({"path": "solution.py", "content": "answer"})
    assert result.is_error
    assert not (Path(dir) / "solution.py").exists()


@pytest.mark.parametrize(
    "dial,state_,allowed",
    [
        (0, STATE_SCAFFOLD, False),
        (0, STATE_DRIVE, False),
        (1, STATE_SCAFFOLD, True),
        (1, STATE_DRIVE, False),
        (2, STATE_SCAFFOLD, True),
        (2, STATE_DRIVE, True),
        (3, STATE_DRIVE, True),
    ],
)
async def test_dial_policy_matrix(tmp_path: Path, dial: int, state_: str, allowed: bool) -> None:
    eng, dir, _ = new_test_engine(tmp_path)
    eng.profile.dial = dial
    eng.state = state_
    result = await eng._write_file({"path": "f.py", "content": "x"})
    assert (not result.is_error) == allowed
    assert (Path(dir) / "f.py").exists() == allowed


async def test_update_profile_rebuilds_system_prompt(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg"))
    eng, _, _ = await started_engine(tmp_path)
    prof = eng.profile
    prof.dial = 0
    eng.update_profile(prof)
    assert "level 0" in eng.system_prompt
    result = await eng._write_file({"path": "f.py", "content": "x"})
    assert result.is_error


async def test_update_plan_replaces_remaining_steps(tmp_path: Path) -> None:
    eng, _, events = await started_engine(tmp_path)

    result = await eng._update_plan(
        {
            "steps": [
                {"title": "New step 2", "goal": "different goal"},
                {"title": "New step 3", "goal": "extra"},
            ]
        }
    )
    assert not result.is_error
    steps = eng.plan.steps
    assert len(steps) == 3
    assert steps[0].title == "Read input"
    assert steps[1].title == "New step 2"
    assert events[-1].kind == EVENT_PLAN_SET


async def test_skip_advances_without_review(tmp_path: Path) -> None:
    eng, dir, events = await started_engine(tmp_path, [ScriptedTurn(text="Step 2 instructions.")])
    (Path(dir) / "main.py").write_text("half-finished\n")

    await eng.skip()
    assert eng.step_index == 1
    assert eng.state == STATE_DRIVE
    assert not any(ev.kind == EVENT_REVIEW for ev in events)

    await eng.done()
    assert eng.step_index == 1


async def test_write_file_rejects_escaping_paths(tmp_path: Path) -> None:
    eng, _, _ = new_test_engine(tmp_path)
    eng.state = STATE_SCAFFOLD
    for path in ("../evil.txt", "/etc/passwd", ""):
        result = await eng._write_file({"path": path, "content": "x"})
        assert result.is_error


async def test_done_pass_advances_step(tmp_path: Path) -> None:
    eng, dir, events = await started_engine(
        tmp_path,
        [
            ScriptedTurn(
                tool_calls=[
                    ScriptedToolCall(
                        TOOL_SUBMIT_REVIEW, {"verdict": "pass", "feedback": "Nice use of input()."}
                    )
                ]
            ),
            ScriptedTurn(text="Step 2: compare the numbers."),
        ],
    )
    (Path(dir) / "main.py").write_text("n = int(input())\n")

    await eng.done()
    assert eng.step_index == 1
    reviews = [ev for ev in events if ev.kind == EVENT_REVIEW]
    assert reviews[-1].verdict == "pass"


async def test_done_stops_after_submit_review(tmp_path: Path) -> None:
    eng, dir, _ = await started_engine(
        tmp_path,
        [
            ScriptedTurn(
                tool_calls=[
                    ScriptedToolCall(
                        TOOL_SUBMIT_REVIEW, {"verdict": "pass", "feedback": "Nice use of input()."}
                    )
                ]
            ),
            ScriptedTurn(text="Step 2: compare the numbers."),
        ],
    )
    (Path(dir) / "main.py").write_text("n = int(input())\n")

    assert isinstance(eng.session, FakeAgentSession)
    before = eng.session.calls
    await eng.done()
    assert eng.session.calls - before == 2


async def test_done_retry_keeps_step(tmp_path: Path) -> None:
    eng, dir, events = await started_engine(
        tmp_path,
        [
            ScriptedTurn(
                tool_calls=[
                    ScriptedToolCall(
                        TOOL_SUBMIT_REVIEW,
                        {
                            "verdict": "retry",
                            "feedback": "What happens if the input is not a number?",
                        },
                    )
                ]
            )
        ],
    )
    (Path(dir) / "main.py").write_text("n = input()\n")

    await eng.done()
    assert eng.step_index == 0
    reviews = [ev for ev in events if ev.kind == EVENT_REVIEW]
    assert reviews[-1].verdict == "retry"


async def test_done_with_no_changes(tmp_path: Path) -> None:
    eng, _, events = await started_engine(tmp_path)

    await eng.done()
    assert eng.step_index == 0
    assert events[-1].kind == EVENT_INFO


async def test_session_completes_after_last_step(tmp_path: Path) -> None:
    pass_turns = [
        ScriptedTurn(
            tool_calls=[
                ScriptedToolCall(TOOL_SUBMIT_REVIEW, {"verdict": "pass", "feedback": "Good."})
            ]
        ),
        ScriptedTurn(text="Step 2 instructions."),
        ScriptedTurn(
            tool_calls=[
                ScriptedToolCall(TOOL_SUBMIT_REVIEW, {"verdict": "pass", "feedback": "Done!"})
            ]
        ),
    ]
    eng, dir, events = await started_engine(tmp_path, pass_turns)

    (Path(dir) / "main.py").write_text("step one\n")
    await eng.done()
    (Path(dir) / "main.py").write_text("step two\n")
    await eng.done()

    assert eng.state == STATE_DONE
    assert any(ev.kind == EVENT_SESSION_DONE for ev in events)


async def test_persist_and_restore_continues_session(tmp_path: Path) -> None:
    eng, dir, _ = await started_engine(tmp_path)
    sess = state.load(dir)
    assert sess is not None
    assert sess.state == STATE_DRIVE
    assert sess.goal == "learn python"

    ws = open_workspace(dir)
    events: list[Event] = []
    restored = new_engine(
        ws,
        dir,
        default_profile(),
        events.append,
        lambda sp, h: FakeAgentSession(
            h,
            [
                ScriptedTurn(
                    tool_calls=[
                        ScriptedToolCall(
                            TOOL_SUBMIT_REVIEW, {"verdict": "pass", "feedback": "Nice."}
                        )
                    ]
                ),
                ScriptedTurn(text="Step 2 instructions."),
            ],
        ),
    )
    restored.restore(sess)

    assert restored.state == STATE_DRIVE
    assert restored.plan.title == eng.plan.title
    (Path(dir) / "main.py").write_text("n = int(input())\n")
    await restored.done()
    assert restored.step_index == 1


async def test_snapshots_exclude_nina_dir(tmp_path: Path) -> None:
    eng, dir, _ = await started_engine(
        tmp_path,
        [
            ScriptedTurn(
                tool_calls=[
                    ScriptedToolCall(
                        TOOL_SUBMIT_REVIEW, {"verdict": "retry", "feedback": "keep going"}
                    )
                ]
            )
        ],
    )
    await eng.done()
    assert eng.review is None
    assert (Path(dir) / ".nina" / "session.json").exists()


async def test_summarize_writes_file(tmp_path: Path) -> None:
    eng, dir, events = await started_engine(
        tmp_path,
        [ScriptedTurn(text="You built a guessing game and learned about input().")],
    )

    await eng.summarize()
    entries = list(Path(dir, ".nina").glob("summary-*.md"))
    assert len(entries) == 1
    assert "guessing game" in entries[0].read_text()
    assert events[-1].kind == EVENT_INFO
    assert "Summary saved" in events[-1].text


async def test_run_command_approved(tmp_path: Path) -> None:
    eng, _, events = confirming_engine(tmp_path, ConfirmAnswer(approve=True))

    result = await eng._run_command({"command": "echo hello", "reason": "verify"})
    assert not result.is_error
    assert "exit code: 0" in result.content
    assert "hello" in result.content
    kinds = [ev.kind for ev in events]
    assert kinds.count(EVENT_CONFIRM) == 1
    assert kinds.count(EVENT_COMMAND_RUN) == 1


async def test_run_command_declined(tmp_path: Path) -> None:
    eng, dir, events = confirming_engine(tmp_path, ConfirmAnswer(approve=False))

    result = await eng._run_command({"command": "touch declined.txt", "reason": "verify"})
    assert not result.is_error
    assert "declined" in result.content
    assert not (Path(dir) / "declined.txt").exists()
    assert not any(ev.kind == EVENT_COMMAND_RUN for ev in events)


async def test_run_command_always_skips_second_confirm(tmp_path: Path) -> None:
    eng, _, events = confirming_engine(tmp_path, ConfirmAnswer(approve=True, always=True))

    for _ in range(2):
        result = await eng._run_command({"command": "true", "reason": "verify"})
        assert not result.is_error
    confirms = sum(1 for ev in events if ev.kind == EVENT_CONFIRM)
    assert confirms == 1


async def test_read_file(tmp_path: Path) -> None:
    eng, dir, _ = confirming_engine(tmp_path, ConfirmAnswer(approve=True))
    (Path(dir) / "main.py").write_text("print('hi')\n")

    result = await eng._read_file({"path": "main.py"})
    assert not result.is_error
    assert result.content == "print('hi')\n"

    result = await eng._read_file({"path": "../secret"})
    assert result.is_error


async def test_start_raises_and_persists_on_fatal_rate_limit(tmp_path: Path) -> None:
    eng, dir, events = new_test_engine(tmp_path, [ScriptedTurn(rate_limited=1_800_000_000.0)])

    with pytest.raises(RateLimitExceeded):
        await eng.start("learn python")

    assert eng.state == STATE_PROPOSE
    sess = state.load(dir)
    assert sess is not None
    assert sess.state == STATE_PROPOSE
    assert any(ev.kind == EVENT_INFO and "Rate limit reached" in (ev.text or "") for ev in events)


async def test_fatal_rate_limit_without_resets_at(tmp_path: Path) -> None:
    eng, _, _ = new_test_engine(tmp_path, [ScriptedTurn(rate_limited=True)])

    with pytest.raises(RateLimitExceeded, match="later"):
        await eng.start("learn python")


async def test_screening_buffers_deltas_and_blocks_tools_during_rewrite(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    leak_calls = iter([True, False])

    async def fake_leaks(step_goal: str, text: str) -> bool:
        return next(leak_calls)

    monkeypatch.setattr(screening_module, "leaks", fake_leaks)

    turns = [
        ScriptedTurn(text="```py\nanswer = 42\n```"),
        ScriptedTurn(
            text="use a variable and print it",
            tool_calls=[ScriptedToolCall(TOOL_WRITE_FILE, {"path": "leak.py", "content": "x"})],
        ),
        ScriptedTurn(),
    ]
    eng, dir, events = new_test_engine(tmp_path, turns)
    eng.profile.dial = 1
    eng.state = STATE_DRIVE

    await eng.user_message("show me the code")

    deltas = [ev.text for ev in events if ev.kind == EVENT_TEXT_DELTA]
    assert deltas == ["use a variable and print it"]
    assert not (Path(dir) / "leak.py").exists()


async def test_screening_inactive_streams_deltas_live(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    async def fail_if_called(step_goal: str, text: str) -> bool:
        raise AssertionError("leaks() should not run when screening is inactive")

    monkeypatch.setattr(screening_module, "leaks", fail_if_called)

    eng, _, events = new_test_engine(tmp_path, [ScriptedTurn(text="```py\nanswer = 42\n```")])
    eng.profile.dial = 2
    eng.state = STATE_DRIVE

    await eng.user_message("show me the code")

    deltas = [ev.text for ev in events if ev.kind == EVENT_TEXT_DELTA]
    assert deltas == ["```py\nanswer = 42\n```"]
