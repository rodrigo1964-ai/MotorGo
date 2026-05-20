package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk"
)

const (
	// ServerName es el nombre del servidor MCP (debe coincidir con el wrapper Python)
	ServerName = "motor-render"

	// ServerVersion es la versión del servidor MCP
	ServerVersion = "1.0.0"
)

// allowedHosts es la allowlist de Host headers permitidos (anti DNS-rebinding).
//
// Esta lista se construye en initAllowedHosts() usando la variable de entorno
// ALLOWED_HOST (default: motor-render-mcp.onrender.com).
//
// Patrón de seguridad: DNS rebinding attack
// Sin esta protección, un atacante podría crear un sitio web malicioso que
// haga requests al servidor MCP usando un hostname que el atacante controla,
// y que resuelve a 127.0.0.1 o a la IP interna del servidor. El navegador
// de la víctima haría el request con el hostname del atacante en el header Host,
// y el servidor respondería (porque está escuchando en 0.0.0.0).
// Al validar el Host header, rechazamos requests que no vengan de hostnames
// conocidos y confiables.
var allowedHosts []string

// initAllowedHosts construye la allowlist de Host headers a partir de ALLOWED_HOST.
func initAllowedHosts() {
	allowedHost := os.Getenv("ALLOWED_HOST")
	if allowedHost == "" {
		allowedHost = "motor-render-mcp.onrender.com"
	}

	// Permitir el hostname con y sin puerto
	// También permitir localhost y 127.0.0.1 (con y sin puerto) para testing local
	allowedHosts = []string{
		allowedHost,
		"localhost",
		"127.0.0.1",
	}
}

// isHostAllowed verifica si un Host header está en la allowlist.
//
// Lógica:
//   - Si host es exactamente igual a algún elemento de allowedHosts: OK
//   - Si host empieza con "<elemento>:" (hostname con puerto): OK
//   - Sino: rechazar
//
// Ejemplos:
//   isHostAllowed("motor-render-mcp.onrender.com") → true
//   isHostAllowed("motor-render-mcp.onrender.com:443") → true
//   isHostAllowed("localhost:8766") → true
//   isHostAllowed("evil.com") → false
func isHostAllowed(host string) bool {
	for _, allowed := range allowedHosts {
		if host == allowed {
			return true
		}
		// Permitir hostname con puerto: "allowed:1234"
		if strings.HasPrefix(host, allowed+":") {
			return true
		}
	}
	return false
}

// dnsRebindingProtection es un middleware HTTP que valida el Host header.
//
// Si el Host header no está en la allowlist, devuelve HTTP 400 Bad Request
// con el mensaje "Invalid Host header".
//
// Go idiom: middleware HTTP
// Un middleware en Go es una función que toma un http.Handler y devuelve otro http.Handler.
// Patrón:
//   func middleware(next http.Handler) http.Handler {
//       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//           // ... lógica del middleware ...
//           next.ServeHTTP(w, r) // llamar al siguiente handler
//       })
//   }
func dnsRebindingProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if !isHostAllowed(host) {
			http.Error(w, "Invalid Host header", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// healthHandler implementa el endpoint GET /health.
//
// Devuelve HTTP 200 con el JSON:
//   {"status":"ok","name":"motor-render","version":"1.0.0"}
//
// Este endpoint NO pasa por la lógica MCP y debe responder siempre rápido.
// Se usa para keep-alive desde GitHub Actions, monitoreo externo, o health checks de Render.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"name":    ServerName,
		"version": ServerVersion,
	})
}

// ---------------------------------------------------------------------------
// Handlers de los 4 tools MCP
// ---------------------------------------------------------------------------

// Patrón de handler de tool en el SDK de Go:
//   func(ctx context.Context, req *mcp.CallToolRequest, args <TipoParams>) (*mcp.CallToolResult, any, error)
//
// Parámetros:
//   - ctx: context.Context para cancelación y timeouts
//   - req: metadata del request (no lo usamos en estos tools)
//   - args: struct con los parámetros del tool (el SDK deserializa automáticamente)
//
// Retorno:
//   - *mcp.CallToolResult: metadata de la respuesta (nil si no hay metadata especial)
//   - any: el resultado del tool (un map, struct, o cualquier cosa serializable a JSON)
//   - error: error si hubo fallo (el SDK lo convierte en un CallToolResult con isError=true)
//
// IMPORTANTE: si devolvemos un error, el SDK ya lo maneja y envía un CallToolResult
// con isError=true al cliente MCP. No necesitamos construir CallToolResult manualmente
// en caso de error.

// CSPSyntaxCheckArgs son los parámetros del tool csp_syntax_check.
type CSPSyntaxCheckArgs struct {
	ModeloJSON map[string]interface{} `json:"modelo_json"`
	TimeoutS   float64                `json:"timeout_s,omitempty"`
}

// handleCSPSyntaxCheck implementa el tool csp_syntax_check.
func handleCSPSyntaxCheck(ctx context.Context, req *mcp.CallToolRequest, args CSPSyntaxCheckArgs) (*mcp.CallToolResult, any, error) {
	timeoutS := args.TimeoutS
	if timeoutS == 0 {
		timeoutS = DefaultTimeoutS
	}

	result, err := RunSyntaxCheck(args.ModeloJSON, timeoutS)
	if err != nil {
		// Si el error es PascalError, convertirlo a texto legible
		if pascalErr, ok := err.(*PascalError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.ToText())
		}
		if pascalErr, ok := err.(*PascalTimeoutError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.PascalError.ToText())
		}
		if pascalErr, ok := err.(*PascalJSONError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.PascalError.ToText())
		}
		return nil, nil, err
	}

	return nil, result, nil
}

