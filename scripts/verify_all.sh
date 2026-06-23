#!/bin/bash
# verify_all.sh — Orquestador maestro de validaciones OLAP (7.6).
#
# Ejecuta secuencialmente todas las validaciones y reporta PASS/FAIL.
# Uso: bash scripts/verify_all.sh
#
# Requiere: todos los servicios Docker corriendo
#   docker-compose up -d

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASS=0
FAIL=0
RESULTS=()

echo "╔══════════════════════════════════════════════════════════╗"
echo "║        OLAP Verification Suite — BD2-P01-Restaurantes   ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# Load .env
if [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  source "$PROJECT_DIR/.env"
  set +a
  echo "  DB_ENGINE=${DB_ENGINE:-mongodb}"
  echo ""
fi

# ── Lista de validaciones ───────────────────────────────────
VERIFICATIONS=(
  "verify_warehouse.py:verify_warehouse.py:7.1 — Validación de Datos en Warehouse"
  "verify_etl.sh:verify_etl.sh:7.2 — Validación del Pipeline ETL"
  "verify_spark.sh:verify_spark.sh:7.3 — Validación de Análisis Spark"
  "verify_neo4j.sh:verify_neo4j.sh:7.4 — Validación de Neo4j"
  "verify_airflow.sh:verify_airflow.sh:7.5 — Validación de Airflow"
)

# ── Pre-flight check ────────────────────────────────────────
echo "--- Pre-flight: verificando Docker ---"
if ! docker info >/dev/null 2>&1; then
  echo -e "${RED}ERROR: Docker no está disponible${NC}"
  exit 1
fi
echo "  ✓ Docker OK"
echo ""

# ── Ejecutar validaciones ──────────────────────────────────
for entry in "${VERIFICATIONS[@]}"; do
  NAME="${entry%%:*}"
  SCRIPT=$(echo "$entry" | cut -d: -f2)
  DESC=$(echo "$entry" | cut -d: -f3-)

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  $DESC"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""

  if [[ "$SCRIPT" == *.py ]]; then
    (cd "$PROJECT_DIR" && python "$SCRIPT_DIR/$SCRIPT")
    RC=$?
  else
    bash "$SCRIPT_DIR/$SCRIPT"
    RC=$?
  fi

  if [ $RC -eq 0 ]; then
    RESULTS+=("$NAME:PASS")
    PASS=$((PASS + 1))
    echo -e "${GREEN}  ✓ $NAME: PASS${NC}"
  else
    RESULTS+=("$NAME:FAIL")
    FAIL=$((FAIL + 1))
    echo -e "${RED}  ✗ $NAME: FAIL${NC}"
  fi
  echo ""
done

# ── Resumen final ───────────────────────────────────────────
echo "╔══════════════════════════════════════════════════════════╗"
echo "║                    RESUMEN FINAL                        ║"
echo "╠══════════════════════════════════════════════════════════╣"
for result in "${RESULTS[@]}"; do
  name="${result%%:*}"
  status="${result##*:}"
  if [ "$status" = "PASS" ]; then
    echo -e "║  ${GREEN}✓${NC} $(printf '%-50s' "$name")${GREEN}PASS${NC}  ║"
  else
    echo -e "║  ${RED}✗${NC} $(printf '%-50s' "$name")${RED}FAIL${NC}  ║"
  fi
done
echo "╠══════════════════════════════════════════════════════════╣"
TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  echo -e "║  ${GREEN}✓ $PASS/$TOTAL validaciones pasaron${NC}                ║"
else
  echo -e "║  ${RED}✗ $PASS/$TOTAL pasaron, $FAIL fallaron${NC}               ║"
fi
echo "╚══════════════════════════════════════════════════════════╝"

exit $FAIL
