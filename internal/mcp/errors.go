// Package mcp implementa el wrapper MCP en Go para los binarios Pascal de MotorInferencia.
//
// Este paquete NO reimplementa la lógica CSP/AC-3: simplemente envuelve los binarios
// Pascal existentes en bin/ y expone sus capacidades vía el protocolo MCP sobre HTTP.
package mcp

import (
	"fmt"
	"strings"
)

// PascalError representa un error producido al invocar un binario Pascal del pipeline CSP.
//
// Puede originarse por:
//   - Binario no encontrado o no ejecutable
//   - Timeout durante la ejecución
//   - Exit code != 0
//   - Stdout no parseable como JSON (cuando se esperaba JSON)
//
// El método ToText() formatea el error en un string legible para el cliente MCP,
// incluyendo el nombre del binario, exit code, y fragmentos truncados de stderr/stdout.
type PascalError struct {
	// Message es el mensaje principal del error (ej: "Timeout 30s ejecutando SyntaxChecker")
	Message string

	// Binary es el nombre del binario que falló (ej: "SyntaxChecker", "ForwardChain")
	Binary string

	// ExitCode es el código de salida del proceso. nil si el proceso no se ejecutó
	// (ej: binario no encontrado) o si terminó por timeout.
	ExitCode *int

	// Stderr contiene la salida de error estándar del binario (puede estar vacía)
	Stderr string

	// Stdout contiene la salida estándar del binario (puede estar vacía)
	Stdout string
}

// Error implementa la interfaz error de Go.
func (e *PascalError) Error() string {
	return e.Message
}

// ToText formatea el error en un string legible para el cliente MCP.
//
// Formato:
//   [<binary>] <message>
//   exit_code=<code>          (si está presente)
//   stderr: <primeros 500 chars>   (si no está vacío)
//   stdout: <primeros 500 chars>   (si exit != 0 y no está vacío)
//
// Este formato replica el comportamiento de errors.py del wrapper Python original.
func (e *PascalError) ToText() string {
	var parts []string

	// Primera línea: [binary] message
	parts = append(parts, fmt.Sprintf("[%s] %s", e.Binary, e.Message))

	// Exit code si está presente
	if e.ExitCode != nil {
		parts = append(parts, fmt.Sprintf("exit_code=%d", *e.ExitCode))
	}

	// Stderr si no está vacío (truncado a 500 chars)
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		if len(stderr) > 500 {
			stderr = stderr[:500]
		}
		parts = append(parts, fmt.Sprintf("stderr: %s", stderr))
	}

	// Stdout si exit != 0 y no está vacío (truncado a 500 chars)
	// Nota: solo mostramos stdout cuando hay un exit code != 0, porque si el binario
	// terminó con exit 0, el stdout debería ser el resultado válido, no un error.
	if e.ExitCode != nil && *e.ExitCode != 0 {
		if stdout := strings.TrimSpace(e.Stdout); stdout != "" {
			if len(stdout) > 500 {
				stdout = stdout[:500]
			}
			parts = append(parts, fmt.Sprintf("stdout: %s", stdout))
		}
	}

	return strings.Join(parts, "\n")
}

// PascalTimeoutError es un PascalError específico para timeouts.
//
// Se levanta cuando un binario no termina dentro del timeout configurado.
// En Go, esto ocurre cuando el context.Context asociado al comando expira.
type PascalTimeoutError struct {
	*PascalError
}

// PascalJSONError es un PascalError específico para fallos de parseo JSON.
//
// Se levanta cuando un binario termina con exit 0 pero su stdout no es JSON válido,
// o cuando un archivo JSON (ej: modelo CSP) no se puede parsear.
type PascalJSONError struct {
	*PascalError
}
