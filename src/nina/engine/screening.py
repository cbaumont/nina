from __future__ import annotations

import re
from collections.abc import Awaitable, Callable

from claude_agent_sdk import AssistantMessage, ClaudeAgentOptions, TextBlock, query

from nina.agent import ollama
from nina.agent.model import is_ollama, ollama_model_name
from nina.engine.events import STATE_DRIVE

_FENCED_BLOCK_RE = re.compile(r"```[^\n]*\n(.*?)```", re.S)

SCREEN_MODEL = "claude-haiku-4-5-20251001"

SCREEN_SYSTEM_PROMPT = (
    "You are a strict classifier inside a pair-programming teaching tool. The "
    "learner must type the step's code themselves; the navigator may guide but "
    "not hand over the solution. Decide whether the navigator message contains "
    "complete code that solves the learner's current step — code they could "
    "paste to finish the step without writing it themselves. Guidance, "
    "construct names, and small fragments (an import, a signature, one short "
    "illustrative line) are fine. Reply with exactly one word: LEAK if the "
    "message hands over the step's solution, otherwise OK."
)

REWRITE_NOTE = (
    "System note: your previous message contained complete code solving the "
    "current step, which the typing dial forbids. Rewrite the message now: "
    "keep the teaching content and name the constructs to use, but let the "
    "learner write the code. Reply with the rewritten message only."
)

LEAK_WARNING = (
    "> ⚠️ *Heads-up: this may include more of the solution than intended — try "
    "writing your own version before reading closely.*\n\n"
)


def is_active(dial: int, state: str) -> bool:
    return dial <= 1 and state == STATE_DRIVE


async def leaks(step_goal: str, text: str, model: str | None = None) -> bool:
    if "```" not in text:
        return False
    code_lines = sum(
        1 for block in _FENCED_BLOCK_RE.findall(text) for line in block.splitlines() if line.strip()
    )
    if code_lines <= 1:
        return False
    prompt = f"Current step goal:\n{step_goal}\n\nNavigator message:\n{text}"
    if is_ollama(model):
        assert model is not None
        reply = await ollama.classify(SCREEN_SYSTEM_PROMPT, prompt, ollama_model_name(model))
    else:
        options = ClaudeAgentOptions(
            system_prompt=SCREEN_SYSTEM_PROMPT, model=SCREEN_MODEL, max_turns=1
        )
        reply = ""
        async for message in query(prompt=prompt, options=options):
            if isinstance(message, AssistantMessage):
                reply += "".join(
                    block.text for block in message.content if isinstance(block, TextBlock)
                )
    return "LEAK" in reply.upper()


async def screen_text(
    step_goal: str, text: str, rewrite: Callable[[str], Awaitable[str]], model: str | None = None
) -> str:
    if not await leaks(step_goal, text, model):
        return text
    rewritten = await rewrite(REWRITE_NOTE)
    if rewritten.strip() == "":
        return text
    if await leaks(step_goal, rewritten, model):
        return LEAK_WARNING + rewritten
    return rewritten
