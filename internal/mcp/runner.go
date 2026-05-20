package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultTimeoutS es el timeout por defecto para la ejecución de binarios Pascal.
// Cada binario debe terminar dentro de este límite o se levanta PascalTimeoutError.
const DefaultTimeoutS = 30.0

// Variables de path que se inicializan al primer uso (lazy initialization).
// Se calculan relativos al ejecutable para que el binario Go sea portable.
var (
	projectRoot string // Raíz del proyecto (donde está bin/, data/, etc.)
	binDir      string // Directorio bin/ con los binarios Pascal
	dataBase    string // Directorio data/Base/ con los modelos CSP
)

// initPaths inicializa las variables de path la primera vez que se invocan.
//
// Estrategia:
//  1. Buscar la variable de entorno PROJECT_ROOT (si existe, usarla)
//  2. Si no, usar el directorio del ejecutable actual
//  3. Validar que bin/ y data/Base/ existan en esa ruta
//
// En Go, os.Executable() devuelve la ruta absoluta del binario en ejecución.
// Subimos dos niveles desde cmd/server/main -> raíz del proyecto.
// Esta función usa sync.Once implícitamente via un flag para ejecutarse una sola vez.
func initPaths() error {
	if projectRoot != "" {
		return nil // Ya inicializado
	}

	// Opción 1: variable de entorno PROJECT_ROOT (útil para testing)
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		projectRoot = root
	} else {
		// Opción 2: calcular desde el ejecutable
		// os.Executable() devuelve la ruta completa al binario (ej: /path/to/motorgo-server)
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("no se pudo obtener la ruta del ejecutable: %w", err)
		}
		// Resolver symlinks si los hay (ej: si el binario es un symlink)
		exe, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return fmt.Errorf("no se pudo resolver symlinks del ejecutable: %w", err)
		}

		// Estrategia robusta: partir del directorio del ejecutable y ascender
		// hasta encontrar bin/ y data/Base/, con un máximo de 4 niveles.
		// Esto permite que el binario funcione tanto si está en la raíz del proyecto
		// (go build -o motorgo-server ./cmd/server) como si está en un subdirectorio.
		projectRoot = filepath.Dir(exe)
		found := false
		for i := 0; i < 4; i++ {
			binCandidate := filepath.Join(projectRoot, "bin")
			dataCandidate := filepath.Join(projectRoot, "data", "Base")

			// Verificar si ambos directorios existen
			binStat, binErr := os.Stat(binCandidate)
			dataStat, dataErr := os.Stat(dataCandidate)

			if binErr == nil && binStat.IsDir() && dataErr == nil && dataStat.IsDir() {
				// Encontrado: ambos directorios existen en este nivel
				found = true
				break
			}

			// No encontrado en este nivel: subir un nivel
			parent := filepath.Dir(projectRoot)
			if parent == projectRoot {
				// No podemos subir más (llegamos al root del filesystem)
				break
			}
			projectRoot = parent
		}

		if !found {
			return fmt.Errorf("no se pudo encontrar bin/ y data/Base/ ascendiendo desde %s", filepath.Dir(exe))
		}
	}

	binDir = filepath.Join(projectRoot, "bin")
	dataBase = filepath.Join(projectRoot, "data", "Base")

	// Validar que existan los directorios críticos
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		return fmt.Errorf("directorio bin/ no encontrado en %s", binDir)
	}
	if _, err := os.Stat(dataBase); os.IsNotExist(err) {
		return fmt.Errorf("directorio data/Base/ no encontrado en %s", dataBase)
	}

	return nil
}

// binPath devuelve la ruta completa a un binario Pascal en bin/.
//
// Verifica que el archivo exista y sea ejecutable; si no, levanta PascalError.
// Esta función debe llamarse antes de intentar ejecutar cualquier binario.
func binPath(name string) (string, error) {
	if err := initPaths(); err != nil {
		return "", err
	}

	path := filepath.Join(binDir, name)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", &PascalError{
			Message: fmt.Sprintf("Binario no encontrado: %s", path),
			Binary:  name,
		}
	}
	if err != nil {
		return "", &PascalError{
			Message: fmt.Sprintf("Error al verificar binario: %v", err),
			Binary:  name,
		}
	}

	// En Unix, verificar que tenga el bit de ejecución
	// info.Mode().Perm() & 0111 verifica si alguno de los bits de ejecución (owner/group/other) está activo
	if info.Mode().Perm()&0111 == 0 {
		return "", &PascalError{
			Message: fmt.Sprintf("Binario no es ejecutable: %s", path),
			Binary:  name,
		}
	}

	return path, nil
}

