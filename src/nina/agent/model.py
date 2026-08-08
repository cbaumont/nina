from __future__ import annotations

OLLAMA_PREFIX = "ollama:"


def is_ollama(model: str | None) -> bool:
    return model is not None and model.startswith(OLLAMA_PREFIX)


def ollama_model_name(model: str) -> str:
    return model[len(OLLAMA_PREFIX) :]
