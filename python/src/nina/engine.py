from __future__ import annotations

import asyncio
import time
from collections.abc import Callable
from pathlib import Path

from nina import profile as profile_module
from nina import prompts, runner, state
from nina.agent.session import AgentSession, RateLimited, TextDelta, ToolHandler, TurnComplete
from nina.events import (
    EVENT_COMMAND_RUN,
    EVENT_CONFIRM,
    EVENT_INFO,
    EVENT_PLAN_SET,
    EVENT_REVIEW,
    EVENT_SESSION_DONE,
    EVENT_STEP_STARTED,
    EVENT_TEXT_DELTA,
    STATE_DONE,
    STATE_DRIVE,
    STATE_IDLE,
    STATE_PROPOSE,
    STATE_SCAFFOLD,
    ConfirmAnswer,
    ConfirmRequest,
    Event,
    Plan,
)
from nina.profile import Profile
from nina.runner import Result
from nina.tools import (
    TOOL_READ_FILE,
    TOOL_RUN_COMMAND,
    TOOL_SET_PLAN,
    TOOL_SUBMIT_REVIEW,
    TOOL_UPDATE_PLAN,
    TOOL_WRITE_FILE,
    SubmitReviewInput,
    ToolResult,
    plan_steps_from,
)
from nina.workspace import Workspace, snapshot_ref

COMMAND_TIMEOUT = 120.0
MAX_READ_FILE_BYTES = 32 * 1024