// run ejecuta un binario Pascal y devuelve su stdout como string.
//
// Parámetros:
//   - binary: nombre del binario (ej: "SyntaxChecker", "ForwardChain")
//   - args: argumentos de línea de comandos
//   - timeoutS: timeout en segundos (usar DefaultTimeoutS si no se especifica)
//   - stdinBytes: datos para enviar al stdin del proceso (opcional)
//
// Manejo de errores:
//   - Si el binario no existe o no es ejecutable: PascalError
//   - Si el proceso no termina dentro del timeout: PascalTimeoutError
//   - Si el exit code != 0: PascalError con exit_code, stdout, stderr
//
// Esta función captura stdout y stderr por separado para diagnóstico detallado.
// El cwd del proceso se fija en PROJECT_ROOT (importante: algunos binarios Pascal
// esperan encontrar data/ relativo al cwd).
//
// Go idiom: context.Context para timeouts y cancelación
// Creamos un context con timeout, y si el comando no termina a tiempo,
// Go mata el proceso automáticamente y devuelve context.DeadlineExceeded.
func run(binary string, args []string, timeoutS float64, stdinBytes []byte) (string, error) {
	exePath, err := binPath(binary)
	if err != nil {
		return "", err
	}

	if err := initPaths(); err != nil {
		return "", err
	}

	// Crear un context con timeout
	// context.WithTimeout devuelve un context que se cancela automáticamente
	// después del tiempo especificado, y una función cancel() que debemos llamar
	// cuando terminemos (para liberar recursos del context).
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS*float64(time.Second)))
	defer cancel() // defer: se ejecuta al salir de la función, sin importar cómo salga

	// exec.CommandContext crea un comando que será matado si el context se cancela
	cmd := exec.CommandContext(ctx, exePath, args...)
	cmd.Dir = projectRoot // cwd = PROJECT_ROOT
	cmd.Stdin = nil
	if stdinBytes != nil {
		cmd.Stdin = strings.NewReader(string(stdinBytes))
	}

	// Capturar stdout y stderr por separado
	// cmd.Output() solo captura stdout; aquí usamos CombinedOutput() no es suficiente
	// porque necesitamos stdout y stderr separados para diagnóstico.
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Ejecutar el comando
	err = cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Chequear si el error fue por timeout
	if ctx.Err() == context.DeadlineExceeded {
		return "", &PascalTimeoutError{
			PascalError: &PascalError{
				Message: fmt.Sprintf("Timeout %.1fs ejecutando %s", timeoutS, binary),
				Binary:  binary,
				Stdout:  stdoutStr,
				Stderr:  stderrStr,
			},
		}
	}

	// Chequear otros errores de ejecución (ej: binario no encontrado, permiso denegado)
	if err != nil {
		// Si es un ExitError, el proceso se ejecutó pero terminó con exit code != 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			return "", &PascalError{
				Message:  fmt.Sprintf("%s terminó con exit code %d", binary, exitCode),
				Binary:   binary,
				ExitCode: &exitCode,
				Stdout:   stdoutStr,
				Stderr:   stderrStr,
			}
		}
		// Otro error (ej: no se pudo iniciar el proceso)
		return "", &PascalError{
			Message: fmt.Sprintf("No se pudo ejecutar %s: %v", binary, err),
			Binary:  binary,
			Stdout:  stdoutStr,
			Stderr:  stderrStr,
		}
	}

	// Exit 0: éxito
	return stdoutStr, nil
}

