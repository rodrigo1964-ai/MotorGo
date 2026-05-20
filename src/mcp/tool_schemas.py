"""JSON Schemas (inputSchema) por tool MCP.

Una sola fuente de verdad. server.py los importa y los pasa tal cual al
SDK mcp. La validación se hace contra estos schemas antes de invocar a
pascal_runner.
"""

from __future__ import annotations

from typing import Any

# Schema reutilizable para un modelo CSP en memoria.
# No restringe tipos internos (eso lo hace SyntaxChecker); solo exige
# que sea un objeto con 'variables' y 'expressions'.
_MODELO_CSP_SCHEMA: dict[str, Any] = {
    "type": "object",
    "required": ["variables", "expressions"],
    "properties": {
        "_metadata": {"type": "object"},
        "variables": {"type": "array"},
        "expressions": {"type": "array"},
        "functions": {"type": "array"},
    },
    "additionalProperties": True,
}


TOOL_SCHEMAS: dict[str, dict[str, Any]] = {
    "csp_syntax_check": {
        "description": (
            "Valida un modelo CSP contra las 9 reglas semánticas R0-R8 de "
            "MotorInferencia (bin/SyntaxChecker). Devuelve {status, errors[]}. "
            "No modifica el modelo."
        ),
        "inputSchema": {
            "type": "object",
            "required": ["modelo_json"],
            "properties": {
                "modelo_json": {
                    **_MODELO_CSP_SCHEMA,
                    "description": "Modelo CSP completo (formato *_csp.json).",
                },
                "timeout_s": {
                    "type": "number",
                    "minimum": 1,
                    "maximum": 120,
                    "default": 30,
                    "description": "Timeout en segundos para SyntaxChecker.",
                },
            },
            "additionalProperties": False,
        },
    },

    "csp_propagate": {
        "description": (
            "Ejecuta el pipeline JsonToGraph → <motor> sobre un modelo CSP. "
            "motor ∈ {forward, csp, fwd, bwd} → ForwardChain/CSPEval/"
            "FwdConsistency/BwdConsistency. Devuelve el JSON del motor "
            "(status, iterations, steps[], vars[]). Recomendado validar "
            "con csp_syntax_check antes."
        ),
        "inputSchema": {
            "type": "object",
            "required": ["modelo_json"],
            "properties": {
                "modelo_json": {
                    **_MODELO_CSP_SCHEMA,
                    "description": "Modelo CSP a propagar.",
                },
                "motor": {
                    "type": "string",
                    "enum": ["forward", "csp", "fwd", "bwd"],
                    "default": "forward",
                    "description": (
                        "Motor de propagación: 'forward'=ForwardChain (trazado paso a paso), "
                        "'csp'=CSPEval (AC-3), 'fwd'=FwdConsistency, 'bwd'=BwdConsistency."
                    ),
                },
                "timeout_s": {
                    "type": "number",
                    "minimum": 1,
                    "maximum": 120,
                    "default": 30,
                },
            },
            "additionalProperties": False,
        },
    },

    "csp_list_domains": {
        "description": (
            "Lista las carpetas de dominio bajo data/Base/ con conteo de "
            "modelos *_csp.json. No acepta parámetros."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },

    "csp_load_model": {
        "description": (
            "Lee data/Base/<dominio>/<subdominio>_csp.json y devuelve el "
            "JSON completo. dominio y subdominio no pueden contener '/' "
            "ni '..'. Usar csp_list_domains para descubrir nombres."
        ),
        "inputSchema": {
            "type": "object",
            "required": ["dominio", "subdominio"],
            "properties": {
                "dominio": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 200,
                    "description": "Carpeta bajo data/Base/, ej. '50_evaluacion_contaminacion_suelos'.",
                },
                "subdominio": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 200,
                    "description": "Nombre base del archivo *_csp.json sin la extensión ni el sufijo. Ej: 'metales_pesados' carga 'metales_pesados_csp.json'.",
                },
            },
            "additionalProperties": False,
        },
    },
}
