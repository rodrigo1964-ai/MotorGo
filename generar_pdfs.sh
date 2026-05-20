#!/bin/bash
# generar_pdfs.sh - Generar PDFs de documentación con Pandoc

set -e

DOCS_DIR="docs"
PDF_DIR="$DOCS_DIR/pdf"
TMP_DIR="/tmp/parser_pdf_$$"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Crear directorios
mkdir -p "$PDF_DIR"
mkdir -p "$TMP_DIR"

# Limpiar al salir
trap "rm -rf $TMP_DIR" EXIT

# Verificar pandoc instalado
if ! command -v pandoc &> /dev/null; then
    echo -e "${RED}Error: pandoc no está instalado${NC}"
    echo ""
    echo "Instalar con:"
    echo "  Ubuntu/Debian: sudo apt install pandoc texlive-latex-base texlive-fonts-recommended texlive-latex-extra"
    echo "  Fedora/RHEL:   sudo dnf install pandoc texlive-scheme-basic"
    echo "  macOS:         brew install pandoc && brew install --cask basictex"
    exit 1
fi

# Función para convertir emojis y símbolos Unicode a texto compatible con LaTeX
remove_emojis() {
    local input="$1"
    local output="$2"

    sed 's/✅/**[OK]**/g;
         s/🔄/**[WIP]**/g;
         s/📚/**[DOCS]**/g;
         s/📁/**[DIR]**/g;
         s/🚀/**[START]**/g;
         s/📖/**[BOOK]**/g;
         s/📝/**[EDIT]**/g;
         s/🎨/**[STYLE]**/g;
         s/📧/**[MAIL]**/g;
         s/🔗/**[LINK]**/g;
         s/📊/**[STATS]**/g;
         s/⊆/subset/g;
         s/≤/<=/g;
         s/≥/>=/g;
         s/≠/!=/g;
         s/×/x/g;
         s/→/->/g;
         s/←/<-/g;
         s/↓/|/g;
         s/↑/|/g;
         s/ℝ/R/g;
         s/π/pi/g;
         s/∈/in/g;
         s/∅/empty/g;
         s/∪/union/g;
         s/∩/intersect/g;
         s/≈/~=/g;
         s/┌/+/g;
         s/┐/+/g;
         s/└/+/g;
         s/┘/+/g;
         s/├/+/g;
         s/┤/+/g;
         s/┬/+/g;
         s/┴/+/g;
         s/┼/+/g;
         s/─/-/g;
         s/│/|/g;
         s/║/|/g;
         s/═/=/g;
         s/╔/+/g;
         s/╗/+/g;
         s/╚/+/g;
         s/╝/+/g;
         s/╠/+/g;
         s/╣/+/g;
         s/╦/+/g;
         s/╩/+/g;
         s/╬/+/g' "$input" > "$output"
}

# Opciones comunes de pandoc
PANDOC_OPTS=(
    --pdf-engine=pdflatex
    --toc
    --toc-depth=3
    -V papersize=letter
    -V fontsize=11pt
    -V geometry:margin=2.5cm
    -V colorlinks=true
    -V linkcolor=blue
    -V urlcolor=blue
    -V toccolor=black
)

echo -e "${GREEN}=== Generando PDFs de Documentación ===${NC}"
echo ""

# 1. Generar 01_introduccion.pdf
if [ -f "$DOCS_DIR/01_introduccion.md" ]; then
    echo -e "${YELLOW}Generando 01_introduccion.pdf...${NC}"
    remove_emojis "$DOCS_DIR/01_introduccion.md" "$TMP_DIR/01_introduccion.md"
    pandoc "$TMP_DIR/01_introduccion.md" \
        -o "$PDF_DIR/01_introduccion.pdf" \
        "${PANDOC_OPTS[@]}"
    echo -e "${GREEN}✓ 01_introduccion.pdf generado${NC}"
else
    echo -e "${RED}✗ $DOCS_DIR/01_introduccion.md no encontrado${NC}"
fi

# 2. Generar estructura_proyecto.pdf
if [ -f "$DOCS_DIR/estructura_proyecto.md" ]; then
    echo -e "${YELLOW}Generando estructura_proyecto.pdf...${NC}"
    remove_emojis "$DOCS_DIR/estructura_proyecto.md" "$TMP_DIR/estructura_proyecto.md"
    pandoc "$TMP_DIR/estructura_proyecto.md" \
        -o "$PDF_DIR/estructura_proyecto.pdf" \
        "${PANDOC_OPTS[@]}" \
        -V title="Estructura del Proyecto CSP Parser" \
        -V author="Proyecto CSP Parser" \
        -V date="2026"
    echo -e "${GREEN}✓ estructura_proyecto.pdf generado${NC}"
else
    echo -e "${RED}✗ $DOCS_DIR/estructura_proyecto.md no encontrado${NC}"
fi

# 3. Generar manual completo (combinando todos los .md)
echo -e "${YELLOW}Generando CSP_Manual_Completo.pdf...${NC}"

MD_FILES=()
if [ -f "$DOCS_DIR/01_introduccion.md" ]; then
    remove_emojis "$DOCS_DIR/01_introduccion.md" "$TMP_DIR/01_introduccion.md"
    MD_FILES+=("$TMP_DIR/01_introduccion.md")
fi
# Agregar futuros documentos cuando existan:
# if [ -f "$DOCS_DIR/02_arquitectura_pipeline.md" ]; then
#     remove_emojis "$DOCS_DIR/02_arquitectura_pipeline.md" "$TMP_DIR/02_arquitectura_pipeline.md"
#     MD_FILES+=("$TMP_DIR/02_arquitectura_pipeline.md")
# fi

if [ ${#MD_FILES[@]} -gt 0 ]; then
    pandoc "${MD_FILES[@]}" \
        -o "$PDF_DIR/CSP_Manual_Completo.pdf" \
        "${PANDOC_OPTS[@]}" \
        -V title="CSP Parser and Evaluator - Manual Completo" \
        -V author="Proyecto CSP Parser" \
        -V date="2026"
    echo -e "${GREEN}✓ CSP_Manual_Completo.pdf generado${NC}"
else
    echo -e "${RED}✗ No se encontraron archivos .md para el manual completo${NC}"
fi

echo ""
echo -e "${GREEN}=== PDFs Generados ===${NC}"
echo ""

# Listar PDFs generados con tamaños
if [ -d "$PDF_DIR" ]; then
    ls -lh "$PDF_DIR"/*.pdf 2>/dev/null | awk '{printf "  %-40s %8s\n", $9, $5}' || echo "  (ninguno)"
else
    echo "  (ninguno)"
fi

echo ""
echo -e "${GREEN}Ubicación: $PDF_DIR/${NC}"
echo ""
echo "Para abrir:"
echo "  Linux:  xdg-open $PDF_DIR/CSP_Manual_Completo.pdf"
echo "  macOS:  open $PDF_DIR/CSP_Manual_Completo.pdf"
