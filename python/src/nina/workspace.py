from __future__ import annotations

import os
import subprocess
import tempfile
from pathlib import Path


class WorkspaceError(RuntimeError):
    pass


class Workspace:
    def __init__(self, dir: str) -> None:
        self.dir = dir

    def _git(self, args: list[str], extra_env: dict[str, str] | None = None) -> str:
        env = os.environ.copy()
        env.update(
            {
                "GIT_AUTHOR_NAME": "nina",
                "GIT_AUTHOR_EMAIL": "nina@localhost",
                "GIT_COMMITTER_NAME": "nina",
                "GIT_COMMITTER_EMAIL": "nina@localhost",
            }
        )
        if extra_env:
            env.update(extra_env)
        proc = subprocess.run(
            ["git", *args],
            cwd=self.dir,
            env=env,
            capture_output=True,
            text=True,
        )
        if proc.returncode != 0:
            raise WorkspaceError(f"git {' '.join(args)}: {proc.stderr.strip()}")
        return proc.stdout.strip()

    def _exclude_from_snapshots(self, pattern: str) -> None:
        git_dir = Path(self._git(["rev-parse", "--git-dir"]))
        if not git_dir.is_absolute():
            git_dir = Path(self.dir) / git_dir
        path = git_dir / "info" / "exclude"
        existing = path.read_text() if path.exists() else ""
        if any(line.strip() == pattern for line in existing.split("\n")):
            return
        path.parent.mkdir(parents=True, exist_ok=True)
        content = existing
        if content and not content.endswith("\n"):
            content += "\n"
        path.write_text(content + pattern + "\n")

    def snapshot(self, ref: str) -> str:
        with tempfile.TemporaryDirectory(prefix="nina-index-") as index_dir:
            index_env = {"GIT_INDEX_FILE": str(Path(index_dir) / "index")}
            self._git(["add", "-A", "."], extra_env=index_env)
            tree = self._git(["write-tree"], extra_env=index_env)
        commit = self._git(["commit-tree", tree, "-m", "nina snapshot"])
        self._git(["update-ref", ref, commit])
        return commit

    def diff(self, ref_a: str, ref_b: str) -> str:
        return self._git(["diff", ref_a, ref_b])


def open_workspace(dir: str) -> Workspace:
    w = Workspace(dir)
    try:
        w._git(["rev-parse", "--git-dir"])
    except WorkspaceError:
        try:
            w._git(["init"])
        except WorkspaceError as err:
            raise WorkspaceError(f"initializing git repository: {err}") from err
    w._exclude_from_snapshots("/.nina/")
    return w


def snapshot_ref(session: str, step: int) -> str:
    return f"refs/nina/{session}/{step}"