// parseJSON parsea un string como JSON y devuelve el resultado.
//
// Si el string no es JSON válido, levanta PascalJSONError.
// Esta función se usa para parsear el stdout de los binarios Pascal,
// que siempre deben devolver JSON en caso de éxito.
//
// Tolera espacios y saltos de línea al inicio y al final del string.
func parseJSON(stdout string, binary string) (map[string]interface{}, error) {
	text := strings.TrimSpace(stdout)
	if text == "" {
		return nil, &PascalJSONError{
			PascalError: &PascalError{
				Message: fmt.Sprintf("%s devolvió stdout vacío", binary),
				Binary:  binary,
				Stdout:  stdout,
			},
		}
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, &PascalJSONError{
			PascalError: &PascalError{
				Message: fmt.Sprintf("stdout de %s no es JSON válido: %v", binary, err),
				Binary:  binary,
				Stdout:  stdout,
			},
		}
	}

	return result, nil
}

// writeTempJSON escribe un objeto Go (map o struct) como JSON a un archivo temporal.
//
// Los archivos temporales se crean con prefijo "mcp_" y el sufijo especificado.
// El caller es responsable de borrar el archivo cuando termine (típicamente con defer).
//
// Go idiom: defer para garantizar cleanup
// Patrón común en Go:
//   tmp := writeTempJSON(data, "_csp.json")
//   defer os.Remove(tmp)  // se ejecuta al salir de la función, incluso si hay panic
//   // ... usar tmp ...
func writeTempJSON(payload interface{}, suffix string) (string, error) {
	// os.CreateTemp crea un archivo temporal con un nombre único
	// El prefijo "mcp_" ayuda a identificar estos archivos en caso de debugging
	f, err := os.CreateTemp("", "mcp_*"+suffix)
	if err != nil {
		return "", fmt.Errorf("no se pudo crear archivo temporal: %w", err)
	}
	path := f.Name()

	// Serializar el payload a JSON (con indentación para legibilidad)
	// ensure_ascii=False en Python → no usar HTML escaping en Go
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("no se pudo serializar JSON: %w", err)
	}

	// Escribir al archivo y cerrarlo
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("no se pudo escribir JSON: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("no se pudo cerrar archivo temporal: %w", err)
	}

	return path, nil
}

// ---------------------------------------------------------------------------
// Tool entry points (funciones que implementan cada tool MCP)
// ---------------------------------------------------------------------------

