"""Wrapper de subprocess para los binarios Pascal de MotorInferencia.

No reimplementa nada: invoca bin/<Tool> con archivos temporales o stdin,
captura stdout JSON y mapea fallos a PascalError.
"""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
from pathlib import Path
from typing import Any

from .errors import PascalError, PascalJSONError, PascalTimeoutError

PROJECT_ROOT = Path(__file__).resolve().parents[2]
BIN_DIR = PROJECT_ROOT / "bin"
DATA_BASE = PROJECT_ROOT / "data" / "Base"

DEFAULT_TIMEOUT_S = 30.0


def _bin_path(name: str) -> Path:
    p = BIN_DIR / name
    if not p.is_file() or not os.access(p, os.X_OK):
        raise PascalError(
            f"Binario no encontrado o no ejecutable: {p}",
            binary=name,
            exit_code=None,
        )
    return p


def _run(
    binary: str,
    args: list[str],
    *,
    timeout_s: float,
    stdin_bytes: bytes | None = None,
) -> str:
    """Ejecuta un binario y devuelve stdout (str). Levanta PascalError en fallos."""
    exe = _bin_path(binary)
    try:
        proc = subprocess.run(
            [str(exe), *args],
            input=stdin_bytes,
            capture_output=True,
            timeout=timeout_s,
            cwd=str(PROJECT_ROOT),
        )
    except subprocess.TimeoutExpired as e:
        raise PascalTimeoutError(
            f"Timeout {timeout_s}s ejecutando {binary}",
            binary=binary,
            stdout=(e.stdout or b"").decode("utf-8", "replace"),
            stderr=(e.stderr or b"").decode("utf-8", "replace"),
        ) from e
    except OSError as e:
        raise PascalError(
            f"No se pudo ejecutar {binary}: {e}",
            binary=binary,
        ) from e

    stdout = proc.stdout.decode("utf-8", "replace")
    stderr = proc.stderr.decode("utf-8", "replace")

    if proc.returncode != 0:
        raise PascalError(
            f"{binary} terminó con exit code {proc.returncode}",
            binary=binary,
            exit_code=proc.returncode,
            stdout=stdout,
            stderr=stderr,
        )
    return stdout


def _parse_json(stdout: str, *, binary: str) -> Any:
    """Parsea stdout como JSON. Tolera espacios/saltos al inicio y al final."""
    text = stdout.strip()
    if not text:
        raise PascalJSONError(
            f"{binary} devolvió stdout vacío",
            binary=binary,
            stdout=stdout,
        )
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        raise PascalJSONError(
            f"stdout de {binary} no es JSON válido: {e}",
            binary=binary,
            stdout=stdout,
        ) from e


def _write_temp_json(payload: dict[str, Any], suffix: str = "_csp.json") -> Path:
    fd, path = tempfile.mkstemp(suffix=suffix, prefix="mcp_")
    os.close(fd)
    p = Path(path)
    p.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
    return p


# ---------------------------------------------------------------------------
# Tool entry points
# ---------------------------------------------------------------------------


def run_syntax_check(modelo_json: dict[str, Any], *, timeout_s: float = DEFAULT_TIMEOUT_S) -> dict[str, Any]:
    """Invoca bin/SyntaxChecker sobre un modelo CSP en memoria."""
    tmp = _write_temp_json(modelo_json)
    try:
        out = _run("SyntaxChecker", [str(tmp)], timeout_s=timeout_s)
    finally:
        tmp.unlink(missing_ok=True)
    return _parse_json(out, binary="SyntaxChecker")


_MOTOR_MAP = {
    "forward": "ForwardChain",
    "csp": "CSPEval",
    "fwd": "FwdConsistency",
    "bwd": "BwdConsistency",
}


def run_propagate(
    modelo_json: dict[str, Any],
    motor: str = "forward",
    *,
    timeout_s: float = DEFAULT_TIMEOUT_S,
) -> dict[str, Any]:
    """Pipeline JsonToGraph → <motor>. motor ∈ forward|csp|fwd|bwd."""
    if motor not in _MOTOR_MAP:
        raise PascalError(
            f"motor inválido: {motor!r} (esperado: {sorted(_MOTOR_MAP)})",
            binary="pipeline",
        )

    modelo_tmp = _write_temp_json(modelo_json)
    graph_tmp = Path(tempfile.mkstemp(suffix="_graph.json", prefix="mcp_")[1])
    try:
        graph_out = _run("JsonToGraph", [str(modelo_tmp)], timeout_s=timeout_s)
        graph_tmp.write_text(graph_out, encoding="utf-8")
        result_out = _run(_MOTOR_MAP[motor], [str(graph_tmp)], timeout_s=timeout_s)
        return _parse_json(result_out, binary=_MOTOR_MAP[motor])
    finally:
        modelo_tmp.unlink(missing_ok=True)
        graph_tmp.unlink(missing_ok=True)


def list_domains() -> list[dict[str, Any]]:
    """Lista carpetas data/Base/NN_*/ con el conteo de modelos *_csp.json."""
    if not DATA_BASE.is_dir():
        return []
    dominios = []
    for sub in sorted(DATA_BASE.iterdir()):
        if not sub.is_dir():
            continue
        if sub.name.startswith("_"):
            continue  # logs, reportes
        modelos = sorted(p.name for p in sub.glob("*_csp.json"))
        dominios.append({
            "dominio": sub.name,
            "ruta": str(sub.relative_to(PROJECT_ROOT)),
            "n_modelos": len(modelos),
            "modelos": modelos,
        })
    return dominios


def load_model(dominio: str, subdominio: str) -> dict[str, Any]:
    """Lee data/Base/<dominio>/<subdominio>_csp.json y devuelve el JSON parseado."""
    # Defensa contra path traversal.
    if "/" in dominio or "/" in subdominio or ".." in dominio or ".." in subdominio:
        raise PascalError(
            "dominio/subdominio no pueden contener '/' ni '..'",
            binary="load_model",
        )
    candidate = DATA_BASE / dominio / f"{subdominio}_csp.json"
    # Verificación final: el path resuelto debe seguir bajo DATA_BASE.
    try:
        resolved = candidate.resolve()
        resolved.relative_to(DATA_BASE.resolve())
    except (OSError, ValueError) as e:
        raise PascalError(
            f"ruta inválida: {candidate}",
            binary="load_model",
        ) from e
    if not resolved.is_file():
        raise PascalError(
            f"archivo no encontrado: {candidate.relative_to(PROJECT_ROOT)}",
            binary="load_model",
        )
    try:
        return json.loads(resolved.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        raise PascalJSONError(
            f"archivo no es JSON válido: {e}",
            binary="load_model",
            stdout=resolved.read_text(encoding="utf-8")[:500],
        ) from e
