from __future__ import annotations

from dataclasses import dataclass

TOOL_WRITE_FILE = "write_file"
TOOL_SET_PLAN = "set_plan"
TOOL_SUBMIT_REVIEW = "submit_review"
TOOL_RUN_COMMAND = "run_command"
TOOL_READ_FILE = "read_file"
TOOL_UPDATE_PLAN = "update_plan"


@dataclass
class ToolResult:
    content: str
    is_error: bool = False


@dataclass
class PlanStep:
    title: str
    goal: str


@dataclass
class SubmitReviewInput:
    verdict: str
    feedback: str


def plan_steps_from(raw: object) -> list[PlanStep]:
    if not isinstance(raw, list):
        raise ValueError("steps must be a list")
    steps = []
    for item in raw:
        if not isinstance(item, dict) or "title" not in item or "goal" not in item:
            raise ValueError("each step needs a title and a goal")
        steps.append(PlanStep(title=str(item["title"]), goal=str(item["goal"])))
    return steps


_STEP_ITEM_SCHEMA = {
    "type": "object",
    "properties": {
        "title": {"type": "string", "description": "Short step title"},
        "goal": {
            "type": "string",
            "description": "What the learner's code must achieve for this step to pass review",
        },
    },
    "required": ["title", "goal"],
}


@dataclass
class ToolSpec:
    name: str
    description: str
    schema: dict[str, object]


TOOL_SPECS: list[ToolSpec] = [
    ToolSpec(
        name=TOOL_WRITE_FILE,
        description=(
            "Write a file in the learning project workspace. Only permitted when the "
            "typing dial allows it; otherwise the call is rejected."
        ),
        schema={
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Relative path within the workspace"},
                "content": {"type": "string", "description": "Full file content"},
            },
            "required": ["path", "content"],
        },
    ),
    ToolSpec(
        name=TOOL_SET_PLAN,
        description=(
            "Set the task plan for the session: a short project title and 3-5 small, "
            "ordered steps the learner will implement."
        ),
        schema={
            "type": "object",
            "properties": {
                "title": {"type": "string", "description": "Short project title"},
                "steps": {"type": "array", "items": _STEP_ITEM_SCHEMA},
            },
            "required": ["title", "steps"],
        },
    ),
    ToolSpec(
        name=TOOL_UPDATE_PLAN,
        description=(
            "Revise the not-yet-started steps of the task plan. Completed steps and the "
            "step in progress are kept; the given steps replace everything after the "
            "current one. Always tell the learner what changed and why."
        ),
        schema={
            "type": "object",
            "properties": {
                "steps": {
                    "type": "array",
                    "description": "Replacement steps for the remainder of the session",
                    "items": _STEP_ITEM_SCHEMA,
                },
            },
            "required": ["steps"],
        },
    ),
    ToolSpec(
        name=TOOL_RUN_COMMAND,
        description=(
            "Propose a shell command to run in the workspace (run code, tests, installs, "
            "version checks). The learner confirms before it executes. Returns the exit "
            "code and combined stdout/stderr. Use it to verify the learner's code by "
            "running it or its tests, and for environment setup while scaffolding."
        ),
        schema={
            "type": "object",
            "properties": {
                "command": {
                    "type": "string",
                    "description": "Shell command to run in the workspace root",
                },
                "reason": {
                    "type": "string",
                    "description": "One short sentence shown to the learner explaining why",
                },
            },
            "required": ["command", "reason"],
        },
    ),
    ToolSpec(
        name=TOOL_READ_FILE,
        description=(
            "Read a file from the workspace to see its current content, for example when "
            "a diff alone lacks context during review."
        ),
        schema={
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Relative path within the workspace"},
            },
            "required": ["path"],
        },
    ),
    ToolSpec(
        name=TOOL_SUBMIT_REVIEW,
        description=(
            "Submit the verdict after reviewing the learner's diff against the current "
            "step's goal. Use 'pass' when the goal is met by any valid approach, 'retry' "
            "when it is not."
        ),
        schema={
            "type": "object",
            "properties": {
                "verdict": {"type": "string", "enum": ["pass", "retry"]},
                "feedback": {"type": "string", "description": "Teaching feedback for the learner"},
            },
            "required": ["verdict", "feedback"],
        },
    ),
]
