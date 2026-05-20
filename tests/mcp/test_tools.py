"""Un caso por tool MCP, contra un modelo real de data/Base/.

Modelo de prueba: data/Base/50_evaluacion_contaminacion_suelos/metales_pesados_csp.json
Elegido porque pasa SyntaxChecker R0-R8 sin errores.
"""

from __future__ import annotations

import json

import pytest

from _mcp_session import mcp_session

DOMINIO = "50_evaluacion_contaminacion_suelos"
SUBDOMINIO = "metales_pesados"


def _payload(call_result) -> dict:
    """Extrae el JSON del primer TextContent."""
    assert call_result.content, "respuesta sin content"
    return json.loads(call_result.content[0].text)


@pytest.mark.asyncio
async def test_csp_list_domains():
    async with mcp_session() as session:
        res = await session.call_tool("csp_list_domains", {})
        assert not res.isError
        data = _payload(res)
        assert data["total"] >= 1
        nombres = {d["dominio"] for d in data["dominios"]}
        assert DOMINIO in nombres, f"dominio de referencia ausente"


@pytest.mark.asyncio
async def test_csp_load_model():
    async with mcp_session() as session:
        res = await session.call_tool(
            "csp_load_model", {"dominio": DOMINIO, "subdominio": SUBDOMINIO}
        )
        assert not res.isError
        modelo = _payload(res)
        assert "variables" in modelo
        assert "expressions" in modelo
        assert len(modelo["variables"]) > 0


@pytest.mark.asyncio
async def test_csp_syntax_check_modelo_valido():
    async with mcp_session() as session:
        load = await session.call_tool(
            "csp_load_model", {"dominio": DOMINIO, "subdominio": SUBDOMINIO}
        )
        modelo = _payload(load)
        res = await session.call_tool("csp_syntax_check", {"modelo_json": modelo})
        assert not res.isError, res.content[0].text
        data = _payload(res)
        assert data["status"] == "ok"
        assert data["errors"] == []


@pytest.mark.asyncio
async def test_csp_syntax_check_detecta_modelo_sin_variables():
    # Sin 'variables' viola el inputSchema; debe marcarse como error.
    async with mcp_session() as session:
        res = await session.call_tool(
            "csp_syntax_check", {"modelo_json": {"expressions": [], "functions": []}}
        )
        assert res.isError


@pytest.mark.asyncio
async def test_csp_propagate_forward():
    async with mcp_session() as session:
        load = await session.call_tool(
            "csp_load_model", {"dominio": DOMINIO, "subdominio": SUBDOMINIO}
        )
        modelo = _payload(load)
        res = await session.call_tool(
            "csp_propagate", {"modelo_json": modelo, "motor": "forward"}
        )
        assert not res.isError, res.content[0].text
        data = _payload(res)
        # ForwardChain devuelve status ∈ {ok, solved, contradiction}.
        assert data.get("status") in {"ok", "solved", "contradiction"}, data
        assert "iterations" in data
        assert isinstance(data.get("steps"), list)


@pytest.mark.asyncio
async def test_csp_load_model_path_traversal_bloqueado():
    async with mcp_session() as session:
        res = await session.call_tool(
            "csp_load_model", {"dominio": "..", "subdominio": "etc/passwd"}
        )
        assert res.isError
