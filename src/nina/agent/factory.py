from __future__ import annotations

from collections.abc import Callable

from nina.agent.claude_sdk import ClaudeSdkAgentSession
from nina.agent.model import is_ollama, ollama_model_name
from nina.agent.ollama import OllamaAgentSession
from nina.agent.session import AgentSession, ToolHandler
from nina.system import state


def session_factory(
    dir: str, model: str | None, resume: str | None = None, replay_history: bool = False
) -> Callable[[str, dict[str, ToolHandler]], AgentSession]:
    if is_ollama(model):
        assert model is not None
        name = ollama_model_name(model)
        history = state.load_transcript(dir) if replay_history else None
        return lambda sp, h: OllamaAgentSession(sp, dir, h, model=name, history=history)
    return lambda sp, h: ClaudeSdkAgentSession(sp, dir, h, resume=resume, model=model)
