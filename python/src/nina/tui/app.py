from __future__ import annotations

import contextlib
from collections.abc import Coroutine

from rich.markdown import Markdown
from textual.app import App, ComposeResult
from textual.widgets import Input, RichLog, Static

from nina.engine import Engine, RateLimitExceeded
from nina.events import (
    EVENT_COMMAND_RUN,
    EVENT_CONFIRM,
    EVENT_INFO,
    EVENT_NUDGE,
    EVENT_PLAN_SET,
    EVENT_REVIEW,
    EVENT_SESSION_DONE,
    EVENT_STEP_STARTED,
    EVENT_TEXT_DELTA,
    STATE_DRIVE,
    STATE_IDLE,
    ConfirmAnswer,
    ConfirmRequest,
    Event,
)
from nina.profile import parse_dial
from nina.state import save_history
from nina.tui.commands import COMMANDS
from nina.tui.commands import match_commands as match_slash_commands
from nina.tui.setup import SETUP_QUESTIONS, SetupFlow, apply_setup_answer, setup_question

WELCOME_GOAL = (
    "## Welcome to Nina\n\nWhat would you like to learn or build? Tell Nina your goal "
    "— you can always refine it later.\n"
)
WELCOME_SETUP = (
    "## Welcome to Nina\n\nA quick minute of setup so Nina can teach at your level — "
    "press Enter to keep any default.\n"
)


