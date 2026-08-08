from __future__ import annotations

import contextlib
import json
from dataclasses import asdict, dataclass, field
from pathlib import Path

from nina.agent.tools import PlanStep

DIR_NAME = ".nina"


@dataclass
class Session:
    session_id: str = ""
    goal: str = ""
    state: str = ""
    plan_title: str = ""
    steps: list[PlanStep] = field(default_factory=list)
    step_index: int = 0
    snapshots: int = 0
    last_ref: str = ""
    sdk_session_id: str | None = None
    model: str | None = None


def save(workspace_dir: str, sess: Session) -> None:
    dir_path = Path(workspace_dir) / DIR_NAME
    dir_path.mkdir(parents=True, exist_ok=True)
    raw = json.dumps(asdict(sess), indent=2)
    _write_atomic(dir_path / "session.json", raw)


def load(workspace_dir: str) -> Session | None:
    path = Path(workspace_dir) / DIR_NAME / "session.json"
    if not path.exists():
        return None
    data = json.loads(path.read_text())
    steps = [PlanStep(title=s["title"], goal=s["goal"]) for s in data.get("steps", [])]
    return Session(
        session_id=data.get("session_id", ""),
        goal=data.get("goal", ""),
        state=data.get("state", ""),
        plan_title=data.get("plan_title", ""),
        steps=steps,
        step_index=data.get("step_index", 0),
        snapshots=data.get("snapshots", 0),
        last_ref=data.get("last_ref", ""),
        sdk_session_id=data.get("sdk_session_id"),
        model=data.get("model"),
    )


def save_history(workspace_dir: str, text: str) -> None:
    dir_path = Path(workspace_dir) / DIR_NAME
    dir_path.mkdir(parents=True, exist_ok=True)
    _write_atomic(dir_path / "history.md", text)


def load_history(workspace_dir: str) -> str:
    path = Path(workspace_dir) / DIR_NAME / "history.md"
    if not path.exists():
        return ""
    return path.read_text()


def append_transcript(workspace_dir: str, entry: dict[str, object]) -> None:
    dir_path = Path(workspace_dir) / DIR_NAME
    dir_path.mkdir(parents=True, exist_ok=True)
    path = dir_path / "transcript.jsonl"
    with path.open("a") as f:
        f.write(json.dumps(entry) + "\n")


def load_transcript(workspace_dir: str) -> list[dict[str, object]]:
    path = Path(workspace_dir) / DIR_NAME / "transcript.jsonl"
    if not path.exists():
        return []
    entries: list[dict[str, object]] = []
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line:
            continue
        with contextlib.suppress(json.JSONDecodeError):
            entries.append(json.loads(line))
    return entries


def _write_atomic(path: Path, data: str) -> None:
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(data)
    tmp.rename(path)
