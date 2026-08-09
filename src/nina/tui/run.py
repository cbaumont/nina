from __future__ import annotations

from nina.agent.factory import session_factory
from nina.engine import new_engine
from nina.engine.events import STATE_DONE
from nina.system import credentials
from nina.system.profile import load as load_profile
from nina.system.state import load as load_session
from nina.system.state import load_history
from nina.system.workspace import open_workspace
from nina.tui.app import NinaApp


def run(
    goal: str, dir: str, is_start: bool, model: str | None = None, auto: bool = False
) -> None:
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
    resume_id = sess.sdk_session_id if (not is_start and sess is not None) else None
    if not is_start and sess is not None:
        model = sess.model
        auto = sess.auto

    engine = new_engine(
        ws,
        dir,
        prof,
        lambda ev: None,
        session_factory(dir, model, resume=resume_id, replay_history=not is_start),
        model=model,
        auto=auto,
    )
    if not is_start:
        assert sess is not None
        engine.restore(sess)
        goal = sess.goal

    need_setup = not profile_found and sess is None
    need_goal = is_start and goal == ""

    app = NinaApp(engine, goal, dir, need_setup, need_goal, prior_history, credentials.check())
    app.run()
