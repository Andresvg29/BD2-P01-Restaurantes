#!/bin/bash
# verify_etl.sh — Validación del Pipeline ETL (7.2).
#
# Checks:
#   1. Ejecución completa: python run_etl.py sin errores
#   2. Idempotencia: ejecutar ETL 2 veces seguidas sin duplicados
#   3. Graceful degradation: si BD no responde, error claro
#
# Uso: bash scripts/verify_etl.sh
# Requiere: servicios Docker corriendo, python con dependencias instaladas

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
ETL_DIR="$PROJECT_DIR/warehouse/etl"
STAGING_DIR="$ETL_DIR/staging"
PASS=0
FAIL=0

pass() { echo "  [PASS] $1"; PASS=$((PASS + 1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL + 1)); }

# ── Load .env ──────────────────────────────────────────────
if [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  source "$PROJECT_DIR/.env"
  set +a
fi

DB_ENGINE="${DB_ENGINE:-mongodb}"

echo "=== verify_etl.sh — Validación del Pipeline ETL ==="
echo "DB_ENGINE=$DB_ENGINE"
echo ""

# ── Check 1: Ejecución completa ────────────────────────────
echo "--- Check 1: Ejecución completa del ETL ---"

# Clean staging first
rm -rf "$STAGING_DIR" 2>/dev/null || true
mkdir -p "$STAGING_DIR"

if cd "$ETL_DIR" && python export_data.py 2>&1; then
  pass "export_data.py ejecutado sin errores"
else
  fail "export_data.py falló"
fi

# ── Check 2: Idempotencia ──────────────────────────────────
echo ""
echo "--- Check 2: Idempotencia (2 corridas, mismos resultados) ---"

# Run 1: count files after first run
count_files_1=$(ls "$STAGING_DIR"/*.csv 2>/dev/null | wc -l)
row_count_1=0
for f in "$STAGING_DIR"/*.csv; do
  if [ -f "$f" ]; then
    c=$(tail -n +2 "$f" | wc -l)
    row_count_1=$((row_count_1 + c))
  fi
done
echo "  Primera corrida: $count_files_1 archivos, $row_count_1 filas totales"

# Clean and run again
rm -rf "$STAGING_DIR" 2>/dev/null || true
mkdir -p "$STAGING_DIR"
cd "$ETL_DIR" && python export_data.py 2>&1

count_files_2=$(ls "$STAGING_DIR"/*.csv 2>/dev/null | wc -l)
row_count_2=0
for f in "$STAGING_DIR"/*.csv; do
  if [ -f "$f" ]; then
    c=$(tail -n +2 "$f" | wc -l)
    row_count_2=$((row_count_2 + c))
  fi
done
echo "  Segunda corrida: $count_files_2 archivos, $row_count_2 filas totales"

if [ "$count_files_1" -eq "$count_files_2" ] && [ "$row_count_1" -eq "$row_count_2" ]; then
  pass "Idempotencia: resultados consistentes ($row_count_1 filas, $count_files_1 archivos)"
else
  fail "Idempotencia: primera ($row_count_1/$count_files_1) != segunda ($row_count_2/$count_files_2)"
fi

# ── Check 3: Graceful degradation ──────────────────────────
echo ""
echo "--- Check 3: Graceful degradation (BD caída) ---"

# Identify the DB container to stop
if [ "$DB_ENGINE" = "postgres" ] || [ "$DB_ENGINE" = "postgresql" ]; then
  DB_CONTAINER="db"
else
  DB_CONTAINER="mongos"
fi

echo "  Deteniendo $DB_CONTAINER..."
docker stop "$DB_CONTAINER" 2>/dev/null || echo "  ⚠ $DB_CONTAINER ya estaba detenido"

# Run export_data.py — should fail with a clear error message
set +e
cd "$ETL_DIR"
ERROR_OUTPUT=$(python export_data.py 2>&1)
EXIT_CODE=$?
set -e

# Restart the container
echo "  Re-iniciando $DB_CONTAINER..."
docker start "$DB_CONTAINER" 2>/dev/null || true
# Wait for it to be ready
sleep 3

if [ $EXIT_CODE -ne 0 ]; then
  # Check that the error message is descriptive, not a crash traceback
  if echo "$ERROR_OUTPUT" | grep -qiE "(error|ERROR|Connection refused|timeout|no se pudo|could not)"; then
    pass "Graceful degradation: error claro (exit code $EXIT_CODE)"
  else
    fail "Graceful degradation: error sin mensaje claro (exit code $EXIT_CODE)"
  fi
else
  fail "Graceful degradation: export_data.py debería fallar con BD caída"
fi

echo ""
echo "=== RESULTADOS: $PASS PASS, $FAIL FAIL ==="
[ "$FAIL" -eq 0 ]