// RunSyntaxCheck invoca bin/SyntaxChecker sobre un modelo CSP en memoria.
//
// Flujo:
//  1. Escribe modeloJSON a un archivo temporal _csp.json
//  2. Invoca bin/SyntaxChecker <tmpfile>
//  3. Parsea su stdout como JSON
//  4. Borra el archivo temporal (defer garantiza que esto ocurra incluso si hay error)
//
// Devuelve el JSON parseado o un error (PascalError, PascalTimeoutError, PascalJSONError).
func RunSyntaxCheck(modeloJSON map[string]interface{}, timeoutS float64) (map[string]interface{}, error) {
	tmp, err := writeTempJSON(modeloJSON, "_csp.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp) // defer: borrar el temporal al salir, pase lo que pase

	stdout, err := run("SyntaxChecker", []string{tmp}, timeoutS, nil)
	if err != nil {
		return nil, err
	}

	return parseJSON(stdout, "SyntaxChecker")
}

// motorMap mapea los nombres de motor (strings que llegan del cliente MCP)
// a los nombres de los binarios Pascal correspondientes.
//
// Esto replica el _MOTOR_MAP de pascal_runner.py.
var motorMap = map[string]string{
	"forward": "ForwardChain",
	"csp":     "CSPEval",
	"fwd":     "FwdConsistency",
	"bwd":     "BwdConsistency",
}

// RunPropagate ejecuta el pipeline JsonToGraph → <motor>.
//
// Este es el tool más complejo: requiere dos invocaciones secuenciales de binarios
// y dos archivos temporales (modelo_tmp y graph_tmp).
//
// Flujo:
//  1. Validar que motor esté en motorMap
//  2. Escribir modeloJSON a archivo temporal _csp.json
//  3. Invocar bin/JsonToGraph <modelo_tmp> → capturar stdout
//  4. Escribir stdout de JsonToGraph a archivo temporal _graph.json
//  5. Invocar bin/<motor> <graph_tmp> → capturar stdout
//  6. Parsear stdout del motor como JSON
//  7. Borrar ambos archivos temporales (defer garantiza cleanup)
//
// Parámetros:
//   - modeloJSON: el modelo CSP a propagar
//   - motor: "forward" | "csp" | "fwd" | "bwd"
//   - timeoutS: timeout para CADA binario (no para el pipeline completo)
//
// Go idiom: múltiples defer se ejecutan en orden LIFO (last in, first out)
// Si hacemos:
//   defer os.Remove(a)
//   defer os.Remove(b)
// Primero se borra b, luego a (al salir de la función).
func RunPropagate(modeloJSON map[string]interface{}, motor string, timeoutS float64) (map[string]interface{}, error) {
	// Validar motor
	motorBinary, ok := motorMap[motor]
	if !ok {
		// Construir lista de motores válidos para el mensaje de error
		validMotors := make([]string, 0, len(motorMap))
		for k := range motorMap {
			validMotors = append(validMotors, k)
		}
		sort.Strings(validMotors) // ordenar alfabéticamente
		return nil, &PascalError{
			Message: fmt.Sprintf("motor inválido: %q (esperado: %v)", motor, validMotors),
			Binary:  "pipeline",
		}
	}

	// Paso 1: escribir modelo_tmp
	modeloTmp, err := writeTempJSON(modeloJSON, "_csp.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(modeloTmp) // defer 1: borrar modelo_tmp al salir

	// Paso 2: invocar JsonToGraph
	graphOut, err := run("JsonToGraph", []string{modeloTmp}, timeoutS, nil)
	if err != nil {
		return nil, err
	}

	// Paso 3: escribir graph_tmp
	// Nota: graphOut ya es JSON (string), no necesitamos parsearlo y re-serializarlo,
	// simplemente lo escribimos tal cual al archivo temporal
	f, err := os.CreateTemp("", "mcp_*_graph.json")
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear graph_tmp: %w", err)
	}
	graphTmp := f.Name()
	if _, err := f.WriteString(graphOut); err != nil {
		f.Close()
		os.Remove(graphTmp)
		return nil, fmt.Errorf("no se pudo escribir graph_tmp: %w", err)
	}
	f.Close()
	defer os.Remove(graphTmp) // defer 2: borrar graph_tmp al salir (se ejecuta antes que defer 1)

	// Paso 4: invocar el motor
	resultOut, err := run(motorBinary, []string{graphTmp}, timeoutS, nil)
	if err != nil {
		return nil, err
	}

	// Paso 5: parsear resultado
	return parseJSON(resultOut, motorBinary)
}

// ListDomains lista carpetas en data/Base/ que NO empiecen con "_",
// y para cada una cuenta los archivos *_csp.json.
//
// Devuelve:
//   {
//     "dominios": [
//       {
//         "dominio": "100_mantenimiento_transformadores",
//         "ruta": "data/Base/100_mantenimiento_transformadores",
//         "n_modelos": 5,
//         "modelos": ["01_ttr_resistencia_csp.json", ...]
//       },
//       ...
//     ],
//     "total": N
//   }
//
// Los dominios se devuelven ordenados alfabéticamente.
func ListDomains() (map[string]interface{}, error) {
	if err := initPaths(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dataBase)
	if err != nil {
		// Si data/Base no existe o no se puede leer, devolver lista vacía
		return map[string]interface{}{
			"dominios": []interface{}{},
			"total":    0,
		}, nil
	}

	var dominios []map[string]interface{}
	for _, entry := range entries {
		// Filtrar: solo directorios, y que NO empiecen con "_"
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") {
			continue // logs, reportes, etc.
		}

		// Listar los archivos *_csp.json en este dominio
		domainPath := filepath.Join(dataBase, entry.Name())
		cspFiles, err := filepath.Glob(filepath.Join(domainPath, "*_csp.json"))
		if err != nil {
			continue // ignorar dominios con errores de glob
		}

		// Extraer solo los nombres de archivo (sin path completo) y ordenar
		var modelos []string
		for _, f := range cspFiles {
			modelos = append(modelos, filepath.Base(f))
		}
		sort.Strings(modelos)

		// Construir la ruta relativa al PROJECT_ROOT (para el JSON de respuesta)
		relPath, _ := filepath.Rel(projectRoot, domainPath)

		dominios = append(dominios, map[string]interface{}{
			"dominio":   entry.Name(),
			"ruta":      relPath,
			"n_modelos": len(modelos),
			"modelos":   modelos,
		})
	}

	// Ordenar dominios alfabéticamente por nombre
	sort.Slice(dominios, func(i, j int) bool {
		return dominios[i]["dominio"].(string) < dominios[j]["dominio"].(string)
	})

	return map[string]interface{}{
		"dominios": dominios,
		"total":    len(dominios),
	}, nil
}

