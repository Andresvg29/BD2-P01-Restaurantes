#!/bin/bash
# verify_spark.sh — Validación de Análisis Spark (7.3).
#
# Checks:
#   1. Los 3 scripts PySpark se ejecutan sin excepción
#   2. Outputs contienen datos no vacíos con formato esperado (CSV con cabeceras)
#   3. Validación manual: cruzar resultados Spark contra queries SQL directas en Hive
#
# Uso: bash scripts/verify_spark.sh
# Requiere: servicios Docker corriendo (spark-master, hive-server)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
PASS=0
FAIL=0

SPARK_MASTER="spark://spark-master:7077"
SCRIPTS="/opt/scripts"
SPARK_WORKER=$(docker ps --filter name=spark-worker-1 --format '{{.Names}}' 2>/dev/null || echo "bd2-p01-restaurantes-spark-worker-1")
SPARK_WORKER2=$(docker ps --filter name=spark-worker-2 --format '{{.Names}}' 2>/dev/null || echo "bd2-p01-restaurantes-spark-worker-2")

pass() { echo "  [PASS] $1"; PASS=$((PASS + 1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL + 1)); }

echo "=== verify_spark.sh — Validación de Análisis Spark ==="
echo ""

# ── Paso 0: ETL a modelo estrella (pre-requisito) ──────────
echo "--- Paso 0: ETL a modelo estrella (pre-requisito) ---"
set +e
ETL_OUTPUT=$(docker exec spark-master /opt/spark/bin/spark-submit \
  --master "$SPARK_MASTER" \
  "$SCRIPTS/etl_to_star.py" 2>&1)
ETL_RC=$?
set -e
if [ $ETL_RC -eq 0 ]; then
  pass "ETL a modelo estrella completado"
else
  fail "ETL a modelo estrella falló (exit code $ETL_RC)"
  echo "$ETL_OUTPUT" | tail -5 | sed 's/^/    /'
fi

# ── Check 1: Ejecución de los 3 análisis ──────────────────
echo ""
echo "--- Check 1: Ejecución de scripts PySpark ---"

ANALYSIS_JOBS=(
  "analisis_tendencias.py:Tendencias"
  "analisis_horarios.py:Horarios"
  "analisis_crecimiento.py:Crecimiento"
)

for entry in "${ANALYSIS_JOBS[@]}"; do
  JOB_FILE="${entry%%:*}"
  JOB_NAME="${entry##*:}"

  echo "  ▶ Ejecutando $JOB_NAME..."
  set +e
  OUTPUT=$(docker exec spark-master /opt/spark/bin/spark-submit \
    --master "$SPARK_MASTER" \
    "$SCRIPTS/$JOB_FILE" 2>&1)
  RC=$?
  set -e

  if [ $RC -eq 0 ]; then
    pass "$JOB_NAME completado sin errores"
  else
    fail "$JOB_NAME falló (exit code $RC)"
    echo "    Últimas líneas del output:"
    echo "$OUTPUT" | tail -5 | sed 's/^/    /'
  fi
done

# ── Check 2: Outputs existen y tienen formato ──────────────
echo ""
echo "--- Check 2: Outputs contienen datos no vacíos ---"

EXPECTED_OUTPUTS=(
  "tendencias/ingresos_mensuales"
  "tendencias/top_10_productos"
  "tendencias/categorias_trimestre"
  "horarios/horarios_pico"
  "horarios/ventanas_alta_demanda"
  "crecimiento/crecimiento_mensual"
  "crecimiento/resumen_crecimiento"
)

for out_path in "${EXPECTED_OUTPUTS[@]}"; do
  PART_CSV=$(docker exec "$SPARK_WORKER" sh -c "ls /opt/spark/output/$out_path/part-*.csv 2>/dev/null" | head -1)
  if [ -n "$PART_CSV" ]; then
    HEADER=$(docker exec "$SPARK_WORKER" sh -c "head -1 '$PART_CSV'" 2>/dev/null)
    LINE_COUNT=$(docker exec "$SPARK_WORKER" sh -c "wc -l < '$PART_CSV'" 2>/dev/null)
    if [ -n "$HEADER" ] && [ "$LINE_COUNT" -gt 1 ] 2>/dev/null; then
      pass "Output $out_path existe ($((LINE_COUNT - 1)) filas, header: ${HEADER:0:60}...)"
    else
      fail "Output $out_path existe pero está vacío o sin datos"
    fi
  else
    # Fallback: check worker-2
    PART_CSV=$(docker exec "$SPARK_WORKER2" sh -c "ls /opt/spark/output/$out_path/part-*.csv 2>/dev/null" | head -1)
    if [ -n "$PART_CSV" ]; then
      HEADER=$(docker exec "$SPARK_WORKER2" sh -c "head -1 '$PART_CSV'" 2>/dev/null)
      LINE_COUNT=$(docker exec "$SPARK_WORKER2" sh -c "wc -l < '$PART_CSV'" 2>/dev/null)
      if [ -n "$HEADER" ] && [ "$LINE_COUNT" -gt 1 ] 2>/dev/null; then
        pass "Output $out_path existe (spark-worker-2, $((LINE_COUNT - 1)) filas, header: ${HEADER:0:60}...)"
      else
        fail "Output $out_path no encontrado en spark-worker-1 ni spark-worker-2"
      fi
    else
      fail "Output $out_path no encontrado en spark-worker-1 ni spark-worker-2"
    fi
  fi
done

# ── Check 3: Validación cruzada Spark vs Hive ──────────────
echo ""
echo "--- Check 3: Validación cruzada (Spark vs Hive directo) ---"

HIVE_QUERY_INGRESOS="SELECT d.year, d.month, ROUND(SUM(fo.total_amount), 2) AS ingresos
FROM fact_orders fo JOIN dim_date d ON fo.date_key = d.date_key
GROUP BY d.year, d.month ORDER BY d.year, d.month LIMIT 3"

echo "  Consultando Hive directamente (top 3 meses)..."
set +e
HIVE_OUTPUT=$(docker exec hive-server beeline \
  -u "jdbc:hive2://hive-server:10000/default" \
  -e "$HIVE_QUERY_INGRESOS" \
  --silent=true --outputformat=csv2 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ] && [ -n "$HIVE_OUTPUT" ]; then
  pass "Hive directo responde — validación cruzada posible (ver resultados abajo)"
  echo "    Resultados Hive directo (primeros 3 meses):"
  echo "$HIVE_OUTPUT" | head -5 | sed 's/^/    /'
else
  fail "Hive directo no responde (exit code $RC)"
fi

echo ""
echo "=== RESULTADOS: $PASS PASS, $FAIL FAIL ==="
[ "$FAIL" -eq 0 ]
