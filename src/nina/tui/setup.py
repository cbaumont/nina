from __future__ import annotations

from dataclasses import dataclass

from nina.system.profile import Profile, parse_dial, parse_hint_speed, parse_level

SETUP_QUESTIONS = 5


@dataclass
class SetupFlow:
    prof: Profile
    index: int = 0
    editing: bool = False


def setup_question(index: int, prof: Profile) -> str:
    if index == 0:
        return (
            "\n**1/5 — Your general programming experience?** `none` · `beginner` · "
            f"`intermediate` · `professional` *(default: {prof.experience})*\n"
        )
    if index == 1:
        return (
            "\n**2/5 — How familiar are you with the stack you're learning?** `none` · "
            f"`beginner` · `intermediate` · `professional` *(default: {prof.stack_familiarity})*\n"
        )
    if index == 2:
        known = ", ".join(prof.known_stacks) or "none"
        return (
            "\n**3/5 — Languages or stacks you already know?** comma-separated, so Nina "
            f"can use analogies *(default: {known})*\n"
        )
    if index == 3:
        return (
            "\n**4/5 — How much may Nina type?** `0` full manual · `1` scaffold only · "
            f"`2` + boilerplate · `3` collaborative *(default: {prof.dial})*\n"
        )
    return (
        "\n**5/5 — How fast should hints escalate when you're stuck?** `slow` · "
        f"`medium` · `fast` *(default: {prof.hint_escalation})*\n"
    )


def apply_setup_answer(setup: SetupFlow, answer: str) -> None:
    if setup.index == 0:
        setup.prof.experience = parse_level(answer)
    elif setup.index == 1:
        setup.prof.stack_familiarity = parse_level(answer)
    elif setup.index == 2:
        setup.prof.known_stacks = []
        if answer != "none":
            setup.prof.known_stacks = [s.strip() for s in answer.split(",") if s.strip()]
    elif setup.index == 3:
        setup.prof.dial = parse_dial(answer)
    elif setup.index == 4:
        setup.prof.hint_escalation = parse_hint_speed(answer)
