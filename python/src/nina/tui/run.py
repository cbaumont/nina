from __future__ import annotations

from nina.agent.claude_sdk import ClaudeSdkAgentSession
from nina.engine import new_engine
from nina.events import STATE_DONE
from nina.profile import load as load_profile
from nina.state import load as load_session
from nina.state import load_history
from nina.tui.app import NinaApp
from nina.workspace import open_workspace


def run(goal: str, dir: str, is_start: bool) -> None:
    sess = load_session(dir)
    if not is_start:
        if sess is None:
            raise RuntimeError("no session to resume here; start one with nina start")
        if sess.state == STATE_DONE:
            raise RuntimeError("the last session is complete; start a new one with nina start")
    elif sess is not None and sess.state != STATE_DONE:
        raise RuntimeError(
            f"a session is already in progress here ({sess.plan_title}); continue it "
            "with nina resume, or delete .nina/ to start over"
        )

    ws = open_workspace(dir)
    prof, profile_found = load_profile(dir)
    prior_history = "" if is_start else load_history(dir)

    engine = new_engine(
        ws, dir, prof, lambda ev: None, lambda sp, h: ClaudeSdkAgentSession(sp, dir, h)
    )
    if not is_start:
        assert sess is not None
        engine.restore(sess)
        goal = sess.goal

    need_setup = not profile_found and sess is None
    need_goal = is_start and goal == ""

    app = NinaApp(engine, goal, dir, need_setup, need_goal, prior_history)
    app.run()