class NinaApp(App[None]):
    CSS = """
    #status { background: $primary; color: $text; padding: 0 1; height: 1; dock: top; }
    #transcript { height: 1fr; }
    #suggestions { height: auto; color: $text-muted; padding: 0 1; }
    """
    BINDINGS = [("ctrl+c", "quit", "Quit")]

    def __init__(
        self,
        engine: Engine,
        goal: str,
        dir: str,
        need_setup: bool,
        need_goal: bool,
        prior_history: str,
        cred_note: str | None = None,
    ) -> None:
        super().__init__()
        self.engine = engine
        self.goal = goal
        self.dir = dir
        self.busy = False
        self.busy_label = ""
        self.pending_confirm: ConfirmRequest | None = None
        self.nudged_step = -1
        self.setup: SetupFlow | None = None
        self.awaiting_goal = False
        self.setup_after_goal = False
        self.plan_title = ""
        self.step_index = 0
        self.step_count = 0
        self.history = ""
        self._streaming = ""

        if need_goal:
            self.awaiting_goal = True
            self.setup_after_goal = need_setup
            self.history = WELCOME_GOAL
        elif need_setup:
            self.setup = SetupFlow(prof=engine.profile)
            self.history = WELCOME_SETUP + setup_question(0, self.setup.prof)

        if engine.state != STATE_IDLE:
            self.plan_title = engine.plan.title
            self.step_count = len(engine.plan.steps)
            self.step_index = engine.step_index
            if self.step_index < len(engine.plan.steps):
                step = engine.plan.steps[self.step_index]
                resumed = (
                    f"## {engine.plan.title}\n\n"
                    f"**Session resumed** at step {self.step_index + 1}/{self.step_count}: "
                    f"{step.title}\n\nStep goal: {step.goal}\n\nKeep working in your editor "
                    "and `/done` when ready — or ask Nina to remind you where you left off.\n"
                )
            else:
                resumed = (
                    "**Session resumed** — you were still choosing a project. Ask Nina to "
                    "repeat the ideas, or tell it what you'd like to build.\n"
                )
            self.history = (prior_history + "\n---\n" + resumed) if prior_history else resumed

        if cred_note:
            self.history = f"> {cred_note}\n\n{self.history}" if self.history else f"> {cred_note}"

    def compose(self) -> ComposeResult:
        yield Static(id="status")
        yield RichLog(id="transcript", wrap=True, markup=False)
        yield Static(id="suggestions")
        yield Input(
            placeholder="Ask Nina anything · /done when finished · /help for commands",
            id="input",
        )

    def on_mount(self) -> None:
        self.engine.emit = self._on_event
        self._refresh_status()
        if self.history:
            self.query_one("#transcript", RichLog).write(Markdown(self.history))
        self.query_one("#input", Input).focus()
        if self.engine.state == STATE_IDLE and self.setup is None and not self.awaiting_goal:
            self._set_busy(True, "brainstorming project ideas")
            self.run_worker(self._do(self.engine.start(self.goal)))

    def on_input_submitted(self, event: Input.Submitted) -> None:
        text = event.value.strip()
        self.query_one("#input", Input).value = ""
        self._handle_input(text)

    def on_input_changed(self, event: Input.Changed) -> None:
        text = event.value
        if (
            self.setup is not None
            or self.awaiting_goal
            or self.pending_confirm is not None
            or not text.startswith("/")
            or " " in text
            or "\t" in text
        ):
            suggestions = []
        else:
            suggestions = match_slash_commands(text)
        panel = self.query_one("#suggestions", Static)
        panel.update("\n".join(f"{c.display()}  {c.desc}" for c in suggestions))

    async def _do(self, coro: Coroutine[None, None, None]) -> None:
        error: Exception | None = None
        rate_limited: RateLimitExceeded | None = None
        try:
            await coro
        except RateLimitExceeded as err:
            rate_limited = err
        except Exception as err:
            error = err
        self.busy = False
        self.busy_label = ""
        self._flush_streaming()
        if rate_limited is not None:
            self._write_markdown(
                f"> ⏳ {rate_limited}\n>\n"
                "> Your progress is saved — run `nina resume` after the window resets."
            )
            self._persist_history()
            self.exit()
            return
        if error is not None:
            self._write_markdown(f"**Error:** {error}")
        self._persist_history()
        self._refresh_status()

    def _handle_input(self, text: str) -> None:
        if self.pending_confirm is not None:
            self._handle_confirm(text.lower())
            return
        if self.awaiting_goal:
            self._handle_goal(text)
            return
        if self.setup is not None:
            self._handle_setup(text)
            return
        if text in ("/quit", "/exit"):
            self.exit()
            return
        if text == "" or self.busy:
            return
        if text == "/run" or text.startswith("/run "):
            command = text[len("/run") :].strip()
            message = (
                f"Please run `{command}` now and walk me through the output."
                if command
                else "Please run the project (or its tests) now and walk me through the output."
            )
            self._send_to_nina(f"`{text}`", "running", message)
            return
        if text.startswith("/dial "):
            self._handle_dial_set(text[len("/dial ") :].strip())
            return
        if text == "/done":
            self._set_busy(True, "reviewing your changes")
            self._write_markdown("---\n\n`/done`")
            self.run_worker(self._do(self.engine.done()))
            return
        if text == "/skip":
            self._set_busy(True, "skipping to the next step")
            self._write_markdown("---\n\n`/skip`")
            self.run_worker(self._do(self.engine.skip()))
            return
        if text == "/why":
            self._send_to_nina(
                f"`{text}`",
                "thinking",
                "Why this step? Zoom out: explain how it fits into the bigger picture of "
                "what we're building and why it comes now.",
            )
            return
        if text == "/stuck":
            self._send_to_nina(
                f"`{text}`",
                "thinking",
                "I'm stuck on the current step. Help me get moving again, escalating per "
                "my hint settings — start with your next-strongest hint, not the full "
                "solution.",
            )
            return
        if text == "/recap":
            self._send_to_nina(
                f"`{text}`",
                "recapping",
                "Recap the session so far: what we've built, the concepts covered, and "
                "how the pieces fit together.",
            )
            return
        if text == "/summary":
            self._set_busy(True, "writing your session summary")
            self._write_markdown("---\n\n`/summary`")
            self.run_worker(self._do(self.engine.summarize()))
            return
        if text == "/copy":
            self._handle_copy()
            return
        if text == "/profile":
            self.setup = SetupFlow(prof=self.engine.profile, editing=True)
            self._write_markdown(
                "Adjust your profile — press Enter to keep any current value.\n"
                + setup_question(0, self.setup.prof)
            )
            return
        if text == "/dial":
            self._write_markdown(
                f"> The typing dial is at {self.engine.profile.dial}. Change it with `/dial <0-3>`."
            )
            return
        if text == "/help":
            lines = "\n".join(f"> - `{c.display()}` — {c.desc}" for c in COMMANDS)
            self._write_markdown(f"**Commands:**\n{lines}")
            return
        if text.startswith("/"):
            self._write_markdown(f"> Unknown command `{text}` — `/help` lists the commands.")
            return
        self._send_to_nina(f"**You:** {text}", "thinking", text)

    def _handle_dial_set(self, value: str) -> None:
        try:
            dial = parse_dial(value)
        except ValueError as err:
            self._write_markdown(f"> {err}")
            return
        prof = self.engine.profile
        prof.dial = dial
        self.engine.update_profile(prof)
        self._write_markdown(f"> 🎚️ Typing dial set to {dial}.")
        self._refresh_status()

    def _handle_copy(self) -> None:
        try:
            import pyperclip

            pyperclip.copy(self.history)
            self._write_markdown("> 📋 Session copied to clipboard.")
        except Exception as err:
            self._write_markdown(f"> **Error:** could not copy: {err}")

    def _handle_goal(self, text: str) -> None:
        text = text.strip()
        if not text:
            self._write_markdown("> Please tell Nina what you'd like to learn or build.")
            return
        self.goal = text
        self.awaiting_goal = False
        if self.setup_after_goal:
            self.setup_after_goal = False
            self.setup = SetupFlow(prof=self.engine.profile)
            self._write_markdown(
                "A quick minute of setup so Nina can teach at your level — press Enter "
                "to keep any default.\n" + setup_question(0, self.setup.prof)
            )
            return
        self._set_busy(True, "brainstorming project ideas")
        self.run_worker(self._do(self.engine.start(self.goal)))

    def _handle_setup(self, text: str) -> None:
        setup = self.setup
        assert setup is not None
        answer = text.strip().lower()
        if answer:
            try:
                apply_setup_answer(setup, answer)
            except ValueError as err:
                self._write_markdown(f"> {err}\n{setup_question(setup.index, setup.prof)}")
                return
        setup.index += 1
        if setup.index < SETUP_QUESTIONS:
            self._write_markdown(setup_question(setup.index, setup.prof))
            return
        self.setup = None
        self.engine.update_profile(setup.prof)
        if setup.editing:
            self._write_markdown("> ✅ Profile updated — Nina adapts from the next message.")
            return
        self._set_busy(True, "brainstorming project ideas")
        self.run_worker(self._do(self.engine.start(self.goal)))

    def _handle_confirm(self, answer: str) -> None:
        req = self.pending_confirm
        assert req is not None
        if answer in ("y", "yes"):
            reply = ConfirmAnswer(approve=True)
        elif answer in ("a", "always"):
            reply = ConfirmAnswer(approve=True, always=True)
        elif answer in ("n", "no"):
            reply = ConfirmAnswer(approve=False)
        else:
            self._write_markdown(
                "> Please answer **y** (run once), **a** (always this session), or **n** (skip)."
            )
            return
        self.pending_confirm = None
        label = "skipped"
        if reply.approve:
            label = "approved for this session" if reply.always else "approved"
        self._write_markdown(f"> `{req.command}` {label}")
        req.reply.set_result(reply)

    def _send_to_nina(self, display: str, label: str, message: str) -> None:
        self._set_busy(True, label)
        self._write_markdown(f"---\n\n{display}")
        self.run_worker(self._do(self.engine.user_message(message)))

    def _on_event(self, ev: Event) -> None:
        if ev.kind == EVENT_TEXT_DELTA:
            self._streaming += ev.text
            return
        self._flush_streaming()
        if ev.kind == EVENT_INFO:
            self._write_markdown(f"> {ev.text}")
        elif ev.kind == EVENT_COMMAND_RUN:
            self._write_markdown(ev.text)
        elif ev.kind == EVENT_CONFIRM:
            assert ev.confirm is not None
            self.pending_confirm = ev.confirm
            reason = f" — {ev.confirm.reason}" if ev.confirm.reason else ""
            self._write_markdown(
                f"> ⚡ Nina wants to run `{ev.confirm.command}`{reason}\n>\n"
                "> **y** run once · **a** always this session · **n** skip"
            )
        elif ev.kind == EVENT_PLAN_SET:
            assert ev.plan is not None
            self.plan_title = ev.plan.title
            self.step_count = len(ev.plan.steps)
            steps = "\n".join(f"{i + 1}. {s.title}" for i, s in enumerate(ev.plan.steps))
            self._write_markdown(f"## {ev.plan.title}\n\n{steps}")
        elif ev.kind == EVENT_STEP_STARTED:
            self.step_index = ev.step
        elif ev.kind == EVENT_REVIEW:
            icon = "✅" if ev.verdict == "pass" else "🔄"
            self._write_markdown(f"{icon} **Review ({ev.verdict}):** {ev.text}")
        elif ev.kind == EVENT_NUDGE:
            if self.busy or self.engine.state != STATE_DRIVE or self.nudged_step == self.step_index:
                return
            self.nudged_step = self.step_index
            self._write_markdown(
                "> 👀 Looks like you've made changes and paused — `/done` whenever you "
                "want a review."
            )
        elif ev.kind == EVENT_SESSION_DONE:
            self._write_markdown(
                "🎉 **Session complete!** You worked through every step. `/quit` when you're ready."
            )
            self._set_busy(True, "writing your session summary")
            self.run_worker(self._do(self.engine.summarize()))
        self._refresh_status()

    def _flush_streaming(self) -> None:
        if not self._streaming:
            return
        text = self._streaming
        self._streaming = ""
        self._write_markdown(text)

    def _write_markdown(self, text: str) -> None:
        self.history += "\n" + text + "\n"
        with contextlib.suppress(LookupError):
            self.query_one("#transcript", RichLog).write(Markdown(text))

    def _persist_history(self) -> None:
        if self.dir:
            save_history(self.dir, self.history)

    def _set_busy(self, busy: bool, label: str) -> None:
        self.busy = busy
        self.busy_label = label
        self._refresh_status()

    def _refresh_status(self) -> None:
        title = self.plan_title or self.goal
        parts = ["nina", title]
        if self.step_count > 0:
            step = min(self.step_index + 1, self.step_count)
            parts.append(f"step {step}/{self.step_count}")
        parts.append(f"dial {self.engine.profile.dial}")
        if self.busy:
            parts.append(f"⋯ {self.busy_label}")
        with contextlib.suppress(LookupError):
            self.query_one("#status", Static).update("  •  ".join(parts))
