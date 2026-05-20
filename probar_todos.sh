#!/bin/bash
# Script para ejecutar todos los tests

echo "═══════════════════════════════════════════════════════════"
echo "  PROBANDO TODOS LOS TESTS - parser"
echo "═══════════════════════════════════════════════════════════"
echo ""

# Contadores
total=0
exitosos=0
fallidos=0

# Buscar archivos de test
if [ -d "tests" ]; then
  echo "Ejecutando tests en tests/..."
  for test_file in tests/test_*.sh; do
    if [ -f "$test_file" ]; then
      total=$((total + 1))
      echo "Ejecutando: $test_file"
      
      if bash "$test_file"; then
        exitosos=$((exitosos + 1))
        echo "  ✓ OK"
      else
        fallidos=$((fallidos + 1))
        echo "  ✗ FALLO"
      fi
    fi
  done
fi

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  RESUMEN"
echo "═══════════════════════════════════════════════════════════"
echo "Total:    $total"
echo "Exitosos: $exitosos"
echo "Fallidos: $fallidos"

if [ $fallidos -eq 0 ] && [ $total -gt 0 ]; then
  echo ""
  echo "✓ ¡TODOS LOS TESTS PASARON!"
  exit 0
elif [ $total -eq 0 ]; then
  echo ""
  echo "⚠ No se encontraron tests"
  exit 0
else
  echo ""
  echo "✗ Algunos tests fallaron"
  exit 1
fi
