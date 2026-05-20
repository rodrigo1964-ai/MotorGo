#!/usr/bin/env bash
# Lanzador del servidor MCP de MotorInferencia (transporte stdio).
# Activa el venv local y ejecuta el módulo src.mcp.server.
#
# Uso desde Claude Desktop: ver README_MCP.txt.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"

VENV_PY="$HERE/.venv/bin/python"
if [[ ! -x "$VENV_PY" ]]; then
  echo "ERROR: $VENV_PY no existe. Ejecute:" >&2
  echo "  python3 -m venv .venv && .venv/bin/pip install -r requirements.txt" >&2
  exit 1
fi

exec "$VENV_PY" -m src.mcp.server
