from __future__ import annotations

import json
import os
import subprocess


def check() -> str | None:
    try:
        proc = subprocess.run(
            ["claude", "auth", "status"],
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return None
    if proc.returncode != 0:
        return None
    try:
        data = json.loads(proc.stdout)
    except json.JSONDecodeError:
        return None
    if not isinstance(data, dict):
        return None

    auth_method = data.get("authMethod")
    subscription_type = data.get("subscriptionType")
    api_provider = data.get("apiProvider")

    parts = []
    if isinstance(auth_method, str) and auth_method:
        parts.append(f"auth: {auth_method}")
    if isinstance(subscription_type, str) and subscription_type:
        parts.append(f"subscription: {subscription_type}")
    if isinstance(api_provider, str) and api_provider:
        parts.append(f"provider: {api_provider}")
    summary = ", ".join(parts) if parts else None

    warning = None
    if os.environ.get("ANTHROPIC_API_KEY") and auth_method != "api_key":
        warning = (
            "Warning: ANTHROPIC_API_KEY is set in your environment — it will be used "
            "instead of your subscription login, even though `claude auth status` "
            f"reports {auth_method or 'a different credential'}."
        )

    if warning and summary:
        return f"{warning}\n\n({summary})"
    if warning:
        return warning
    if summary:
        return f"Using {summary}."
    return None
