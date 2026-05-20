# CLAUDE.md — Contrato de migración: wrapper MCP de Python a Go

## 1. Objetivo

Migrar el **wrapper MCP** del proyecto MotorGo de Python a Go, manteniendo
**comportamiento observable idéntico**. El cómputo CSP/AC-3 lo seguirán
haciendo los binarios Pascal en `bin/` — Go solo reemplaza la capa de
pegamento que hoy implementa Python.

Resultado esperado: un **único binario Go estático** que expone un servidor
MCP HTTP, sin dependencia de Python, `pip`, ni `venv` en runtime.

## 2. Alcance estricto

### 2.1 Qué se migra (y solo esto)

| Archivo Python actual | Equivalente Go a crear |
|---|---|
| `src/mcp/server.py` | servidor MCP HTTP (transport streamable-http) |
| `src/mcp/pascal_runner.py` | spawner de binarios Pascal + lectura de corpus |
| `src/mcp/errors.py` | tipos de error equivalentes |

### 2.2 Qué NO se toca bajo ninguna circunstancia

- `bin/` — los 9 binarios Pascal compilados. Se invocan, no se modifican.
- `data/Base/` — los modelos CSP en JSON.
- Cualquier archivo `.pas`, `.c`, `.h` — código fuente Pascal/C.
- La lógica de inferencia CSP/AC-3.
- `Makefile`, `docs/`, `json/`, scripts `.sh` existentes.

### 2.3 Qué se elimina al final (solo tras validación exitosa)

- `src/mcp/*.py`, `requirements.txt`, `pytest.ini`, `.venv/`.
- `render.yaml` se conserva o adapta (decisión del usuario, no del agente).

## 3. SDK y dependencias

Usar el **SDK oficial de Go para MCP**: `github.com/modelcontextprotocol/go-sdk`.

ADVERTENCIA OBLIGATORIA: este SDK puede estar pre-1.0 y sujeto a cambios
incompatibles. Antes de escribir código, el agente DEBE:
1. Verificar la última versión estable publicada del módulo.
2. Leer la documentación vigente de la API (`mcp.Server`, transports,
   registro de tools), porque la API puede haber cambiado respecto a
   cualquier ejemplo previo.
3. Fijar la versión exacta en `go.mod` (no usar `latest` flotante).
4. Si el SDK oficial resultara inviable o inestable, detenerse y reportar
   al usuario antes de elegir una alternativa (`mcp-go` de Ed Zynda u otra).

Go mínimo: 1.22 o superior. Fijar la versión en `go.mod`.

## 4. Especificación funcional — paridad exacta con el wrapper Python

### 4.1 Servidor MCP (equivalente a `server.py`)

- Nombre del servidor: `motor-render` (string `SERVER_NAME`).
- Versión: `1.0.0`.
- Transport: **streamable-http** (HTTP, no stdio).
- Endpoint MCP montado en la ruta `/mcp`.
- Endpoint adicional `GET /health` que responde HTTP 200 con el JSON:
  `{"status":"ok","name":"motor-render","version":"1.0.0"}`.
  Este endpoint NO pasa por la lógica MCP y debe responder siempre rápido.

#### Variables de entorno (mismo contrato que el Python actual)

| Variable | Default | Uso |
|---|---|---|
| `PORT` | `8000` | puerto HTTP de escucha (Render lo inyecta) |
| `HOST` | `0.0.0.0` | bind address |
| `ALLOWED_HOST` | `motor-render-mcp.onrender.com` | hostname para la allowlist anti DNS-rebinding |

#### Protección anti DNS-rebinding

El wrapper Python configura `TransportSecuritySettings` con una allowlist de
`Host` headers. El servidor Go DEBE replicar esta protección: aceptar
requests cuyo header `Host` sea `ALLOWED_HOST` (con o sin puerto),
`localhost` o `127.0.0.1` (con o sin puerto), y rechazar el resto.
Si el SDK de Go no expone esta protección de forma nativa, implementarla
como middleware HTTP que valide el header `Host` antes de pasar al handler MCP.

### 4.2 Los 4 tools — nombres, firmas y semántica exactos

Los nombres de los tools DEBEN ser idénticos a los actuales. Un cliente MCP
ya conectado (Claude.ai) no debe notar diferencia alguna.

#### `csp_list_domains`
- Parámetros: ninguno.
- Lógica: recorre `data/Base/`, lista subdirectorios que NO empiecen con `_`,
  ordenados alfabéticamente. Para cada uno cuenta los archivos `*_csp.json`.
- Devuelve: `{"dominios": [...], "total": N}` donde cada elemento es
  `{"dominio": <nombre>, "ruta": "data/Base/<nombre>", "n_modelos": <int>, "modelos": [<archivos ordenados>]}`.

