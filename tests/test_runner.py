from __future__ import annotations

import time
from pathlib import Path

from nina.system.runner import MAX_OUTPUT, run


async def test_run_captures_output_and_exit_code(tmp_path: Path) -> None:
    res = await run(str(tmp_path), "echo out; echo err >&2")
    assert res.exit_code == 0
    assert not res.timed_out
    assert "out" in res.output
    assert "err" in res.output


async def test_run_non_zero_exit(tmp_path: Path) -> None:
    res = await run(str(tmp_path), "exit 3")
    assert res.exit_code == 3


async def test_run_timeout_kills_command(tmp_path: Path) -> None:
    start = time.monotonic()
    res = await run(str(tmp_path), "sleep 10", timeout=0.2)
    assert res.timed_out
    assert time.monotonic() - start < 5


async def test_run_runs_in_dir(tmp_path: Path) -> None:
    res = await run(str(tmp_path), "pwd")
    assert str(tmp_path) in res.output


async def test_run_caps_output_to_tail(tmp_path: Path) -> None:
    res = await run(str(tmp_path), "seq 1 100000")
    assert len(res.output) <= MAX_OUTPUT + 64
    assert res.output.startswith("[output truncated")
    assert "100000" in res.output
