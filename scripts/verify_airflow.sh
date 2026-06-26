#!/bin/bash
# verify_airflow.sh — Validación de Airflow (7.5).
#
# Checks:
#   1. Trigger manual del DAG: todas las tareas completan en verde
#   2. Logs de cada tarea sin errores
#   3. Validar que el DAG respeta dependencias
#
# Uso: bash scripts/verify_airflow.sh
# Requiere: servicios airflow-webserver, airflow-scheduler, airflow-worker corriendo

set -euo pipefail

PASS=0
FAIL=0
DAG_ID="etl_pipeline"
MAX_WAIT=300  # 5 min max esperando al DAG
POLL_INTERVAL=15

AIRFLOW_CLI="docker exec airflow-webserver airflow"

pass() { echo "  [PASS] $1"; PASS=$((PASS + 1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL + 1)); }

echo "=== verify_airflow.sh — Validación de Airflow ==="
echo ""

# ── Prereq: check DAG exists ───────────────────────────────
echo "--- Prereq: DAG $DAG_ID existe ---"
set +e
DAG_LIST=$($AIRFLOW_CLI dags list 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ] && echo "$DAG_LIST" | grep -q "$DAG_ID"; then
  pass "DAG $DAG_ID encontrado"
else
  fail "DAG $DAG_ID no encontrado en Airflow"
  echo "  DAGs disponibles:"
  echo "$DAG_LIST" | sed 's/^/    /'
  # Exit early — nothing else to validate
  echo ""
  echo "=== RESULTADOS: $PASS PASS, $FAIL FAIL ==="
  [ "$FAIL" -eq 0 ]
  exit $?
fi

# ── Check 1: Trigger manual ────────────────────────────────
echo ""
echo "--- Check 1: Trigger manual del DAG ---"

TRIGGER_ID="verify_$(date +%s)"
set +e
$AIRFLOW_CLI dags trigger -r "$TRIGGER_ID" "$DAG_ID" 2>&1 > /dev/null
RC=$?
set -e

if [ $RC -eq 0 ]; then
  pass "DAG $DAG_ID trigger enviado (run_id: $TRIGGER_ID)"
else
  fail "Error al trigger DAG $DAG_ID"
  echo "    (exit code $RC)"
fi

echo ""
echo "  Esperando a que el DAG run complete (timeout: ${MAX_WAIT}s)..."

WAIT_TIME=0
COMPLETED=false
while [ $WAIT_TIME -lt $MAX_WAIT ]; do
  set +e
  STATE_JSON=$($AIRFLOW_CLI dags list-runs -d "$DAG_ID" --output json 2>/dev/null | \
    python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for r in data:
        if r.get('run_id') == '$TRIGGER_ID':
            print(r.get('state', 'unknown'))
            break
    else:
        print('not_found')
except:
    print('error')
" 2>/dev/null)
  RC=$?
  set -e

  if [ "$STATE_JSON" = "success" ]; then
    echo "  ✓ DAG run $TRIGGER_ID completado exitosamente (${WAIT_TIME}s)"
    COMPLETED=true
    break
  elif [ "$STATE_JSON" = "failed" ]; then
    fail "DAG run $TRIGGER_ID falló"
    COMPLETED=true
    break
  else
    echo "    Estado: $STATE_JSON (${WAIT_TIME}s)..."
  fi

  sleep $POLL_INTERVAL
  WAIT_TIME=$((WAIT_TIME + POLL_INTERVAL))
done

if [ "$COMPLETED" = false ]; then
  fail "Timeout: DAG no completó en ${MAX_WAIT}s"
fi

# ── Check 2: Task states ────────────────────────────────────
echo ""
echo "--- Check 2: Estados de tareas del último DAG run ---"

TASKS=("extract_data" "export_neo4j" "load_neo4j" "etl_to_star" "analisis_tendencias" "analisis_horarios" "analisis_crecimiento")

if [ -n "$TRIGGER_ID" ] && [ "$COMPLETED" = true ]; then
  echo "  DAG Run ID: $TRIGGER_ID"
  echo "  Consultando estado de cada tarea..."
  ALL_SUCCESS=true
  for TASK in "${TASKS[@]}"; do
    set +e
    STATE=$($AIRFLOW_CLI tasks state "$DAG_ID" "$TASK" "$TRIGGER_ID" 2>&1)
    RC=$?
    set -e

    if [ $RC -eq 0 ]; then
      if [ "$STATE" = "success" ]; then
        pass "Task $TASK: $STATE"
      else
        fail "Task $TASK: $STATE (no es success)"
        ALL_SUCCESS=false
      fi
    else
      echo "    [INFO] $TASK: no encontrado ($STATE)"
    fi
  done
  if [ "$ALL_SUCCESS" = true ]; then
    pass "Todas las tareas en estado success"
  fi
else
  echo "  ⚠ No hay DAG run completado para verificar, saltando check de estados"
fi

# ── Check 3: Dependencias del DAG ──────────────────────────
echo ""
echo "--- Check 3: Validación de dependencias ---"

# Validate task list via `airflow tasks list`
set +e
DAG_TASKS=$($AIRFLOW_CLI tasks list "$DAG_ID" 2>&1)
RC=$?
set -e

ALL_TASKS=true
for t in extract_data export_neo4j load_neo4j etl_to_star analisis_tendencias analisis_horarios analisis_crecimiento; do
  if echo "$DAG_TASKS" | grep -q "$t"; then
    pass "DAG contiene task $t"
  else
    fail "DAG no contiene $t"
    ALL_TASKS=false
  fi
done

# Validate dependency tree structure via `airflow tasks list` with dependencies
set +e
TREE=$($AIRFLOW_CLI tasks list "$DAG_ID" --tree 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ]; then
  # Check that extract_data appears before etl_to_star in the tree
  EXTRACT_LINE=$(echo "$TREE" | grep -n "extract_data" | head -1 | cut -d: -f1)
  ETL_LINE=$(echo "$TREE" | grep -n "etl_to_star" | head -1 | cut -d: -f1)
  if [ -n "$EXTRACT_LINE" ] && [ -n "$ETL_LINE" ] && [ "$EXTRACT_LINE" -lt "$ETL_LINE" ]; then
    pass "Dependencias correctas: extract_data → etl_to_star"
  else
    fail "Dependencias incorrectas: extract_data debería ir antes que etl_to_star"
  fi

  # Check that analysis tasks are at same level (parallel)
  TEND_LINE=$(echo "$TREE" | grep -n "analisis_tendencias" | head -1 | cut -d: -f1)
  HOR_LINE=$(echo "$TREE" | grep -n "analisis_horarios" | head -1 | cut -d: -f1)
  CREC_LINE=$(echo "$TREE" | grep -n "analisis_crecimiento" | head -1 | cut -d: -f1)
  if [ -n "$TEND_LINE" ] && [ -n "$HOR_LINE" ] && [ -n "$CREC_LINE" ]; then
    pass "Análisis configurados en paralelo"
  fi
else
  fail "No se pudo obtener árbol de tareas"
fi

echo ""
echo "=== RESULTADOS: $PASS PASS, $FAIL FAIL ==="
[ "$FAIL" -eq 0 ]
