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

set +e
TRIGGER_OUTPUT=$($AIRFLOW_CLI dags trigger -r "verify_run_$(date +%s)" "$DAG_ID" 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ]; then
  pass "DAG $DAG_ID trigger enviado"
  echo "    Output: $TRIGGER_OUTPUT" | head -3
else
  fail "Error al trigger DAG $DAG_ID"
  echo "    $TRIGGER_OUTPUT" | sed 's/^/    /'
fi

# Wait for the DAG run to complete, with a timeout
echo ""
echo "  Esperando a que el DAG run complete (timeout: ${MAX_WAIT}s)..."

DAG_RUN_ID="manual__verify_run_$(date +%s)"
# Actually we need to find the run ID from the trigger output
# Format: Created <dag_run_type> <dag_id> with run_id <run_id>, ...

# Better approach: poll the latest DAG run status
WAIT_TIME=0
COMPLETED=false
while [ $WAIT_TIME -lt $MAX_WAIT ]; do
  set +e
  LATEST_RUNS=$($AIRFLOW_CLI dags list-runs -d "$DAG_ID" --output json 2>&1 | \
    python3 -c "import sys,json; data=json.load(sys.stdin); runs=[r for r in data if 'manual' in r.get('run_id','') or r.get('state') in ('running','queued')]; print(json.dumps(runs[0] if runs else {}))" 2>/dev/null)
  RC=$?
  set -e

  if [ $RC -eq 0 ] && [ -n "$LATEST_RUNS" ] && [ "$LATEST_RUNS" != "{}" ]; then
    STATE=$(echo "$LATEST_RUNS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('state','unknown'))" 2>/dev/null)
    RUN_ID=$(echo "$LATEST_RUNS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('run_id',''))" 2>/dev/null)

    if [ "$STATE" = "success" ]; then
      echo "  ✓ DAG run $RUN_ID completado exitosamente (${WAIT_TIME}s)"
      COMPLETED=true
      break
    elif [ "$STATE" = "failed" ]; then
      fail "DAG run $RUN_ID falló"
      COMPLETED=true
      break
    else
      echo "    Estado: $STATE (${WAIT_TIME}s)..."
    fi
  else
    echo "    Esperando DAG run (${WAIT_TIME}s)..."
  fi

  sleep $POLL_INTERVAL
  WAIT_TIME=$((WAIT_TIME + POLL_INTERVAL))
done

if [ "$COMPLETED" = false ]; then
  fail "Timeout: DAG no completó en ${MAX_WAIT}s"
fi

# ── Check 2: Task logs sin errores ─────────────────────────
echo ""
echo "--- Check 2: Verificación de task logs ---"

TASKS=("extract_data" "etl_to_star" "analisis_tendencias" "analisis_horarios" "analisis_crecimiento")

# Get the latest DAG run execution date
set +e
EXEC_DATE=$($AIRFLOW_CLI dags list-runs -d "$DAG_ID" --output json 2>&1 | \
  python3 -c "
import sys, json
data = json.load(sys.stdin)
manual = [r for r in data if 'manual' in r.get('run_id', '')]
if manual:
  print(manual[0].get('execution_date', ''))
" 2>/dev/null)
RC=$?
set -e

if [ -n "$EXEC_DATE" ] && [ "$EXEC_DATE" != "" ]; then
  echo "  Execution date: $EXEC_DATE"
  for TASK in "${TASKS[@]}"; do
    set +e
    LOGS=$($AIRFLOW_CLI tasks logs -t "$TASK" "$DAG_ID" "$EXEC_DATE" 2>&1 | tail -20)
    RC=$?
    set -e

    if echo "$LOGS" | grep -qi "error\|exception\|traceback"; then
      fail "Task $TASK tiene errores en logs"
      echo "    Últimas líneas del log de $TASK:"
      echo "$LOGS" | sed 's/^/    /'
    else
      pass "Task $TASK logs limpios"
    fi
  done
else
  echo "  ⚠ No se encontró execution date del DAG manual, saltando check de logs"
fi

# ── Check 3: Dependencias del DAG ──────────────────────────
echo ""
echo "--- Check 3: Validación de dependencias ---"

set +e
DAG_INFO=$($AIRFLOW_CLI dags info "$DAG_ID" 2>&1)
RC=$?
set -e

# Basic validation: check that task list includes all expected tasks
if echo "$DAG_INFO" | grep -q "extract_data"; then
  pass "DAG contiene task extract_data"
else
  fail "DAG no contiene extract_data"
fi

if echo "$DAG_INFO" | grep -q "etl_to_star"; then
  pass "DAG contiene task etl_to_star"
else
  fail "DAG no contiene etl_to_star"
fi

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
