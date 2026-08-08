from __future__ import annotations

import asyncio
import contextlib
import os
import signal
from dataclasses import dataclass

DEFAULT_TIMEOUT = 120.0
MAX_OUTPUT = 64 * 1024


@dataclass
class Result:
    exit_code: int
    output: str
    timed_out: bool


class TailBuffer:
    def __init__(self, limit: int) -> None:
        self._limit = limit
        self._buf = bytearray()
        self._truncated = False

    def write(self, data: bytes) -> None:
        self._buf += data
        if len(self._buf) > self._limit:
            self._buf = self._buf[-self._limit :]
            self._truncated = True

    def __str__(self) -> str:
        s = self._buf.decode(errors="replace").strip()
        if self._truncated:
            return "[output truncated, showing the tail]\n" + s
        return s


async def run(dir: str, command: str, timeout: float | None = None) -> Result:
    if not timeout or timeout <= 0:
        timeout = DEFAULT_TIMEOUT

    proc = await asyncio.create_subprocess_shell(
        command,
        cwd=dir,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        preexec_fn=os.setsid,
    )

    out = TailBuffer(MAX_OUTPUT)
    timed_out = False

    async def pump() -> None:
        assert proc.stdout is not None
        while True:
            chunk = await proc.stdout.read(4096)
            if not chunk:
                return
            out.write(chunk)

    pump_task = asyncio.create_task(pump())
    try:
        await asyncio.wait_for(asyncio.shield(proc.wait()), timeout=timeout)
    except TimeoutError:
        timed_out = True
        with contextlib.suppress(ProcessLookupError):
            os.killpg(proc.pid, signal.SIGKILL)
        await proc.wait()
    await pump_task

    exit_code = -1 if timed_out else (proc.returncode or 0)
    return Result(exit_code=exit_code, output=str(out), timed_out=timed_out)
