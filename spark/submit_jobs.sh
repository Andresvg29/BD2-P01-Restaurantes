#!/bin/bash
# submit_jobs.sh — Envía los 4 jobs PySpark a spark-submit via docker exec.
# Uso: ./spark/submit_jobs.sh
#
# Ejecuta secuencialmente:
#   1. etl_to_star.py        (transformación a estrella → Hive)
# Luego en paralelo:
#   2. analisis_tendencias.py
#   3. analisis_horarios.py
#   4. analisis_crecimiento.py
#
# Requiere: docker, contenedor spark-master corriendo

set -euo pipefail

MASTER="spark://spark-master:7077"
SCRIPTS="/opt/scripts"

echo "=== Enviando jobs PySpark a spark://spark-master:7077 ==="
echo ""

# ── 1. ETL a estrella (bloqueante) ──────────────────────────────────────────
echo "▶ [1/4] ETL a modelo estrella..."
docker exec spark-master spark-submit --master "$MASTER" "$SCRIPTS/etl_to_star.py" 2>&1
echo "  ✓ ETL completado"
echo ""

# ── 2-4. Análisis (en paralelo) ─────────────────────────────────────────────
echo "▶ [2/4] Análisis de tendencias..."
docker exec spark-master spark-submit --master "$MASTER" "$SCRIPTS/analisis_tendencias.py" 2>&1 &
PID1=$!

echo "▶ [3/4] Análisis de horarios..."
docker exec spark-master spark-submit --master "$MASTER" "$SCRIPTS/analisis_horarios.py" 2>&1 &
PID2=$!

echo "▶ [4/4] Análisis de crecimiento..."
docker exec spark-master spark-submit --master "$MASTER" "$SCRIPTS/analisis_crecimiento.py" 2>&1 &
PID3=$!

echo "  Esperando análisis en paralelo..."
wait $PID1 $PID2 $PID3
echo "  ✓ Análisis completados"
echo ""

echo "=== Todos los jobs PySpark completados ==="