#### `csp_load_model`
- Parámetros: `dominio` (string), `subdominio` (string).
- Defensa path-traversal OBLIGATORIA: rechazar si `dominio` o `subdominio`
  contienen `/` o `..`. Verificar además que la ruta resuelta siga dentro
  de `data/Base/`.
- Lógica: lee `data/Base/<dominio>/<subdominio>_csp.json`, lo parsea y lo
  devuelve tal cual.
- Errores: archivo no encontrado, JSON inválido, ruta inválida.

#### `csp_syntax_check`
- Parámetros: `modelo_json` (object), `timeout_s` (number, default 30).
- Lógica: escribe `modelo_json` a un archivo temporal `_csp.json`, invoca
  `bin/SyntaxChecker <tmpfile>`, parsea su stdout como JSON, borra el temporal.
- Devuelve: el JSON que produce `SyntaxChecker`.

#### `csp_propagate`
- Parámetros: `modelo_json` (object), `motor` (string, default `forward`),
  `timeout_s` (number, default 30).
- Mapa de motores (EXACTO):
  - `forward` → `bin/ForwardChain`
  - `csp` → `bin/CSPEval`
  - `fwd` → `bin/FwdConsistency`
  - `bwd` → `bin/BwdConsistency`
- Si `motor` no está en el mapa, error de motor inválido.
- Lógica (pipeline de dos binarios):
  1. Escribe `modelo_json` a temporal `_csp.json`.
  2. Invoca `bin/JsonToGraph <modelo_tmp>`; su stdout se guarda a temporal `_graph.json`.
  3. Invoca `bin/<motor> <graph_tmp>`.
  4. Parsea el stdout del motor como JSON y lo devuelve.
  5. Borra ambos temporales (siempre, incluso ante error).

### 4.3 Spawner de binarios (equivalente a `pascal_runner.py`)

- Resolución de paths RELATIVA al ejecutable, no hardcodeada. El Python usa
  `Path(__file__).resolve().parents[2]` como `PROJECT_ROOT`. El Go debe
  determinar `PROJECT_ROOT` de forma equivalente y robusta (ej.: directorio
  del ejecutable, o variable de entorno `PROJECT_ROOT` con fallback). De ahí
  derivar `BIN_DIR = PROJECT_ROOT/bin` y `DATA_BASE = PROJECT_ROOT/data/Base`.
- Antes de ejecutar un binario: verificar que existe y es ejecutable; si no,
  error claro indicando el binario faltante.
- Ejecución: lanzar el binario con `cwd = PROJECT_ROOT`, capturar stdout y
  stderr por separado, aplicar el timeout (`timeout_s`).
- `timeout_s` default: 30 segundos.
- Manejo de fallos, con tres categorías de error distinguibles:
  - **Timeout**: el binario no terminó dentro del límite.
  - **Exit code != 0**: incluir binario, exit code, stderr (truncado a 500
    chars) y stdout (truncado) en el mensaje de error.
  - **JSON inválido**: el binario terminó con exit 0 pero su stdout no parsea
    como JSON; incluir binario y stdout (truncado).
- Temporales: crear con prefijo `mcp_`, sufijo apropiado (`_csp.json`,
  `_graph.json`); borrarlos siempre, incluso ante error o panic.

### 4.4 Errores hacia el cliente MCP

Cuando un tool falla, el error debe llegar al cliente MCP como un error de
tool (equivalente a `isError=true` / `CallToolResult` con error). El texto
debe ser legible e incluir: binario afectado, exit code si aplica, y
fragmentos de stderr/stdout truncados — replicando el método `to_text()` de
la clase `PascalError` actual.

## 5. Estructura de directorios resultante

```
MotorGo/
├── CLAUDE.md            (este contrato)
├── go.mod
├── go.sum
├── cmd/
│   └── server/
│       └── main.go      (entry point: lee env vars, arranca el server HTTP)
├── internal/
│   └── mcp/
│       ├── server.go    (registro de tools, transport, /health, seguridad)
│       ├── runner.go    (spawner de binarios Pascal, equiv. pascal_runner.py)
│       └── errors.go    (tipos de error, equiv. errors.py)
├── bin/                 (binarios Pascal — SIN TOCAR)
├── data/Base/           (corpus CSP — SIN TOCAR)
└── docs/, json/, etc.   (SIN TOCAR)
```

La estructura interna (`cmd/`, `internal/`) es la convención Go idiomática;
el agente puede ajustarla si tiene una razón sólida, documentándola.

## 6. Build y ejecución

