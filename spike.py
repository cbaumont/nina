import asyncio
import os

from claude_agent_sdk import (
    AssistantMessage,
    ClaudeAgentOptions,
    ClaudeSDKClient,
    StreamEvent,
    SystemMessage,
    TextBlock,
    ToolUseBlock,
    create_sdk_mcp_server,
    tool,
)

TOOL_NAMES = [
    "propose_ideas",
    "set_plan",
    "instruct_step",
    "write_file",
    "run_command",
    "submit_review",
]

CALLS: dict[str, list[dict[str, object]]] = {name: [] for name in TOOL_NAMES}


def make_tool(name: str) -> object:
    @tool(name, f"Nina tool: {name}.", {"value": str})
    async def handler(args: dict[str, object]) -> dict[str, object]:
        CALLS[name].append(args)
        return {"content": [{"type": "text", "text": f"{name.upper()} RECEIVED {args['value']}"}]}

    return handler


async def main() -> None:
    assert "ANTHROPIC_API_KEY" not in os.environ, (
        "unset ANTHROPIC_API_KEY for a clean subscription-auth spike"
    )

    handlers = [make_tool(n) for n in TOOL_NAMES]
    server = create_sdk_mcp_server(name="nina", version="0.1.0", tools=handlers)

    options = ClaudeAgentOptions(
        system_prompt=(
            "You are a test harness standing in for six Nina tools. When asked to call "
            "a tool, call it directly with the exact value given, then report the tool's "
            "own output verbatim and stop. Never search for tools, never ask a clarifying "
            "question."
        ),
        tools=[],
        setting_sources=[],
        env={"CLAUDE_CODE_DISABLE_AUTO_MEMORY": "1"},
        mcp_servers={"nina": server},
        allowed_tools=[f"mcp__nina__{n}" for n in TOOL_NAMES],
        permission_mode="dontAsk",
        include_partial_messages=True,
        max_turns=4,
    )

    api_key_source: str | None = None
    saw_delta = False
    init_tools: list[str] = []
    tool_calls: list[str] = []
    final_text = ""

    async with ClaudeSDKClient(options=options) as client:
        await client.query("Call the write_file tool with value=banana.")

        async for message in client.receive_response():
            if isinstance(message, SystemMessage) and message.subtype == "init":
                api_key_source = message.data.get("apiKeySource")
                init_tools = list(message.data.get("tools") or [])
            elif isinstance(message, StreamEvent):
                if message.event.get("type") == "content_block_delta":
                    saw_delta = True
            elif isinstance(message, AssistantMessage):
                for block in message.content:
                    if isinstance(block, ToolUseBlock):
                        tool_calls.append(block.name)
                    if isinstance(block, TextBlock):
                        final_text += block.text

    print("=== SPIKE RESULTS ===")
    print(f"apiKeySource: {api_key_source!r}")
    print(f"init tools ({len(init_tools)}): {init_tools}")
    print(f"tool_calls: {tool_calls}")
    print(f"recorded calls: {CALLS}")
    print(f"saw_delta (partial messages): {saw_delta}")
    print(f"final assistant text: {final_text.strip()!r}")

    assert api_key_source in ("none", "oauth"), (
        f"expected subscription auth (apiKeySource none/oauth), got {api_key_source!r}"
    )

    forbidden = {"Read", "Write", "Bash", "Edit", "Glob", "Grep", "WebFetch", "WebSearch"}
    leaked = forbidden & set(init_tools)
    assert not leaked, f"built-in tools leaked into init tools list despite tools=[]: {leaked}"
    assert len(init_tools) == len(TOOL_NAMES), (
        f"expected all {len(TOOL_NAMES)} mcp schemas present at init with no deferral, "
        f"got {len(init_tools)}: {init_tools}"
    )

    assert CALLS["write_file"], "write_file was never invoked — in-process @tool contract failed"
    assert CALLS["write_file"][0]["value"] == "banana", f"tool received wrong args: {CALLS}"
    assert "mcp__nina__write_file" in tool_calls
    assert not any("search" in c.lower() for c in tool_calls), (
        "model called a tool-search tool before write_file — schema deferral is active"
    )

    assert saw_delta, "no partial-message deltas observed despite include_partial_messages=True"
    assert "BANANA" in final_text.upper(), (
        "final text doesn't reflect the tool's own output — built-ins may not be fully removed"
    )

    print("\nAll SDK contracts verified: subscription auth, tools=[] removal, in-process")
    print("@tool invocation with correct args, streaming deltas, no tool-search deferral.")


if __name__ == "__main__":
    asyncio.run(main())
