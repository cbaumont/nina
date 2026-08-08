from __future__ import annotations

import asyncio
from dataclasses import dataclass, field

from nina.agent.tools import PlanStep

STATE_IDLE = "idle"
STATE_PROPOSE = "propose"
STATE_SCAFFOLD = "scaffold"
STATE_DRIVE = "drive"
STATE_DONE = "done"

EVENT_TEXT_DELTA = "text_delta"
EVENT_INFO = "info"
EVENT_PLAN_SET = "plan_set"
EVENT_STEP_STARTED = "step_started"
EVENT_REVIEW = "review"
EVENT_SESSION_DONE = "session_done"
EVENT_CONFIRM = "confirm"
EVENT_COMMAND_RUN = "command_run"
EVENT_NUDGE = "nudge"


@dataclass
class Plan:
    title: str = ""
    steps: list[PlanStep] = field(default_factory=list)


@dataclass
class ConfirmAnswer:
    approve: bool
    always: bool = False


@dataclass
class ConfirmRequest:
    command: str
    reason: str
    reply: asyncio.Future[ConfirmAnswer]


@dataclass
class Event:
    kind: str
    text: str = ""
    step: int = 0
    plan: Plan | None = None
    verdict: str = ""
    confirm: ConfirmRequest | None = None
