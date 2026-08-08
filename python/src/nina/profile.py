from __future__ import annotations

import contextlib
import json
import os
from dataclasses import asdict, dataclass, field
from pathlib import Path

LEVEL_NONE = "none"
LEVEL_BEGINNER = "beginner"
LEVEL_INTERMEDIATE = "intermediate"
LEVEL_PROFESSIONAL = "professional"
LEVELS = (LEVEL_NONE, LEVEL_BEGINNER, LEVEL_INTERMEDIATE, LEVEL_PROFESSIONAL)

HINT_SLOW = "slow"
HINT_MEDIUM = "medium"
HINT_FAST = "fast"
HINT_SPEEDS = (HINT_SLOW, HINT_MEDIUM, HINT_FAST)


@dataclass
class Profile:
    experience: str = LEVEL_BEGINNER
    stack_familiarity: str = LEVEL_BEGINNER
    known_stacks: list[str] = field(default_factory=list)
    dial: int = 1
    hint_escalation: str = HINT_MEDIUM


def default() -> Profile:
    return Profile()


def parse_level(s: str) -> str:
    if s in LEVELS:
        return s
    raise ValueError("experience must be none, beginner, intermediate, or professional")


def parse_hint_speed(s: str) -> str:
    if s in HINT_SPEEDS:
        return s
    raise ValueError("hint escalation must be slow, medium, or fast")


def parse_dial(s: str) -> int:
    try:
        dial = int(s)
    except ValueError:
        dial = -1
    if dial < 0 or dial > 3:
        raise ValueError("the dial goes from 0 (full manual) to 3 (collaborative)")
    return dial


def _project_path(workspace_dir: str) -> Path:
    return Path(workspace_dir) / ".nina" / "profile.json"


def _global_path() -> Path | None:
    config = os.environ.get("XDG_CONFIG_HOME")
    base = Path(config) if config else Path.home() / ".config"
    return base / "nina" / "profile.json"


def load(workspace_dir: str) -> tuple[Profile, bool]:
    prof = _read(_project_path(workspace_dir))
    if prof is not None:
        return prof, True
    global_path = _global_path()
    if global_path is not None:
        prof = _read(global_path)
        if prof is not None:
            return prof, True
    return default(), False


def _read(path: Path) -> Profile | None:
    try:
        raw = path.read_text()
    except OSError:
        return None
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return None
    prof = default()
    for key in ("experience", "stack_familiarity", "known_stacks", "dial", "hint_escalation"):
        if key in data:
            setattr(prof, key, data[key])
    return prof


def save(workspace_dir: str, prof: Profile) -> None:
    _write(_project_path(workspace_dir), prof)
    global_path = _global_path()
    if global_path is not None:
        with contextlib.suppress(OSError):
            _write(global_path, prof)


def _write(path: Path, prof: Profile) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(asdict(prof), indent=2))
