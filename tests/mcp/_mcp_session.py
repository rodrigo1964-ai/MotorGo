"""Helpers compartidos para tests MCP.

Las versiones recientes de pytest-asyncio (>=1.x) ejecutan setup y teardown
de fixtures async-generator en tareas distintas. Eso choca con las
cancel-scopes de anyio que `mcp.client.stdio.stdio_client` instala
internamente. Para evitar ese conflicto, exponemos un async context manager
que se entra y se sale dentro de un mismo `async with` en el cuerpo del test.
"""

from __future__ import annotations

import sys
from contextlib import asynccontextmanager
from pathlib import Path

from mcp.client.session import ClientSession
from mcp.client.stdio import StdioServerParameters, stdio_client

PROJECT_ROOT = Path(__file__).resolve().parents[2]


@asynccontextmanager
async def mcp_session():
    """Levanta el server por stdio y devuelve una ClientSession ya inicializada.

    Uso en tests::

        async with mcp_session() as session:
            ...
    """
    params = StdioServerParameters(
        command=sys.executable,
        args=["-m", "src.mcp.server"],
        cwd=str(PROJECT_ROOT),
    )
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            yield session
