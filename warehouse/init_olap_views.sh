#!/bin/bash
# init_olap_views.sh — Ejecuta todas las vistas OLAP del esquema estrella
# Uso: ./init_olap_views.sh [host] [port]
#   host: hive-server (default)
#   port: 10000 (default)
# Nota: Ejecutar DESPUÉS de init_hive.sh (las tablas base deben existir)

HIVE_HOST="${1:-hive-server}"
HIVE_PORT="${2:-10000}"
OLAP_DIR="$(dirname "$0")/olap"

echo "→ Creando vistas OLAP en Hive (${HIVE_HOST}:${HIVE_PORT})..."

FILES=(
  "v_ingresos_mensuales.hql"
  "v_ventas_por_categoria.hql"
  "v_reservas_por_ubicacion.hql"
  "v_tendencias_mensuales.hql"
  "v_horarios_pico.hql"
  "v_productividad_usuarios.hql"
)

for f in "${FILES[@]}"; do
  echo "  Ejecutando ${f}..."
  beeline -u "jdbc:hive2://${HIVE_HOST}:${HIVE_PORT}/default" \
          -n "" -p "" \
          -f "${OLAP_DIR}/${f}"
  if [ $? -ne 0 ]; then
    echo "  ERROR: Falló ${f}" >&2
    exit 1
  fi
done

echo "✓ Vistas OLAP creadas exitosamente."