class Engine:
    def __init__(
        self, ws: Workspace, dir: str, prof: Profile, emit: Callable[[Event], None]
    ) -> None:
        self.ws = ws
        self.dir = dir
        self.emit = emit
        self.session_id = time.strftime("%Y%m%d-%H%M%S")
        self.goal = ""
        self.state = STATE_IDLE
        self.plan = Plan()
        self.step_index = 0
        self.snapshots = 0
        self.last_ref = ""
        self.review: SubmitReviewInput | None = None
        self.profile = prof
        self.system_prompt = prompts.system_prompt(prof)
        self.session: AgentSession | None = None
        self._auto_approve: set[str] = set()

    def tool_handlers(self) -> dict[str, ToolHandler]:
        return {
            TOOL_WRITE_FILE: self._write_file,
            TOOL_SET_PLAN: self._set_plan,
            TOOL_UPDATE_PLAN: self._update_plan,
            TOOL_SUBMIT_REVIEW: self._submit_review,
            TOOL_RUN_COMMAND: self._run_command,
            TOOL_READ_FILE: self._read_file,
        }

    def update_profile(self, prof: Profile) -> None:
        self.profile = prof
        self.system_prompt = prompts.system_prompt(prof)
        profile_module.save(self.dir, prof)
        if self.state != STATE_IDLE:
            self.persist()

    def restore(self, sess: state.Session) -> None:
        self.session_id = sess.session_id
        self.goal = sess.goal
        self.state = sess.state
        self.plan = Plan(title=sess.plan_title, steps=list(sess.steps))
        self.step_index = sess.step_index
        self.snapshots = sess.snapshots
        self.last_ref = sess.last_ref

    def persist(self) -> None:
        sdk_session_id = self.session.session_id if self.session is not None else None
        sess = state.Session(
            session_id=self.session_id,
            goal=self.goal,
            state=self.state,
            plan_title=self.plan.title,
            steps=list(self.plan.steps),
            step_index=self.step_index,
            snapshots=self.snapshots,
            last_ref=self.last_ref,
            sdk_session_id=sdk_session_id,
        )
        try:
            state.save(self.dir, sess)
        except OSError as err:
            self.emit(Event(kind=EVENT_INFO, text=f"Warning: could not save session state: {err}"))

    async def start(self, goal: str) -> None:
        if self.state != STATE_IDLE:
            raise RuntimeError("session already started")
        self.state = STATE_PROPOSE
        self.goal = goal
        try:
            await self._converse(prompts.propose_prompt(goal))
        except Exception:
            self.state = STATE_IDLE
            raise
        self.persist()

    async def done(self) -> None:
        if self.state != STATE_DRIVE:
            raise RuntimeError("no step in progress")
        previous_ref = self.last_ref
        self._snapshot()
        diff = self.ws.diff(previous_ref, self.last_ref)
        if diff.strip() == "":
            self.emit(
                Event(
                    kind=EVENT_INFO,
                    text=(
                        "No changes since the last snapshot — edit the files in your "
                        "editor, then /done."
                    ),
                )
            )
            return

        self.review = None
        step = self.plan.steps[self.step_index]
        await self._converse(prompts.review_prompt(step, diff))
        if self.review is None:
            self.emit(
                Event(
                    kind=EVENT_INFO,
                    text=(
                        "The navigator did not submit a verdict; treating this step "
                        "as still in progress."
                    ),
                )
            )
            return
        self.emit(
            Event(
                kind=EVENT_REVIEW,
                verdict=self.review.verdict,
                text=self.review.feedback,
                step=self.step_index,
            )
        )
        if self.review.verdict != "pass":
            self.persist()
            return

        self.step_index += 1
        if self.step_index >= len(self.plan.steps):
            self.state = STATE_DONE
            self.persist()
            self.emit(Event(kind=EVENT_SESSION_DONE))
            return
        await self._converse(
            prompts.instruct_prompt(self.step_index, self.plan.steps[self.step_index])
        )
        self.persist()
        self.emit(Event(kind=EVENT_STEP_STARTED, step=self.step_index))

    async def skip(self) -> None:
        if self.state != STATE_DRIVE:
            raise RuntimeError("no step in progress")
        skipped = self.step_index
        self._snapshot()
        self.step_index += 1
        if self.step_index >= len(self.plan.steps):
            self.state = STATE_DONE
            self.persist()
            self.emit(Event(kind=EVENT_SESSION_DONE))
            return
        text = (
            prompts.skip_prompt(skipped, self.plan.steps[skipped])
            + "\n\n"
            + prompts.instruct_prompt(self.step_index, self.plan.steps[self.step_index])
        )
        await self._converse(text)
        self.persist()
        self.emit(Event(kind=EVENT_STEP_STARTED, step=self.step_index))

    async def user_message(self, text: str) -> None:
        if self.state == STATE_IDLE:
            raise RuntimeError("session not started")
        await self._converse(text)
        if self.state == STATE_SCAFFOLD:
            if len(self.plan.steps) == 0:
                self.state = STATE_PROPOSE
            else:
                self._snapshot()
                self.state = STATE_DRIVE
                self.emit(Event(kind=EVENT_STEP_STARTED, step=self.step_index))
        self.persist()

    async def summarize(self) -> None:
        if self.state == STATE_IDLE:
            raise RuntimeError("session not started")
        text = await self._converse(prompts.summary_prompt())
        self.persist()
        if text.strip() == "":
            return
        name = f"summary-{self.session_id}.md"
        path = Path(self.dir) / ".nina" / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text + "\n")
        self.emit(Event(kind=EVENT_INFO, text=f"Summary saved to `.nina/{name}`"))

    async def _converse(self, text: str) -> str:
        assert self.session is not None
        self.review = None
        parts: list[str] = []
        async for event in self.session.send(text):
            if isinstance(event, TextDelta):
                parts.append(event.text)
                if self.review is None:
                    self.emit(Event(kind=EVENT_TEXT_DELTA, text=event.text))
            elif isinstance(event, RateLimited):
                self.emit(Event(kind=EVENT_INFO, text=f"Rate limited ({event.rate_limit_type})."))
            elif isinstance(event, TurnComplete):
                break
        return "".join(parts)

    async def _write_file(self, args: dict[str, object]) -> ToolResult:
        if self.profile.dial == 0:
            return ToolResult(
                "Denied by the typing dial policy: at dial level 0 you may not write files "
                "at all. Instruct the learner to write this themselves.",
                is_error=True,
            )
        if self.profile.dial == 1 and self.state != STATE_SCAFFOLD:
            return ToolResult(
                "Denied by the typing dial policy: at dial level 1 you may only write "
                "files while scaffolding the project. Instruct the learner to write this "
                "code themselves.",
                is_error=True,
            )
        path_arg = str(args.get("path", ""))
        content = str(args.get("content", ""))
        try:
            path = self._safe_path(path_arg)
        except ValueError as err:
            return ToolResult(str(err), is_error=True)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        self.emit(Event(kind=EVENT_INFO, text=f"Nina wrote `{path_arg}`"))
        return ToolResult(f"wrote {path_arg}")

    async def _set_plan(self, args: dict[str, object]) -> ToolResult:
        if self.state not in (STATE_PROPOSE, STATE_SCAFFOLD):
            return ToolResult(
                "the session already has a plan; revise the remaining steps with "
                "update_plan instead",
                is_error=True,
            )
        try:
            steps = plan_steps_from(args.get("steps"))
        except ValueError as err:
            return ToolResult(f"invalid set_plan input: {err}", is_error=True)
        if not steps:
            return ToolResult("a plan needs at least one step", is_error=True)
        self.plan = Plan(title=str(args.get("title", "")), steps=steps)
        if self.state == STATE_PROPOSE:
            self.state = STATE_SCAFFOLD
        self.emit(Event(kind=EVENT_PLAN_SET, plan=self.plan))
        return ToolResult(f"plan set with {len(steps)} steps")

    async def _update_plan(self, args: dict[str, object]) -> ToolResult:
        if self.state != STATE_DRIVE:
            return ToolResult(
                "update_plan is only available once the session is underway; use "
                "set_plan for the initial plan",
                is_error=True,
            )
        try:
            steps = plan_steps_from(args.get("steps"))
        except ValueError as err:
            return ToolResult(f"invalid update_plan input: {err}", is_error=True)
        if not steps:
            return ToolResult("update_plan needs at least one replacement step", is_error=True)
        self.plan.steps = self.plan.steps[: self.step_index + 1] + steps
        self.persist()
        self.emit(Event(kind=EVENT_PLAN_SET, plan=self.plan))
        return ToolResult(f"plan revised: {len(steps)} steps remain after the current one")

    async def _submit_review(self, args: dict[str, object]) -> ToolResult:
        verdict = str(args.get("verdict", ""))
        feedback = str(args.get("feedback", ""))
        if verdict not in ("pass", "retry"):
            return ToolResult("verdict must be pass or retry", is_error=True)
        self.review = SubmitReviewInput(verdict=verdict, feedback=feedback)
        return ToolResult("verdict recorded")

    async def _run_command(self, args: dict[str, object]) -> ToolResult:
        command = str(args.get("command", "")).strip()
        reason = str(args.get("reason", ""))
        if not command:
            return ToolResult("run_command needs a non-empty command", is_error=True)
        if command not in self._auto_approve:
            future: asyncio.Future[ConfirmAnswer] = asyncio.get_running_loop().create_future()
            self.emit(
                Event(
                    kind=EVENT_CONFIRM,
                    confirm=ConfirmRequest(command=command, reason=reason, reply=future),
                )
            )
            answer = await future
            if answer.approve and answer.always:
                self._auto_approve.add(command)
            if not answer.approve:
                return ToolResult(
                    "The learner declined to run this command. Continue without it, or "
                    "propose an alternative and explain why it is needed."
                )
        result = await runner.run(self.dir, command, COMMAND_TIMEOUT)
        self.emit(Event(kind=EVENT_COMMAND_RUN, text=_command_output_markdown(command, result)))
        return ToolResult(_command_tool_content(result))

    async def _read_file(self, args: dict[str, object]) -> ToolResult:
        path_arg = str(args.get("path", ""))
        try:
            path = self._safe_path(path_arg)
        except ValueError as err:
            return ToolResult(str(err), is_error=True)
        try:
            data = path.read_bytes()
        except OSError as err:
            return ToolResult(str(err), is_error=True)
        if len(data) > MAX_READ_FILE_BYTES:
            return ToolResult(
                data[:MAX_READ_FILE_BYTES].decode(errors="replace") + "\n[file truncated]"
            )
        return ToolResult(data.decode(errors="replace"))

    def _safe_path(self, rel: str) -> Path:
        if not rel or Path(rel).is_absolute():
            raise ValueError(f"path must be relative to the workspace: {rel!r}")
        root = Path(self.dir).resolve()
        path = (root / rel).resolve()
        if path != root and root not in path.parents:
            raise ValueError(f"path escapes the workspace: {rel!r}")
        return path

    def _snapshot(self) -> None:
        ref = snapshot_ref(self.session_id, self.snapshots)
        self.ws.snapshot(ref)
        self.last_ref = ref
        self.snapshots += 1


def _command_output_markdown(command: str, result: Result) -> str:
    body = f"```console\n$ {command}\n"
    if result.output:
        body += result.output + "\n"
    body += "```\n"
    if result.timed_out:
        body += f"*timed out after {int(COMMAND_TIMEOUT)}s and was killed*"
    elif result.exit_code == 0:
        body += "*exit code 0*"
    else:
        body += f"*exit code {result.exit_code}*"
    return body


def _command_tool_content(result: Result) -> str:
    content = f"exit code: {result.exit_code}"
    if result.timed_out:
        content = f"command timed out after {int(COMMAND_TIMEOUT)}s and was killed\n{content}"
    if not result.output:
        return content + "\n(no output)"
    return content + "\n" + result.output


def new_engine(
    ws: Workspace,
    dir: str,
    prof: Profile,
    emit: Callable[[Event], None],
    session_factory: Callable[[str, dict[str, ToolHandler]], AgentSession],
) -> Engine:
    engine = Engine(ws, dir, prof, emit)
    engine.session = session_factory(engine.system_prompt, engine.tool_handlers())
    return engine
