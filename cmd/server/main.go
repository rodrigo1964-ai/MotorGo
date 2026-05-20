// Servidor MCP HTTP para MotorRender (deploy remoto en Render).
//
// Expone 4 tools MCP que envuelven los binarios Pascal de bin/ sin modificarlos:
//   - csp_syntax_check: valida sintaxis vía bin/SyntaxChecker
//   - csp_propagate: pipeline JsonToGraph -> motor (forward|csp|fwd|bwd)
//   - csp_list_domains: lista dominios en data/Base/
//   - csp_load_model: carga modelo CSP desde data/Base/
//
// Variables de entorno:
//   PORT          - puerto HTTP (default: 8000, Render lo inyecta automáticamente)
//   HOST          - bind address (default: 0.0.0.0, escucha en todas las interfaces)
//   ALLOWED_HOST  - hostname público para la allowlist anti DNS-rebinding
//                   (default: motor-render-mcp.onrender.com)
//   PROJECT_ROOT  - raíz del proyecto (opcional, se calcula desde el ejecutable si no se especifica)
//
// Build:
//   go build -o motorgo-server ./cmd/server
//
// Ejecución local de prueba:
//   PORT=8766 ./motorgo-server
//
// El binario es autocontenido (sin dependencias de runtime) y portable.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rodolfo-rodrigo/motor-go/internal/mcp"
)

func main() {
	// Leer variables de entorno con defaults
	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[mcp] ERROR: PORT inválido: %s\n", portStr)
		os.Exit(1)
	}

	// Arrancar el servidor (bloquea hasta que se detenga)
	if err := mcp.Serve(host, port); err != nil {
		fmt.Fprintf(os.Stderr, "[mcp] ERROR: servidor falló: %v\n", err)
		os.Exit(1)
	}
}
