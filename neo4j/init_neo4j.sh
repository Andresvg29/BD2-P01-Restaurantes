#!/bin/bash
# init_neo4j.sh — Orquestador de importacion de datos a Neo4j.
#
# 1. Ejecuta import_data.py para exportar datos desde MongoDB/PostgreSQL a CSVs
# 2. Copia CSVs a staging/ (montado en /var/lib/neo4j/import en el contenedor)
# 3. Ejecuta load_schema.cypher via cypher-shell en el contenedor neo4j
#
# Uso: bash neo4j/init_neo4j.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
STAGING_DIR="$SCRIPT_DIR/staging"
NEO4J_CONTAINER="neo4j"
DB_ENGINE="${DB_ENGINE:-mongodb}"

echo "=== Neo4j Init ==="
echo "DB_ENGINE=$DB_ENGINE"
echo "Staging dir: $STAGING_DIR"

# 1. Exportar datos a CSV
echo ""
echo ">>> Exportando datos..."
cd "$SCRIPT_DIR"
python import_data.py
echo "Exportacion completada."

# 2. Verificar que los CSVs existen
echo ""
echo ">>> Verificando archivos CSV..."
REQUIRED_FILES=(
  "users.csv" "products.csv" "restaurants.csv"
  "orders.csv" "reservations.csv"
  "placed.csv" "includes.csv" "at_order.csv"
  "made_reservation.csv" "at_reservation.csv"
  "locations.csv" "located_at.csv" "roads.csv"
)
for f in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$STAGING_DIR/$f" ]; then
    echo "ERROR: $f no encontrado en $STAGING_DIR" >&2
    exit 1
  fi
  echo "  OK $f"
done

# 3. Esperar a que Neo4j esté listo
echo ""
echo ">>> Esperando a que Neo4j esté listo..."
for i in $(seq 1 30); do
  if docker exec "$NEO4J_CONTAINER" cypher-shell -u neo4j -p password "RETURN 1" >/dev/null 2>&1; then
    echo "Neo4j listo (intento $i)"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: Neo4j no respondio tras 30 intentos" >&2
    exit 1
  fi
  sleep 2
done

# 4. Ejecutar schema
echo ""
echo ">>> Cargando schema..."
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u neo4j -p password < "$SCRIPT_DIR/load_schema.cypher"
echo "Schema cargado exitosamente."

# 5. Verificacion rapida
echo ""
echo ">>> Verificando datos..."
docker exec "$NEO4J_CONTAINER" cypher-shell -u neo4j -p password "
  MATCH (u:User) RETURN count(u) AS usuarios;
  MATCH (p:Product) RETURN count(p) AS productos;
  MATCH (r:Restaurant) RETURN count(r) AS restaurantes;
  MATCH (o:Order) RETURN count(o) AS pedidos;
  MATCH (res:Reservation) RETURN count(res) AS reservas;
"

echo ""
echo "=== Init Neo4j completado ==="
