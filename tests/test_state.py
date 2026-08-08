from __future__ import annotations

import json
from pathlib import Path

from nina.agent.tools import PlanStep
from nina.system import state


def test_load_without_session(tmp_path: Path) -> None:
    assert state.load(str(tmp_path)) is None


def test_save_load_round_trip(tmp_path: Path) -> None:
    sess = state.Session(
        session_id="20260719-120000",
        goal="learn python",
        state="drive",
        plan_title="Guessing Game",
        steps=[
            PlanStep(title="Read input", goal="reads a number"),
            PlanStep(title="Compare", goal="says higher or lower"),
        ],
        step_index=1,
        snapshots=2,
        last_ref="refs/nina/20260719-120000/1",
    )

    state.save(str(tmp_path), sess)
    got = state.load(str(tmp_path))
    assert got == sess


def test_load_history_without_file(tmp_path: Path) -> None:
    assert state.load_history(str(tmp_path)) == ""


def test_save_load_history_round_trip(tmp_path: Path) -> None:
    text = "## Guessing Game\n\nStep 1: read input\n"
    state.save_history(str(tmp_path), text)
    assert state.load_history(str(tmp_path)) == text

    state.save_history(str(tmp_path), "replaced")
    assert state.load_history(str(tmp_path)) == "replaced"


def test_save_overwrites(tmp_path: Path) -> None:
    state.save(str(tmp_path), state.Session(step_index=0))
    state.save(str(tmp_path), state.Session(step_index=1))
    got = state.load(str(tmp_path))
    assert got is not None
    assert got.step_index == 1
    assert not (tmp_path / ".nina" / "session.json.tmp").exists()


def test_save_load_round_trip_with_sdk_session_id(tmp_path: Path) -> None:
    sess = state.Session(session_id="20260719-120000", sdk_session_id="abc-123")
    state.save(str(tmp_path), sess)
    got = state.load(str(tmp_path))
    assert got is not None
    assert got.sdk_session_id == "abc-123"


def test_append_transcript(tmp_path: Path) -> None:
    state.append_transcript(str(tmp_path), {"role": "user", "text": "hi"})
    state.append_transcript(str(tmp_path), {"role": "assistant", "text": "hello"})
    lines = (tmp_path / ".nina" / "transcript.jsonl").read_text().splitlines()
    assert [json.loads(line) for line in lines] == [
        {"role": "user", "text": "hi"},
        {"role": "assistant", "text": "hello"},
    ]
