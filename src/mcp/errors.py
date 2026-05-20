"""Mapeo de errores de los binarios Pascal a respuestas MCP.

Cada binario MotorInferencia escribe JSON a stdout. Cuando el proceso falla
(exit code != 0, timeout, o JSON inválido), el wrapper levanta PascalError;
el server.py lo captura y produce un CallToolResult con isError=true.
"""

from __future__ import annotations


class PascalError(Exception):
    """Error producido al invocar un binario Pascal del pipeline CSP."""

    def __init__(
        self,
        message: str,
        *,
        binary: str,
        exit_code: int | None = None,
        stderr: str = "",
        stdout: str = "",
    ) -> None:
        super().__init__(message)
        self.binary = binary
        self.exit_code = exit_code
        self.stderr = stderr
        self.stdout = stdout

    def to_text(self) -> str:
        parts = [f"[{self.binary}] {self.args[0]}"]
        if self.exit_code is not None:
            parts.append(f"exit_code={self.exit_code}")
        if self.stderr.strip():
            parts.append(f"stderr: {self.stderr.strip()[:500]}")
        if self.stdout.strip() and self.exit_code != 0:
            parts.append(f"stdout: {self.stdout.strip()[:500]}")
        return "\n".join(parts)


class PascalTimeoutError(PascalError):
    """El binario no terminó dentro del timeout configurado."""


class PascalJSONError(PascalError):
    """El binario terminó con exit 0 pero su stdout no es JSON válido."""
