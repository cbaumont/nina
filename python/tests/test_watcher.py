from __future__ import annotations

import queue
import time
from pathlib import Path

from nina.system.watcher import Watcher

IDLE = 0.1


def start_watcher(tmp_path: Path) -> tuple[Watcher, queue.Queue[object]]:
    nudges: queue.Queue[object] = queue.Queue()
    watcher = Watcher(str(tmp_path), IDLE, lambda: nudges.put(object()))
    return watcher, nudges


def expect_nudge(nudges: queue.Queue[object]) -> None:
    nudges.get(timeout=3)


def expect_quiet(nudges: queue.Queue[object]) -> None:
    try:
        nudges.get(timeout=0.4)
    except queue.Empty:
        return
    raise AssertionError("nudge fired for ignored activity")


def test_nudge_after_edit_then_idle(tmp_path: Path) -> None:
    watcher, nudges = start_watcher(tmp_path)
    try:
        (tmp_path / "main.py").write_text("x")
        expect_nudge(nudges)
    finally:
        watcher.close()


def test_ignored_paths_stay_quiet(tmp_path: Path) -> None:
    for sub in (".git", ".nina"):
        (tmp_path / sub).mkdir()
    watcher, nudges = start_watcher(tmp_path)
    try:
        (tmp_path / ".git" / "index").write_text("x")
        (tmp_path / ".nina" / "session.json").write_text("{}")
        (tmp_path / "main.py.swp").write_text("x")
        (tmp_path / "backup~").write_text("x")
        expect_quiet(nudges)
    finally:
        watcher.close()


def test_watches_new_subdirectories(tmp_path: Path) -> None:
    watcher, nudges = start_watcher(tmp_path)
    try:
        sub = tmp_path / "src"
        sub.mkdir()
        time.sleep(0.2)
        (sub / "app.py").write_text("x")
        expect_nudge(nudges)
    finally:
        watcher.close()
