#!/bin/bash
# verify_neo4j.sh — Validación de Neo4j (7.4).
#
# Checks:
#   1. Las 3 consultas Cypher devuelven resultados no vacíos
#   2. La consulta de camino mínimo retorna una ruta válida
#   3. Prueba de co-compra: verificar pares devueltos tienen sentido
#
# Uso: bash scripts/verify_neo4j.sh
# Requiere: contenedor neo4j corriendo con datos cargados

set -euo pipefail

PASS=0
FAIL=0
NEO4J_USER="${NEO4J_USER:-neo4j}"
NEO4J_PASSWORD="${NEO4J_PASSWORD:-password}"

CYPHER_SHELL="docker exec neo4j cypher-shell -u $NEO4J_USER -p $NEO4J_PASSWORD"

pass() { echo "  [PASS] $1"; PASS=$((PASS + 1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL + 1)); }

echo "=== verify_neo4j.sh — Validación de Neo4j ==="
echo ""

# ── Check 1: Co-compra ─────────────────────────────────────
echo "--- Check 1: Top 5 productos más comprados juntos ---"

CO_COMPRA_QUERY='MATCH (p1:Product)<-[:INCLUDES]-(o:Order)-[:INCLUDES]->(p2:Product)
WHERE p1 <> p2
RETURN p1.nombre AS producto_a, p2.nombre AS producto_b, count(*) AS frecuencia
ORDER BY frecuencia DESC LIMIT 5;'

set +e
CO_OUTPUT=$($CYPHER_SHELL "$CO_COMPRA_QUERY" 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ]; then
  # Count lines of actual data (skip header + separator)
  DATA_LINES=$(echo "$CO_OUTPUT" | grep -c -v "^---" 2>/dev/null || echo 0)
  ROWS=$((DATA_LINES - 1))  # Subtract header row
  if [ "$ROWS" -gt 0 ]; then
    pass "Co-compra: $ROWS pares encontrados"
    echo "    Resultados:"
    echo "$CO_OUTPUT" | sed 's/^/    /'
    # Verify pairs are different products
    SAME_PAIRS=$(echo "$CO_OUTPUT" | tail -n +2 | awk -F', ' '$1 == $2' | wc -l)
    if [ "$SAME_PAIRS" -gt 0 ]; then
      fail "Co-compra: algunos pares tienen producto_a = producto_b"
    else
      pass "Co-compra: todos los pares son productos distintos"
    fi
  else
    fail "Co-compra: sin resultados (no hay datos de co-compra)"
  fi
else
  fail "Co-compra: error ejecutando consulta ($RC)"
  echo "    Error: $CO_OUTPUT" | head -3
fi

# ── Check 2: Usuarios con afinidad ─────────────────────────
echo ""
echo "--- Check 2: Usuarios que recomiendan (afinidad) ---"

AFINIDAD_QUERY='MATCH (u1:User)-[:PLACED]->(:Order)-[:AT]->(r:Restaurant),
      (u2:User)-[:PLACED]->(:Order)-[:AT]->(r)
WHERE u1 <> u2
MATCH (u1)-[:PLACED]->(:Order)-[:INCLUDES]->(p:Product),
      (u2)-[:PLACED]->(:Order)-[:INCLUDES]->(p)
RETURN u1.nombre AS usuario_a, u2.nombre AS usuario_b,
       count(DISTINCT r) + count(DISTINCT p) AS afinidad
ORDER BY afinidad DESC LIMIT 10;'

set +e
AF_OUTPUT=$($CYPHER_SHELL "$AFINIDAD_QUERY" 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ]; then
  DATA_LINES=$(echo "$AF_OUTPUT" | grep -c -v "^---" 2>/dev/null || echo 0)
  ROWS=$((DATA_LINES - 1))
  if [ "$ROWS" -gt 0 ]; then
    pass "Afinidad: $ROWS pares de usuarios encontrados"
  else
    fail "Afinidad: sin resultados"
  fi
else
  fail "Afinidad: error ejecutando consulta ($RC)"
  echo "    Error: $AF_OUTPUT" | head -3
fi

# ── Check 3: ShortestPath ──────────────────────────────────
echo ""
echo "--- Check 3: Camino mínimo entre ubicaciones ---"

# First, get a sample of location_ids
GET_LOCS="MATCH (l:Location) RETURN l.location_id AS id LIMIT 2;"
set +e
LOCS_OUTPUT=$($CYPHER_SHELL "$GET_LOCS" 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ] && [ -n "$LOCS_OUTPUT" ]; then
  LOC_IDS=($(echo "$LOCS_OUTPUT" | tail -n +2 | head -2 | awk -F', ' '{print $1}'))
  if [ ${#LOC_IDS[@]} -ge 2 ]; then
    ORIGEN="${LOC_IDS[0]}"
    DESTINO="${LOC_IDS[1]}"
    echo "  Probando shortestPath: $ORIGEN → $DESTINO"

    SP_QUERY="MATCH (a:Location {location_id: '${ORIGEN}'}),
                    (b:Location {location_id: '${DESTINO}'}),
                    p = shortestPath((a)-[:ROAD*]-(b))
              RETURN [n IN nodes(p) | n.address] AS ruta,
                     reduce(acc = 0, r IN relationships(p) | acc + r.distance_km) AS distancia_total_km;"

    set +e
    SP_OUTPUT=$($CYPHER_SHELL "$SP_QUERY" 2>&1)
    RC=$?
    set -e

    if [ $RC -eq 0 ]; then
      DATA_LINES=$(echo "$SP_OUTPUT" | grep -c -v "^---" 2>/dev/null || echo 0)
      ROWS=$((DATA_LINES - 1))
      if [ "$ROWS" -gt 0 ]; then
        pass "ShortestPath: ruta encontrada ($ORIGEN → $DESTINO)"
        echo "    Ruta: $SP_OUTPUT" | sed 's/^/    /'
      else
        fail "ShortestPath: no hay ruta entre las ubicaciones"
      fi
    else
      fail "ShortestPath: error ejecutando consulta ($RC)"
    fi
  else
    fail "ShortestPath: no hay suficientes Location nodes para probar"
  fi
else
  fail "ShortestPath: no se pudieron obtener location_ids"
fi

# ── Check 4: Usuarios influyentes (alternativa centralidad) ─
echo ""
echo "--- Check 4: Usuarios influyentes (centralidad por grado) ---"

INFLUENCIA_QUERY='MATCH (u:User)-[:PLACED]->(o:Order)-[:INCLUDES]->(p:Product)
RETURN u.nombre AS usuario,
       count(DISTINCT o) AS total_pedidos,
       count(DISTINCT p) AS productos_distintos
ORDER BY total_pedidos DESC LIMIT 5;'

set +e
IN_OUTPUT=$($CYPHER_SHELL "$INFLUENCIA_QUERY" 2>&1)
RC=$?
set -e

if [ $RC -eq 0 ]; then
  DATA_LINES=$(echo "$IN_OUTPUT" | grep -c -v "^---" 2>/dev/null || echo 0)
  ROWS=$((DATA_LINES - 1))
  if [ "$ROWS" -gt 0 ]; then
    pass "Influencia: $ROWS usuarios influyentes encontrados"
    echo "    Top influyentes:"
    echo "$IN_OUTPUT" | sed 's/^/    /'
  else
    fail "Influencia: sin resultados"
  fi
else
  fail "Influencia: error ejecutando consulta ($RC)"
fi

echo ""
echo "=== RESULTADOS: $PASS PASS, $FAIL FAIL ==="
[ "$FAIL" -eq 0 ]
