from __future__ import annotations

from pathlib import Path

import pytest

from nina.system import profile


def test_parse_validation() -> None:
    profile.parse_level("beginner")
    with pytest.raises(ValueError):
        profile.parse_level("expert")
    profile.parse_hint_speed("slow")
    with pytest.raises(ValueError):
        profile.parse_hint_speed("warp")
    assert profile.parse_dial("0") == 0
    assert profile.parse_dial("3") == 3
    for bad in ("4", "-1", "x", ""):
        with pytest.raises(ValueError):
            profile.parse_dial(bad)


def test_load_without_profile(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg"))
    prof, found = profile.load(str(tmp_path))
    assert not found
    assert prof == profile.default()


def test_save_load_round_trip(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg"))
    project_dir = tmp_path / "project"
    project_dir.mkdir()
    prof = profile.Profile(
        experience=profile.LEVEL_PROFESSIONAL,
        stack_familiarity=profile.LEVEL_NONE,
        known_stacks=["go", "sql"],
        dial=2,
        hint_escalation=profile.HINT_FAST,
    )
    profile.save(str(project_dir), prof)

    got, found = profile.load(str(project_dir))
    assert found
    assert got == prof

    other_dir = tmp_path / "other"
    other_dir.mkdir()
    got, found = profile.load(str(other_dir))
    assert found
    assert got == prof
