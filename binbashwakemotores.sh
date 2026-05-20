#!/bin/bash
# wake-motores.sh
URLS=(
  "https://servicio-uno.onrender.com/health"
  "https://servicio-dos.onrender.com/health"
)
echo "Despertando servicios..."
for url in "${URLS[@]}"; do
  ( until curl -sf "$url" > /dev/null; do sleep 3; done; echo "OK: $url" ) &
done
wait
echo "Todos los servicios despiertos."