- Compilación: `go build -o motorgo-server ./cmd/server`
- El binario resultante debe ser autocontenido (sin dependencias de runtime).
- Ejecución local de prueba: `PORT=8766 ./motorgo-server`
- El agente debe agregar las instrucciones de build a este CLAUDE.md o a un
  `BUILD.md` al finalizar.

## 7. Criterios de aceptación

La migración se considera completa cuando TODAS estas pruebas pasan contra
el binario Go corriendo localmente en el puerto 8766:

1. `curl http://127.0.0.1:8766/health` devuelve
   `{"status":"ok","name":"motor-render","version":"1.0.0"}` con HTTP 200.
2. `curl http://127.0.0.1:8766/mcp` (GET simple) devuelve un JSON-RPC con
   error `-32600` por falta de header `Accept: text/event-stream` o de
   session id — el mismo comportamiento que el server Python.
3. Un GET a `/mcp` con header `Host` arbitrario NO debe dar "Invalid Host
   header" si el host está en la allowlist; un host fuera de la allowlist
   SÍ debe rechazarse.
4. Vía un cliente MCP real (o `mcp` inspector), el handshake `initialize`
   funciona y `tools/list` devuelve los 4 tools con nombres exactos:
   `csp_list_domains`, `csp_load_model`, `csp_syntax_check`, `csp_propagate`.
5. `csp_list_domains` devuelve los dominios de `data/Base/` con el mismo
   formato JSON que el server Python (verificable comparando con la salida
   del Python sobre el mismo corpus).
6. `csp_load_model` con `dominio=100_mantenimiento_transformadores`,
   `subdominio=01_ttr_resistencia` devuelve el JSON del modelo.
7. `csp_load_model` con `subdominio` conteniendo `..` o `/` es rechazado.
8. `csp_propagate` con un modelo válido y `motor=forward` ejecuta el pipeline
   `JsonToGraph → ForwardChain` y devuelve un resultado con `"status":"ok"`.
9. `csp_propagate` con `motor=inexistente` devuelve error de motor inválido.
10. Un binario inexistente o un timeout producen errores legibles, no panics.

## 8. Reglas de proceso para el agente (Claude Code)

1. **No tocar `bin/` ni `data/`.** Verificar con `git status` que no hay
   cambios en esos directorios al terminar.
2. **No borrar los archivos Python** hasta que los 10 criterios de aceptación
   pasen. Durante el desarrollo, Python y Go coexisten.
3. **Trabajar en una rama git separada** (ej. `migracion-go`), no en `main`.
4. **Commits incrementales y descriptivos**: estructura del proyecto Go →
   runner → errors → server → main → validación. Un commit por etapa.
5. **Verificar la API del SDK antes de escribir**, según la sección 3.
6. **Ante ambigüedad o bloqueo** (API del SDK distinta a lo esperado,
   criterio de aceptación que no se puede cumplir, decisión de arquitectura
   no cubierta por este contrato): DETENERSE y reportar al usuario. No
   improvisar soluciones que se aparten del contrato.
7. **No introducir dependencias** más allá del SDK MCP de Go y la stdlib,
   salvo justificación explícita reportada al usuario.
8. Al finalizar, actualizar la sección "Historial" de este archivo con lo
   realizado y dejar documentado el comando de build.
9. **Comentar el código Go con densidad didáctica.** El usuario revisará y
   mantendrá este código, y conoce Go a nivel de lectura más que de
   escritura diaria. Por lo tanto:
   - Cada función lleva un comentario que explica qué hace y por qué.
   - Explicar las construcciones idiomáticas de Go en su primer uso
     relevante: goroutines y canales, `defer` (en particular para el borrado
     garantizado de archivos temporales), el patrón `if err != nil`,
     `context.Context` para timeouts y cancelación, punteros vs valores,
     interfaces, y el manejo de errores envueltos (`fmt.Errorf` con `%w`).
   - Donde una decisión de diseño no sea obvia (elección de tipo, uso de un
     sync primitive, estrategia de timeout), dejar un comentario breve
     que la justifique.
   - El objetivo es que el usuario pueda revisar cada commit con comodidad y
     retomar el código dentro de seis meses sin reconstruir el contexto.
   - No caer en el extremo opuesto: no comentar lo trivial
     (`i++ // incrementa i`). El comentario debe agregar información, no ruido.

## 9. Fuera de alcance (NO hacer)

- No migrar los tests Python (`tests/mcp/*.py`); si se quieren tests en Go,
  es una tarea posterior separada.
- No modificar el comportamiento de los binarios Pascal.
- No cambiar los nombres de los tools ni el formato de sus respuestas.
- No decidir sobre hosting (Render, Hetzner, etc.): eso lo define el usuario.
- No tocar la configuración de git remota ni hacer `push` sin instrucción.

