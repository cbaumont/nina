from __future__ import annotations

import subprocess
from typing import Any

import pytest

from nina.system import credentials


def _run(returncode: int = 0, stdout: str = "{}") -> Any:
    return subprocess.CompletedProcess(args=["claude"], returncode=returncode, stdout=stdout)


def test_check_returns_none_when_cli_missing(monkeypatch: pytest.MonkeyPatch) -> None:
    def raise_missing(*args: object, **kwargs: object) -> Any:
        raise FileNotFoundError

    monkeypatch.setattr(subprocess, "run", raise_missing)
    assert credentials.check() is None


def test_check_returns_none_on_nonzero_exit(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(subprocess, "run", lambda *a, **k: _run(returncode=1))
    assert credentials.check() is None


def test_check_summarizes_subscription_auth(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    monkeypatch.setattr(
        subprocess,
        "run",
        lambda *a, **k: _run(stdout='{"authMethod": "subscription", "subscriptionType": "max"}'),
    )
    note = credentials.check()
    assert note is not None
    assert "subscription" in note


def test_check_warns_when_api_key_shadows_subscription(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-test")
    monkeypatch.setattr(
        subprocess,
        "run",
        lambda *a, **k: _run(stdout='{"authMethod": "subscription"}'),
    )
    note = credentials.check()
    assert note is not None
    assert "ANTHROPIC_API_KEY" in note


def test_check_no_warning_when_api_key_is_the_auth_method(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-test")
    monkeypatch.setattr(
        subprocess,
        "run",
        lambda *a, **k: _run(stdout='{"authMethod": "api_key"}'),
    )
    note = credentials.check()
    assert note is not None
    assert "Warning" not in note


def test_check_returns_none_on_bad_json(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(subprocess, "run", lambda *a, **k: _run(stdout="not json"))
    assert credentials.check() is None
