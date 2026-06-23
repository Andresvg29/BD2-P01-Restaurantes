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
SPARK_OUTPUT="$PROJECT_DIR/spark/output"
PASS=0
FAIL=0

SPARK_MASTER="spark://spark-master:7077"
SCRIPTS="/opt/scripts"

pass() { echo "  [PASS] $1"; PASS=$((PASS + 1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL + 1)); }

echo "=== verify_spark.sh — Validación de Análisis Spark ==="
echo ""

# ── Check 1: Ejecución de los 3 análisis ──────────────────
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
  OUTPUT=$(docker exec spark-master spark-submit \
    --master "$SPARK_MASTER" \
    --conf "spark.hadoop.hive.metastore.uris=thrift://hive-metastore:9083" \
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
  FULL_PATH="$SPARK_OUTPUT/$out_path"
  if [ -d "$FULL_PATH" ]; then
    # Spark writes CSV as directory with part-*.csv files
    CSV_FILES=$(ls "$FULL_PATH"/part-*.csv 2>/dev/null || ls "$FULL_PATH"/*.csv 2>/dev/null || true)
    if [ -n "$CSV_FILES" ]; then
      FIRST_CSV=$(echo "$CSV_FILES" | head -1)
      LINE_COUNT=$(wc -l < "$FIRST_CSV")
      HEADER=$(head -1 "$FIRST_CSV")
      if [ "$LINE_COUNT" -gt 1 ] && [ -n "$HEADER" ]; then
        pass "Output $out_path existe ($((LINE_COUNT - 1)) filas, header: ${HEADER:0:60}...)"
      else
        fail "Output $out_path existe pero está vacío o sin datos"
      fi
    else
      fail "Output $out_path no contiene archivos CSV"
    fi
  else
    fail "Output $out_path no encontrado"
  fi
done

# ── Check 3: Validación cruzada Spark vs Hive ──────────────
echo ""
echo "--- Check 3: Validación cruzada (Spark vs Hive directo) ---"

HIVE_QUERY_INGRESOS="SELECT year, month, ROUND(SUM(total_amount), 2) AS ingresos
FROM fact_orders fo JOIN dim_date d ON fo.date_key = d.date_key
GROUP BY year, month ORDER BY year, month LIMIT 3"

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
