#!/bin/bash
# csp-pipeline.sh — procesa uno o varios JSON de sistema
#
# Uso:
#   ./csp-pipeline.sh sistema.json
#   ./csp-pipeline.sh sistemas/*.json
#   ./csp-pipeline.sh --path ./lib:./obj sistema.json
#   cat lista.txt | xargs ./csp-pipeline.sh
#
# Opciones:
#   --path DIR1:DIR2:...   path de búsqueda para FunctionChecker
#                          (default: . ./lib ./obj /usr/local/lib/csp)

BINDIR="$(dirname "$0")"
TMPGRAPH=$(mktemp /tmp/csp_graph_XXXXXX.json)
TMPFC=$(mktemp /tmp/csp_fc_XXXXXX.json)
trap "rm -f $TMPGRAPH $TMPFC" EXIT

FC_PATH=""

# ── parsear opciones globales ────────────────────────────────────────────────
ARGS=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --path)
            FC_PATH="$2"
            shift 2
            ;;
        --path=*)
            FC_PATH="${1#--path=}"
            shift
            ;;
        *)
            ARGS+=("$1")
            shift
            ;;
    esac
done

if [ ${#ARGS[@]} -eq 0 ]; then
    echo "Uso: $0 [--path dir1:dir2:...] sistema.json [...]" >&2
    exit 2
fi

# ── procesar cada archivo ────────────────────────────────────────────────────
for f in "${ARGS[@]}"; do
    echo "=== $f ==="

    # Etapa 1: validar sintaxis
    "$BINDIR/SyntaxChecker" "$f" > /tmp/sc_out.json 2>&1
    SC_STATUS=$(python3 -c "import json,sys; d=json.load(open('/tmp/sc_out.json')); print(d['status'])" 2>/dev/null || echo "error")
    if [ "$SC_STATUS" != "ok" ]; then
        echo "  [SyntaxChecker] FAIL"
        cat /tmp/sc_out.json
        echo ""
        continue
    fi
    echo "  [SyntaxChecker] ok"

    # Etapa 2: construir grafo
    "$BINDIR/JsonToGraph" "$f" > "$TMPGRAPH" 2>&1
    if [ $? -ne 0 ]; then
        echo "  [JsonToGraph] ERROR"
        cat "$TMPGRAPH"
        echo ""
        continue
    fi
    echo "  [JsonToGraph] ok"

    # Etapa 3: verificar objetos de funciones user-defined
    FC_ARGS=("$TMPGRAPH")
    if [ -n "$FC_PATH" ]; then
        FC_ARGS+=(--path "$FC_PATH")
    fi
    "$BINDIR/FunctionChecker" "${FC_ARGS[@]}" > "$TMPFC" 2>&1
    FC_EXIT=$?
    if [ $FC_EXIT -eq 2 ]; then
        echo "  [FunctionChecker] ERROR FATAL"
        cat "$TMPFC"
        echo ""
        continue
    elif [ $FC_EXIT -eq 1 ]; then
        echo "  [FunctionChecker] FAIL — objetos faltantes"
        cat "$TMPFC"
        echo ""
        continue
    fi
    FC_CHECKED=$(python3 -c "import json,sys; d=json.load(open('$TMPFC')); print(d['checked'])" 2>/dev/null || echo "?")
    echo "  [FunctionChecker] ok (checked: $FC_CHECKED)"

    # Etapa 4: evaluar CSP
    "$BINDIR/CSPEval" "$TMPGRAPH"
    echo ""
done
