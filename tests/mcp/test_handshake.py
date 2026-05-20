"""Verifica initialize + tools/list contra el servidor MCP."""

from __future__ import annotations

import pytest

from _mcp_session import mcp_session

EXPECTED_TOOLS = {
    "csp_syntax_check",
    "csp_propagate",
    "csp_list_domains",
    "csp_load_model",
}


@pytest.mark.asyncio
async def test_handshake_y_tools_list():
    async with mcp_session() as session:
        tools = await session.list_tools()
        names = {t.name for t in tools.tools}
        assert names == EXPECTED_TOOLS, f"tools inesperados: {names ^ EXPECTED_TOOLS}"


@pytest.mark.asyncio
async def test_tools_tienen_inputSchema_objeto():
    async with mcp_session() as session:
        tools = await session.list_tools()
        for t in tools.tools:
            assert t.inputSchema, f"{t.name} no expone inputSchema"
            assert t.inputSchema.get("type") == "object", f"{t.name} inputSchema no es object"
            assert t.description and len(t.description) > 20, f"{t.name} sin descripción"
