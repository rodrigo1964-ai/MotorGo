"""Servidor MCP stdio para MotorInferencia.

JSON-RPC 2.0 sobre stdin/stdout, vía el SDK oficial `mcp` (>= 1.0).
Expone 4 tools que envuelven los binarios Pascal de bin/ sin modificarlos.

Lanzamiento:
    python3 -m src.mcp.server

Para Claude Desktop, ver README_MCP.txt en la raíz del proyecto.
"""

from __future__ import annotations

import asyncio
import json
import logging
import sys
from typing import Any

from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import TextContent, Tool

from .errors import PascalError
from .pascal_runner import (
    DEFAULT_TIMEOUT_S,
    list_domains,
    load_model,
    run_propagate,
    run_syntax_check,
)
from .tool_schemas import TOOL_SCHEMAS

# Logging a stderr (stdout está reservado para el frame JSON-RPC).
logging.basicConfig(
    level=logging.INFO,
    stream=sys.stderr,
    format="[mcp] %(asctime)s %(levelname)s %(message)s",
)
log = logging.getLogger("motor_inferencia.mcp")

SERVER_NAME = "motor-inferencia"
SERVER_VERSION = "1.0.0"

server: Server = Server(SERVER_NAME)


# ---------------------------------------------------------------------------
# Handlers
# ---------------------------------------------------------------------------


@server.list_tools()
async def handle_list_tools() -> list[Tool]:
    """Reporta los 4 tools declarados en tool_schemas.py."""
    return [
        Tool(
            name=name,
            description=spec["description"],
            inputSchema=spec["inputSchema"],
        )
        for name, spec in TOOL_SCHEMAS.items()
    ]


def _text(payload: Any) -> list[TextContent]:
    """Serializa un payload a un único TextContent (JSON pretty-printed)."""
    body = json.dumps(payload, ensure_ascii=False, indent=2)
    return [TextContent(type="text", text=body)]


def _error_text(exc: PascalError) -> list[TextContent]:
    return [TextContent(type="text", text=exc.to_text())]


@server.call_tool()
async def handle_call_tool(
    name: str, arguments: dict[str, Any] | None
) -> list[TextContent]:
    """Dispatch al pascal_runner. No cachea entre invocaciones."""
    arguments = arguments or {}
    log.info("call_tool name=%s keys=%s", name, sorted(arguments.keys()))

    try:
        if name == "csp_syntax_check":
            modelo = arguments["modelo_json"]
            timeout = float(arguments.get("timeout_s", DEFAULT_TIMEOUT_S))
            result = await asyncio.to_thread(
                run_syntax_check, modelo, timeout_s=timeout
            )
            return _text(result)

        if name == "csp_propagate":
            modelo = arguments["modelo_json"]
            motor = arguments.get("motor", "forward")
            timeout = float(arguments.get("timeout_s", DEFAULT_TIMEOUT_S))
            result = await asyncio.to_thread(
                run_propagate, modelo, motor, timeout_s=timeout
            )
            return _text(result)

        if name == "csp_list_domains":
            result = await asyncio.to_thread(list_domains)
            return _text({"dominios": result, "total": len(result)})

        if name == "csp_load_model":
            dominio = arguments["dominio"]
            subdominio = arguments["subdominio"]
            result = await asyncio.to_thread(load_model, dominio, subdominio)
            return _text(result)

        raise PascalError(f"tool desconocido: {name}", binary="server")

    except PascalError as e:
        log.warning("tool %s error: %s", name, e)
        # El SDK convierte una excepción en CallToolResult con isError=true.
        # Re-raise como RuntimeError con texto para que aparezca en el cliente.
        raise RuntimeError(e.to_text()) from e
    except KeyError as e:
        raise RuntimeError(f"argumento requerido faltante: {e}") from e


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


async def main() -> None:
    log.info("MotorInferencia MCP server %s iniciando (stdio)", SERVER_VERSION)
    async with stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            server.create_initialization_options(),
        )
    log.info("MotorInferencia MCP server detenido")


if __name__ == "__main__":
    asyncio.run(main())
