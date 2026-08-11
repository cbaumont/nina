from __future__ import annotations

import threading
from collections.abc import Callable
from pathlib import Path

from watchdog.events import FileSystemEvent, FileSystemEventHandler
from watchdog.observers import Observer

IGNORED_DIRS = {
    ".git",
    ".nina",
    "node_modules",
    "__pycache__",
    ".venv",
    "venv",
    ".idea",
    ".vscode",
}


class Watcher:
    def __init__(self, dir: str, idle_seconds: float, nudge: Callable[[], None]) -> None:
        self._root = Path(dir).resolve()
        self._idle_seconds = idle_seconds
        self._nudge = nudge
        self._lock = threading.Lock()
        self._timer: threading.Timer | None = None
        self._paused = False
        self._observer = Observer()
        self._observer.schedule(
            _Handler(self._root, self._on_activity), str(self._root), recursive=True
        )
        self._observer.start()

    def close(self) -> None:
        self._observer.stop()
        self._observer.join(timeout=2)
        with self._lock:
            if self._timer is not None:
                self._timer.cancel()
                self._timer = None

    def pause(self) -> None:
        """Stop arming the idle timer, and cancel any timer already pending.

        Nina's own writes (scaffolding files, running commands) generate
        filesystem events indistinguishable from the user's. Pausing while
        Nina is working keeps those writes from later firing a false nudge.
        """
        with self._lock:
            self._paused = True
            if self._timer is not None:
                self._timer.cancel()
                self._timer = None

    def resume(self) -> None:
        with self._lock:
            self._paused = False

    def _on_activity(self) -> None:
        with self._lock:
            if self._paused:
                return
            if self._timer is not None:
                self._timer.cancel()
            self._timer = threading.Timer(self._idle_seconds, self._fire)
            self._timer.daemon = True
            self._timer.start()

    def _fire(self) -> None:
        with self._lock:
            self._timer = None
        self._nudge()


class _Handler(FileSystemEventHandler):
    def __init__(self, root: Path, on_activity: Callable[[], None]) -> None:
        self._root = root
        self._on_activity = on_activity

    def on_any_event(self, event: FileSystemEvent) -> None:
        if event.is_directory:
            return
        path = Path(str(event.src_path))
        if self._ignored(path):
            return
        self._on_activity()

    def _ignored(self, path: Path) -> bool:
        try:
            rel = path.resolve().relative_to(self._root)
        except ValueError:
            return True
        if any(part in IGNORED_DIRS for part in rel.parts):
            return True
        name = path.name
        return (
            name.endswith(("~", ".swp", ".swo", ".tmp"))
            or name.startswith(".#")
            or name.startswith("#")
        )
