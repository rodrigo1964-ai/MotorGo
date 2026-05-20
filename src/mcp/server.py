"""Servidor MCP HTTP para MotorRender (deploy remoto en Render).

JSON-RPC 2.0 sobre HTTP streamable, vía FastMCP (mcp >= 1.2).
Expone 4 tools que envuelven los binarios Pascal de bin/ sin modificarlos.

Lanzamiento local:
    python3 -m src.mcp.server

Variables de entorno:
    PORT          - puerto HTTP (default: 8000, Render lo inyecta automáticamente)
    HOST          - bind address (default: 0.0.0.0, escucha en todas las interfaces)
    ALLOWED_HOST  - hostname público para la allowlist anti DNS-rebinding
                    (default: motor-render-mcp.onrender.com)
"""
from __future__ import annotations

import logging
import os
import sys
from typing import Any

from mcp.server.fastmcp import FastMCP
from mcp.server.transport_security import TransportSecuritySettings

from .errors import PascalError
from .pascal_runner import (
    DEFAULT_TIMEOUT_S,
    list_domains,
    load_model,
    run_propagate,
    run_syntax_check,
)

# Logging a stderr (Render lo recolecta en su panel de logs)
logging.basicConfig(
    level=logging.INFO,
    stream=sys.stderr,
    format="[mcp] %(asctime)s %(levelname)s %(message)s",
)
log = logging.getLogger("motor_render.mcp")

SERVER_NAME = "motor-render"
SERVER_VERSION = "1.0.0"

# Hostname público (Render). Configurable por env var para portabilidad.
ALLOWED_HOST = os.environ.get("ALLOWED_HOST", "motor-render-mcp.onrender.com")

# Allowlist anti DNS-rebinding: sin esto, el transport rechaza cualquier
# Host header que no sea localhost con "Invalid Host header".
_security = TransportSecuritySettings(
    enable_dns_rebinding_protection=True,
    allowed_hosts=[
        ALLOWED_HOST,
        f"{ALLOWED_HOST}:*",
        "localhost",
        "localhost:*",
        "127.0.0.1",
        "127.0.0.1:*",
    ],
    allowed_origins=[
        f"https://{ALLOWED_HOST}",
        "http://localhost:*",
        "http://127.0.0.1:*",
    ],
)

mcp = FastMCP(SERVER_NAME, transport_security=_security)


# ---------------------------------------------------------------------------
# Tools (FastMCP deriva el inputSchema de la firma de cada función)
# ---------------------------------------------------------------------------

@mcp.tool()
def csp_syntax_check(
    modelo_json: dict[str, Any],
    timeout_s: float = DEFAULT_TIMEOUT_S,
) -> dict[str, Any]:
    """Valida sintaxis de un modelo CSP vía bin/SyntaxChecker."""
    try:
        return run_syntax_check(modelo_json, timeout_s=timeout_s)
    except PascalError as e:
        raise RuntimeError(e.to_text()) from e


@mcp.tool()
def csp_propagate(
    modelo_json: dict[str, Any],
    motor: str = "forward",
    timeout_s: float = DEFAULT_TIMEOUT_S,
) -> dict[str, Any]:
    """Pipeline JsonToGraph -> <motor>. motor in {forward, csp, fwd, bwd}."""
    try:
        return run_propagate(modelo_json, motor, timeout_s=timeout_s)
    except PascalError as e:
        raise RuntimeError(e.to_text()) from e


@mcp.tool()
def csp_list_domains() -> dict[str, Any]:
    """Lista carpetas data/Base/NN_*/ con el conteo de modelos *_csp.json."""
    result = list_domains()
    return {"dominios": result, "total": len(result)}


@mcp.tool()
def csp_load_model(dominio: str, subdominio: str) -> dict[str, Any]:
    """Lee data/Base/<dominio>/<subdominio>_csp.json y devuelve el JSON."""
    try:
        return load_model(dominio, subdominio)
    except PascalError as e:
        raise RuntimeError(e.to_text()) from e


# ---------------------------------------------------------------------------
# Health endpoint (para keep-alive desde GitHub Actions o monitoreo externo)
# ---------------------------------------------------------------------------

@mcp.custom_route("/health", methods=["GET"])
async def health(_request):
    from starlette.responses import JSONResponse
    return JSONResponse({
        "status": "ok",
        "name": SERVER_NAME,
        "version": SERVER_VERSION,
    })


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> None:
    port = int(os.environ.get("PORT", "8000"))
    host = os.environ.get("HOST", "0.0.0.0")
    log.info(
        "MotorRender MCP server %s iniciando (streamable-http %s:%d)",
        SERVER_VERSION, host, port,
    )
    mcp.settings.host = host
    mcp.settings.port = port
    mcp.run(transport="streamable-http")


if __name__ == "__main__":
    main()