// LoadModel lee data/Base/<dominio>/<subdominio>_csp.json y devuelve el JSON parseado.
//
// Defensa anti path-traversal OBLIGATORIA:
//   - Rechazar si dominio o subdominio contienen "/" o ".."
//   - Verificar que la ruta resuelta (después de resolver symlinks) siga dentro de data/Base/
//
// Esto evita que un cliente malicioso lea archivos fuera de data/Base/ usando
// payloads como dominio="../../../etc", subdominio="passwd".
//
// Go idiom: filepath.Rel para verificar que una ruta esté dentro de otra
// filepath.Rel(base, target) devuelve el path relativo de target respecto a base.
// Si target está fuera de base, el resultado empieza con "../".
func LoadModel(dominio, subdominio string) (map[string]interface{}, error) {
	if err := initPaths(); err != nil {
		return nil, err
	}

	// Defensa 1: rechazar "/" y ".." en los argumentos
	if strings.Contains(dominio, "/") || strings.Contains(dominio, "..") {
		return nil, &PascalError{
			Message: "dominio no puede contener '/' ni '..'",
			Binary:  "load_model",
		}
	}
	if strings.Contains(subdominio, "/") || strings.Contains(subdominio, "..") {
		return nil, &PascalError{
			Message: "subdominio no puede contener '/' ni '..'",
			Binary:  "load_model",
		}
	}

	// Construir el path candidato
	candidate := filepath.Join(dataBase, dominio, subdominio+"_csp.json")

	// Defensa 2: resolver symlinks y verificar que esté dentro de data/Base/
	// filepath.EvalSymlinks resuelve todos los symlinks en el path y devuelve el path absoluto real
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// EvalSymlinks falla si el archivo no existe, o si hay un symlink roto
		return nil, &PascalError{
			Message: fmt.Sprintf("ruta inválida: %v", err),
			Binary:  "load_model",
		}
	}

	// Verificar que resolved esté dentro de dataBase
	// filepath.Rel devuelve el path relativo; si empieza con "..", está fuera
	relPath, err := filepath.Rel(dataBase, resolved)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return nil, &PascalError{
			Message: fmt.Sprintf("ruta fuera de data/Base/: %s", candidate),
			Binary:  "load_model",
		}
	}

	// Verificar que el archivo exista y sea un archivo regular (no un directorio)
	info, err := os.Stat(resolved)
	if os.IsNotExist(err) {
		relToProject, _ := filepath.Rel(projectRoot, candidate)
		return nil, &PascalError{
			Message: fmt.Sprintf("archivo no encontrado: %s", relToProject),
			Binary:  "load_model",
		}
	}
	if err != nil {
		return nil, &PascalError{
			Message: fmt.Sprintf("error al acceder al archivo: %v", err),
			Binary:  "load_model",
		}
	}
	if info.IsDir() {
		return nil, &PascalError{
			Message: fmt.Sprintf("la ruta es un directorio, no un archivo: %s", candidate),
			Binary:  "load_model",
		}
	}

	// Leer y parsear el archivo JSON
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, &PascalError{
			Message: fmt.Sprintf("no se pudo leer el archivo: %v", err),
			Binary:  "load_model",
		}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &PascalJSONError{
			PascalError: &PascalError{
				Message: fmt.Sprintf("archivo no es JSON válido: %v", err),
				Binary:  "load_model",
				Stdout:  string(data[:min(len(data), 500)]), // primeros 500 bytes
			},
		}
	}

	return result, nil
}

// min devuelve el mínimo de dos enteros (utility function, Go no tiene min builtin para int)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
