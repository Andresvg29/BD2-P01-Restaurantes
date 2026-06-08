#!/bin/bash
# init_hive.sh — Ejecuta todas las DDL del esquema estrella en orden
# Uso: ./init_hive.sh [host] [port]
#   host: hive-server (default)
#   port: 10000 (default)

HIVE_HOST="${1:-hive-server}"
HIVE_PORT="${2:-10000}"
DDL_DIR="$(dirname "$0")/ddl"

echo "→ Inicializando esquema estrella en Hive (${HIVE_HOST}:${HIVE_PORT})..."

# Orden: dimensiones primero, luego fact tables
FILES=(
  "dim_date.hql"
  "dim_user.hql"
  "dim_restaurant.hql"
  "dim_product.hql"
  "dim_order_status.hql"
  "dim_reservation_status.hql"
  "fact_orders.hql"
  "fact_order_details.hql"
  "fact_reservations.hql"
)

for f in "${FILES[@]}"; do
  echo "  Ejecutando ${f}..."
  beeline -u "jdbc:hive2://${HIVE_HOST}:${HIVE_PORT}/default" \
          -n "" -p "" \
          -f "${DDL_DIR}/${f}"
  if [ $? -ne 0 ]; then
    echo "  ERROR: Falló ${f}" >&2
    exit 1
  fi
done

echo "✓ Esquema estrella creado exitosamente."
