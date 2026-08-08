from __future__ import annotations

import subprocess
from pathlib import Path

from nina.system.workspace import open_workspace, snapshot_ref


def git_output(dir: Path, *args: str) -> str:
    proc = subprocess.run(["git", *args], cwd=dir, capture_output=True, text=True)
    assert proc.returncode == 0, proc.stderr
    return proc.stdout.strip()


def write_file(dir: Path, name: str, content: str) -> None:
    (dir / name).write_text(content)


def test_open_inits_repo(tmp_path: Path) -> None:
    open_workspace(str(tmp_path))
    assert (tmp_path / ".git").exists()


def test_open_uses_existing_repo(tmp_path: Path) -> None:
    git_output(tmp_path, "init")
    open_workspace(str(tmp_path))


def test_snapshot_and_diff(tmp_path: Path) -> None:
    w = open_workspace(str(tmp_path))

    write_file(tmp_path, "a.txt", "one\n")
    ref1 = snapshot_ref("s1", 0)
    w.snapshot(ref1)

    write_file(tmp_path, "a.txt", "two\n")
    write_file(tmp_path, "b.txt", "new file\n")
    ref2 = snapshot_ref("s1", 1)
    w.snapshot(ref2)

    diff = w.diff(ref1, ref2)
    for want in ("-one", "+two", "b.txt", "+new file"):
        assert want in diff


def test_snapshot_does_not_touch_index_or_history(tmp_path: Path) -> None:
    git_output(tmp_path, "init")
    git_output(
        tmp_path,
        "-c",
        "user.name=test",
        "-c",
        "user.email=test@test",
        "commit",
        "--allow-empty",
        "-m",
        "user commit",
    )

    w = open_workspace(str(tmp_path))
    write_file(tmp_path, "a.txt", "hello\n")
    status_before = git_output(tmp_path, "status", "--porcelain")

    w.snapshot(snapshot_ref("s1", 0))

    assert git_output(tmp_path, "status", "--porcelain") == status_before
    assert "nina" not in git_output(tmp_path, "log", "--oneline", "HEAD")
    assert "refs/nina/s1/0" in git_output(tmp_path, "for-each-ref", "refs/nina")


def test_snapshot_respects_gitignore(tmp_path: Path) -> None:
    w = open_workspace(str(tmp_path))
    write_file(tmp_path, ".gitignore", "ignored.txt\n")
    write_file(tmp_path, "ignored.txt", "secret\n")
    ref1 = snapshot_ref("s1", 0)
    w.snapshot(ref1)

    write_file(tmp_path, "ignored.txt", "changed secret\n")
    ref2 = snapshot_ref("s1", 1)
    w.snapshot(ref2)

    assert w.diff(ref1, ref2) == ""
