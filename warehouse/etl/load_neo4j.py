"""
load_neo4j.py — Carga CSVs en Neo4j via HTTP API.

Lee load_neo4j.cypher y envia cada statement al endpoint transaccional
de Neo4j (http://neo4j:7474/db/neo4j/tx/commit).
Requiere: requests (disponible en airflow-worker)
"""

import os
import sys
import time
import base64

import requests

NEO4J_HOST = os.environ.get("NEO4J_HOST", "neo4j")
NEO4J_HTTP_PORT = os.environ.get("NEO4J_HTTP_PORT", "7474")
NEO4J_USER = os.environ.get("NEO4J_USER", "neo4j")
NEO4J_PASSWORD = os.environ.get("NEO4J_PASSWORD", "password")
CYPHER_FILE = os.environ.get("CYPHER_FILE", "/opt/warehouse/etl/load_neo4j.cypher")


def neo4j_request(statements, retries=3):
    url = f"http://{NEO4J_HOST}:{NEO4J_HTTP_PORT}/db/neo4j/tx/commit"
    auth_token = base64.b64encode(f"{NEO4J_USER}:{NEO4J_PASSWORD}".encode()).decode()
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Basic {auth_token}",
        "Accept": "application/json",
    }
    stmts = [{"statement": s} for s in statements]
    payload = {"statements": stmts}

    for attempt in range(retries):
        try:
            resp = requests.post(url, json=payload, headers=headers, timeout=120)
            if resp.status_code == 200:
                data = resp.json()
                for result in data.get("results", []):
                    if result.get("errors"):
                        for err in result["errors"]:
                            print(f"  Error: {err['message']}", file=sys.stderr)
                            return False
                return True
            else:
                print(f"  HTTP {resp.status_code}: {resp.text[:200]}", file=sys.stderr)
        except requests.exceptions.RequestException as e:
            print(f"  Connection error (attempt {attempt+1}/{retries}): {e}", file=sys.stderr)
            if attempt < retries - 1:
                time.sleep(3)
    return False


def parse_statements(cypher_text):
    stmts = []
    current = []
    for line in cypher_text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("//"):
            continue
        current.append(line)
        if stripped.endswith(";"):
            stmts.append("\n".join(current))
            current = []
    if current:
        stmts.append("\n".join(current))
    return stmts


def main():
    print("=== Cargando datos en Neo4j ===")

    if not os.path.exists(CYPHER_FILE):
        print(f"ERROR: {CYPHER_FILE} no encontrado", file=sys.stderr)
        sys.exit(1)

    with open(CYPHER_FILE, "r") as f:
        cypher_text = f.read()

    statements = parse_statements(cypher_text)
    print(f"Procesando {len(statements)} statements...")

    for i, stmt in enumerate(statements):
        short = stmt[:80].replace("\n", " ")
        print(f"[{i+1}/{len(statements)}] {short}...")
        success = neo4j_request([stmt])
        if not success:
            print(f"  Statement {i+1} fallo. Abortando.", file=sys.stderr)
            sys.exit(1)
        time.sleep(0.5)

    print("Carga en Neo4j completada exitosamente.")


if __name__ == "__main__":
    main()
