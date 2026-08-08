from __future__ import annotations

import asyncio
import os
import sys

from nina.agent.factory import session_factory
from nina.engine import Engine, RateLimitExceeded, new_engine
from nina.engine.events import (
    EVENT_COMMAND_RUN,
    EVENT_CONFIRM,
    EVENT_INFO,
    EVENT_PLAN_SET,
    EVENT_REVIEW,
    EVENT_SESSION_DONE,
    EVENT_STEP_STARTED,
    EVENT_TEXT_DELTA,
    ConfirmAnswer,
    Event,
)
from nina.system.profile import default as default_profile
from nina.system.workspace import open_workspace


def _print_event(event: Event) -> None:
    if event.kind == EVENT_TEXT_DELTA:
        print(event.text, end="", flush=True)
    elif event.kind == EVENT_CONFIRM:
        assert event.confirm is not None
        print(f"\n[auto-approving] $ {event.confirm.command}  ({event.confirm.reason})")
        event.confirm.reply.set_result(ConfirmAnswer(approve=True))
    elif event.kind == EVENT_COMMAND_RUN:
        print(f"\n{event.text}\n")
    elif event.kind == EVENT_PLAN_SET:
        assert event.plan is not None
        print(f"\n[plan] {event.plan.title}: {len(event.plan.steps)} steps\n")
    elif event.kind == EVENT_STEP_STARTED:
        print(f"\n[step {event.step + 1} started]\n")
    elif event.kind == EVENT_REVIEW:
        print(f"\n[review: {event.verdict}] {event.text}\n")
    elif event.kind == EVENT_SESSION_DONE:
        print("\n[session done]\n")
    elif event.kind == EVENT_INFO:
        print(f"\n[info] {event.text}\n")


async def run(goal: str, dir: str, model: str | None = None) -> Engine:
    ws = open_workspace(dir)
    engine = new_engine(
        ws,
        dir,
        default_profile(),
        _print_event,
        session_factory(dir, model),
        model=model,
    )
    await engine.start(goal)
    return engine


def main() -> None:
    goal = " ".join(sys.argv[1:]) or "learn Python basics"
    dir = "."
    model = os.environ.get("NINA_MODEL")
    try:
        asyncio.run(run(goal, dir, model))
    except RateLimitExceeded as err:
        print(f"\n[rate limited] {err}")
        sys.exit(1)


if __name__ == "__main__":
    main()