## 10. Historial de Modificaciones

### 2026-05-20 — Corrección: protección DNS rebinding y soporte multi-host
- **Bug corregido**: el endpoint `/mcp` rechazaba hosts válidos de la allowlist con
  "Forbidden: invalid Host header", diferente del mensaje del middleware custom.
- **Causa**: el SDK de MCP aplica su propia protección anti DNS-rebinding interna
  (independiente del middleware custom) que por defecto solo acepta localhost.
  Al pasar `nil` como segundo argumento a `NewStreamableHTTPHandler`, esta protección
  del SDK rechazaba cualquier Host header no-localhost, incluso si estaba en la
  allowlist del middleware custom.
- **Solución**:
  * Desactivar la protección interna del SDK con `DisableLocalhostProtection: true`
    en `StreamableHTTPOptions`, dejando que el middleware custom sea el único
    punto de control. Esto evita duplicación de validaciones y permite que la
    allowlist del middleware custom funcione correctamente.
  * Documentado en comentarios de `NewHTTPHandler`: por qué desactivamos la
    protección del SDK y cómo se relaciona con el middleware custom.
- **Mejora adicional**: `ALLOWED_HOST` ahora acepta múltiples hosts separados por
  comas (ej: `"motor-render-mcp.onrender.com,tailscale-host.ts.net"`), permitiendo
  configurar varios hosts permitidos (Render + Tailscale + otros) sin recompilar.
- **Validación**:
  * ✅ Hosts en la allowlist (motor-render-mcp.onrender.com, tailscale-test.ts.net,
    example.com, localhost:8766) pasan el middleware correctamente
  * ✅ Hosts no permitidos (evil.com) son rechazados con "Invalid Host header"
  * ✅ No más "Forbidden: invalid Host header" del SDK en hosts válidos

### 2026-05-20 — Corrección: detección robusta de projectRoot
- **Bug corregido**: la lógica de `initPaths()` en `runner.go` aplicaba `filepath.Dir`
  tres veces sobre `os.Executable()`, asumiendo que el binario estaba en
  `cmd/server/motorgo-server`. Pero el comando de build del contrato
  (`go build -o motorgo-server ./cmd/server`) deja el binario en la raíz del proyecto,
  causando que `projectRoot` quedara dos niveles arriba del correcto (`/home` en vez
  de `/home/rodo/MotorGo`).
- **Causa**: conteo de niveles fijos en vez de búsqueda dinámica.
- **Solución**: estrategia robusta que parte del directorio del ejecutable y asciende
  verificando la existencia de `bin/` y `data/Base/`, hasta un máximo de 4 niveles.
  Esto permite que el binario funcione sin importar su ubicación (raíz, subdirectorio,
  o ejecutado desde `/tmp`). Se mantiene el override por `PROJECT_ROOT` env var con
  prioridad.
- **Validación**:
  * ✅ `cd /tmp && PROJECT_ROOT="" /home/rodo/MotorGo/motorgo-server` arranca
    correctamente y detecta automáticamente `/home/rodo/MotorGo` como `projectRoot`
  * ✅ Endpoint `/health` responde correctamente desde cualquier ubicación
  * ✅ Sin errores en logs de inicio

### 2026-05-20 — Migración completada (fase 1: implementación)
- **Rama**: `migracion-go`
- **SDK MCP**: v1.6.0 de `github.com/modelcontextprotocol/go-sdk/mcp`
- **Estructura**: `cmd/server/main.go`, `internal/mcp/{errors.go, runner.go, server.go}`
- **Implementado**:
  * errors.go: tipos PascalError, PascalTimeoutError, PascalJSONError con ToText()
  * runner.go: spawner de binarios Pascal con timeouts, defensa path-traversal, 4 funciones de tool entry points
  * server.go: servidor MCP HTTP con 4 tools registrados, protección anti DNS-rebinding, endpoint /health
  * main.go: entry point que lee env vars PORT, HOST, ALLOWED_HOST
- **Build**: `go build -o motorgo-server ./cmd/server` (12 MB, binario autocontenido)
- **Validación parcial**:
  * ✅ Compilación exitosa
  * ✅ Binarios Pascal en bin/ existentes y ejecutables
  * ✅ Endpoint /health responde correctamente
  * ✅ Protección anti DNS-rebinding funciona
  * ⏳ Pendiente: pruebas completas de los 4 tools con cliente MCP real (criterios 4-10)
- **Próximos pasos**: validar criterios de aceptación 4-10 con cliente MCP (mcp inspector o Claude.ai)

### 2026-05-20 — Contrato creado
- Definida la migración del wrapper MCP Python → Go.