// CSPPropagateArgs son los parámetros del tool csp_propagate.
type CSPPropagateArgs struct {
	ModeloJSON map[string]interface{} `json:"modelo_json"`
	Motor      string                 `json:"motor,omitempty"`
	TimeoutS   float64                `json:"timeout_s,omitempty"`
}

// handleCSPPropagate implementa el tool csp_propagate.
func handleCSPPropagate(ctx context.Context, req *mcp.CallToolRequest, args CSPPropagateArgs) (*mcp.CallToolResult, any, error) {
	motor := args.Motor
	if motor == "" {
		motor = "forward"
	}
	timeoutS := args.TimeoutS
	if timeoutS == 0 {
		timeoutS = DefaultTimeoutS
	}

	result, err := RunPropagate(args.ModeloJSON, motor, timeoutS)
	if err != nil {
		if pascalErr, ok := err.(*PascalError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.ToText())
		}
		if pascalErr, ok := err.(*PascalTimeoutError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.PascalError.ToText())
		}
		if pascalErr, ok := err.(*PascalJSONError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.PascalError.ToText())
		}
		return nil, nil, err
	}

	return nil, result, nil
}

// handleCSPListDomains implementa el tool csp_list_domains.
//
// Este tool no tiene parámetros, pero el SDK requiere un struct vacío.
func handleCSPListDomains(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	result, err := ListDomains()
	if err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

// CSPLoadModelArgs son los parámetros del tool csp_load_model.
type CSPLoadModelArgs struct {
	Dominio    string `json:"dominio"`
	Subdominio string `json:"subdominio"`
}

// handleCSPLoadModel implementa el tool csp_load_model.
func handleCSPLoadModel(ctx context.Context, req *mcp.CallToolRequest, args CSPLoadModelArgs) (*mcp.CallToolResult, any, error) {
	result, err := LoadModel(args.Dominio, args.Subdominio)
	if err != nil {
		if pascalErr, ok := err.(*PascalError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.ToText())
		}
		if pascalErr, ok := err.(*PascalJSONError); ok {
			return nil, nil, fmt.Errorf("%s", pascalErr.PascalError.ToText())
		}
		return nil, nil, err
	}
	return nil, result, nil
}

// ---------------------------------------------------------------------------
// Construcción del servidor MCP y el handler HTTP
// ---------------------------------------------------------------------------

// NewMCPServer crea y configura el servidor MCP con los 4 tools registrados.
func NewMCPServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, nil)

	// Registrar los 4 tools
	// mcp.AddTool toma: servidor, definición del tool (nombre + descripción), handler
	mcp.AddTool(server, &mcp.Tool{
		Name:        "csp_syntax_check",
		Description: "Valida sintaxis de un modelo CSP vía bin/SyntaxChecker.",
	}, handleCSPSyntaxCheck)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "csp_propagate",
		Description: "Pipeline JsonToGraph -> <motor>. motor in {forward, csp, fwd, bwd}.",
	}, handleCSPPropagate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "csp_list_domains",
		Description: "Lista carpetas data/Base/NN_*/ con el conteo de modelos *_csp.json.",
	}, handleCSPListDomains)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "csp_load_model",
		Description: "Lee data/Base/<dominio>/<subdominio>_csp.json y devuelve el JSON.",
	}, handleCSPLoadModel)

	return server
}

// NewHTTPHandler crea el handler HTTP completo que maneja:
//   - POST /mcp → endpoint MCP (streamable-http)
//   - GET /health → health check
//   - Middleware de protección anti DNS-rebinding en todas las rutas
//
// Retorna un http.Handler listo para pasar a http.ListenAndServe.
//
// Go idiom: http.ServeMux para routing
// ServeMux es el router HTTP estándar de Go. Es simple pero suficiente para este caso.
// Registramos rutas con mux.Handle() o mux.HandleFunc().
func NewHTTPHandler(mcpServer *mcp.Server) http.Handler {
	initAllowedHosts()

	// Crear el handler MCP usando el SDK
	// mcp.NewStreamableHTTPHandler crea un handler que maneja el protocolo MCP sobre HTTP/SSE
	// La función que pasamos como primer argumento es un "server selector": dado un http.Request,
	// devuelve el servidor MCP a usar. En nuestro caso siempre devolvemos el mismo servidor.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	// Crear el router (ServeMux)
	mux := http.NewServeMux()

	// Registrar rutas
	mux.Handle("/mcp", mcpHandler)          // POST /mcp → MCP endpoint
	mux.HandleFunc("/health", healthHandler) // GET /health → health check

	// Aplicar middleware de protección anti DNS-rebinding a todas las rutas
	return dnsRebindingProtection(mux)
}

// Serve arranca el servidor HTTP en host:port.
//
// Esta es la función principal que se llama desde main.go.
// Bloquea hasta que el servidor se detenga (ej: por error, o por señal de shutdown).
func Serve(host string, port int) error {
	mcpServer := NewMCPServer()
	handler := NewHTTPHandler(mcpServer)

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Fprintf(os.Stderr, "[mcp] MotorRender MCP server %s iniciando (streamable-http %s)\n", ServerVersion, addr)

	return http.ListenAndServe(addr, handler)
}
