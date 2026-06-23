#!/usr/bin/env python3
"""
verify_warehouse.py — Validación de integridad de datos en el Warehouse (Hive).

Checks:
  1. COUNT(*) entre BD operacional y Hive
  2. Surrogate keys en fact tables tienen correspondencia en dimensiones
  3. fact_orders.total_amount = SUM(fact_order_details.subtotal)
  4. Unicidad de surrogate keys en dimensiones

Uso: python scripts/verify_warehouse.py
Requiere: variables de entorno de .env, servicios Docker corriendo
"""

import os
import sys
import subprocess
import csv
import io
from pathlib import Path


PASS = 0
FAIL = 0
CHECKS = []


def load_dotenv():
    dotenv_path = Path(__file__).resolve().parent.parent / ".env"
    if dotenv_path.exists():
        with open(dotenv_path) as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, _, val = line.partition("=")
                os.environ.setdefault(key.strip(), val.strip())


def check(name, ok, detail=""):
    global PASS, FAIL
    if ok:
        PASS += 1
        status = "PASS"
    else:
        FAIL += 1
        status = "FAIL"
    CHECKS.append((name, status, detail))
    print(f"  [{status}] {name}" + (f" — {detail}" if detail else ""))


def run_hive(query):
    """Ejecuta query en Hive via docker exec beeline y retorna lista de dicts."""
    hive_url = os.environ.get("HIVE_URL", "jdbc:hive2://localhost:10000/default")
    cmd = [
        "docker", "exec", "hive-server",
        "beeline", "-u", hive_url,
        "-e", query,
        "--silent=true", "--outputformat=csv2",
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        if result.returncode != 0:
            print(f"  ⚠ Hive query error (rc={result.returncode}): {result.stderr[:200]}")
            return None
        reader = csv.DictReader(io.StringIO(result.stdout))
        return list(reader)
    except subprocess.TimeoutExpired:
        print("  ⚠ Hive query timed out")
        return None
    except FileNotFoundError:
        print("  ⚠ docker not found — ensure Docker Desktop is running")
        return None


# ──────────────────── Conexión BD Operacional ────────────────────

def connect_db():
    engine = os.environ.get("DB_ENGINE", "mongodb").lower()
    if engine in ("postgres", "postgresql"):
        import psycopg2
        from psycopg2.extras import RealDictCursor
        conn = psycopg2.connect(
            host=os.environ.get("POSTGRES_HOST", "localhost"),
            port=int(os.environ.get("POSTGRES_PORT", 5432)),
            user=os.environ.get("POSTGRES_USER", "admin"),
            password=os.environ.get("POSTGRES_PASSWORD", "password"),
            dbname=os.environ.get("POSTGRES_DB", "restaurante_db"),
        )
        cur = conn.cursor(cursor_factory=RealDictCursor)
        return {"type": "postgres", "conn": conn, "cur": cur}
    else:
        from pymongo import MongoClient
        uri = os.environ.get("MONGO_URI", "mongodb://localhost:27017/restaurante_db")
        db_name = os.environ.get("MONGO_DB", "restaurante_db")
        client = MongoClient(uri, serverSelectionTimeoutMS=5000)
        client.admin.command("ping")
        db = client[db_name]
        return {"type": "mongodb", "client": client, "db": db}


def count_source(db, table):
    if db["type"] == "postgres":
        db["cur"].execute(f"SELECT COUNT(*) AS cnt FROM {table}")
        return int(db["cur"].fetchone()["cnt"])
    else:
        return db["db"][table].count_documents({})


def close_db(db):
    if db["type"] == "postgres":
        db["cur"].close()
        db["conn"].close()
    else:
        db["client"].close()


# ──────────────────── CHECK 1: COUNT(*) ────────────────────

def check_counts(db):
    print("\n--- Check 1: Integridad COUNT(*) ---")
    mapping = [
        ("usuarios", "dim_user"),
        ("restaurantes", "dim_restaurant"),
        ("platos", "dim_product"),
        ("pedidos", "fact_orders"),
        ("reservas", "fact_reservations"),
    ]
    hive_queries = {
        "dim_user": "SELECT COUNT(*) AS cnt FROM dim_user",
        "dim_restaurant": "SELECT COUNT(*) AS cnt FROM dim_restaurant",
        "dim_product": "SELECT COUNT(*) AS cnt FROM dim_product",
        "fact_orders": "SELECT COUNT(*) AS cnt FROM fact_orders",
        "fact_reservations": "SELECT COUNT(*) AS cnt FROM fact_reservations",
    }

    for source_table, hive_table in mapping:
        src_count = count_source(db, source_table)
        hive_result = run_hive(hive_queries[hive_table])
        if hive_result is None:
            check(f"COUNT(*) {source_table} → {hive_table}", False, "Hive no responde")
            continue
        hive_count = int(hive_result[0]["cnt"])
        ok = src_count == hive_count
        check(
            f"COUNT(*) {source_table} ({src_count}) → {hive_table} ({hive_count})",
            ok,
            "" if ok else f"diferencia: {src_count} vs {hive_count}",
        )


# ──────────────────── CHECK 2: Surrogate Keys ────────────────────

def check_surrogate_keys():
    print("\n--- Check 2: Integridad de Surrogate Keys ---")

    fact_dim_pairs = [
        ("fact_orders", "user_key", "dim_user", "user_key"),
        ("fact_orders", "restaurant_key", "dim_restaurant", "restaurant_key"),
        ("fact_orders", "status_key", "dim_order_status", "status_key"),
        ("fact_orders", "date_key", "dim_date", "date_key"),
        ("fact_order_details", "product_key", "dim_product", "product_key"),
        ("fact_reservations", "user_key", "dim_user", "user_key"),
        ("fact_reservations", "restaurant_key", "dim_restaurant", "restaurant_key"),
        ("fact_reservations", "status_key", "dim_reservation_status", "status_key"),
        ("fact_reservations", "date_key", "dim_date", "date_key"),
    ]

    for fact, fk_col, dim, pk_col in fact_dim_pairs:
        # NULL check
        null_q = f"SELECT COUNT(*) AS cnt FROM {fact} WHERE {fk_col} IS NULL"
        null_res = run_hive(null_q)
        if null_res is None:
            check(f"NULL keys en {fact}.{fk_col}", False, "Hive no responde")
            continue
        null_cnt = int(null_res[0]["cnt"])
        check(f"Sin NULLs en {fact}.{fk_col}", null_cnt == 0, f"{null_cnt} NULLs" if null_cnt else "")

        # Orphan check
        orphan_q = f"""
            SELECT COUNT(*) AS cnt FROM {fact} f
            LEFT JOIN {dim} d ON f.{fk_col} = d.{pk_col}
            WHERE d.{pk_col} IS NULL
        """
        orphan_res = run_hive(orphan_q.replace("\n", " "))
        if orphan_res is None:
            check(f"Sin huérfanos en {fact}.{fk_col} → {dim}", False, "Hive no responde")
            continue
        orphan_cnt = int(orphan_res[0]["cnt"])
        check(
            f"Sin huérfanos en {fact}.{fk_col} → {dim}",
            orphan_cnt == 0,
            f"{orphan_cnt} huérfanos" if orphan_cnt else "",
        )


# ──────────────────── CHECK 3: total_amount vs SUM(subtotal) ────────────────────

def check_total_amount():
    print("\n--- Check 3: fact_orders.total_amount = SUM(details.subtotal) ---")
    q = """
        SELECT COUNT(*) AS cnt FROM fact_orders fo
        JOIN (
            SELECT order_id, ROUND(SUM(subtotal), 2) AS calc_total
            FROM fact_order_details
            GROUP BY order_id
        ) fod ON fo.order_id = fod.order_id
        WHERE ROUND(fo.total_amount, 2) != fod.calc_total
    """
    res = run_hive(q.replace("\n", " "))
    if res is None:
        check("total_amount == SUM(subtotal)", False, "Hive no responde")
        return
    mismatch = int(res[0]["cnt"])
    check(
        "total_amount == SUM(subtotal) en todos los pedidos",
        mismatch == 0,
        f"{mismatch} pedidos con discrepancia" if mismatch else "",
    )


# ──────────────────── CHECK 4: Unicidad de SK ────────────────────

def check_uniqueness():
    print("\n--- Check 4: Unicidad de Surrogate Keys ---")
    dims = ["dim_user", "dim_restaurant", "dim_product", "dim_date",
            "dim_order_status", "dim_reservation_status"]
    pk_cols = ["user_key", "restaurant_key", "product_key", "date_key",
               "status_key", "status_key"]

    for dim, pk in zip(dims, pk_cols):
        q = f"SELECT {pk}, COUNT(*) AS cnt FROM {dim} GROUP BY {pk} HAVING COUNT(*) > 1"
        res = run_hive(q)
        if res is None:
            check(f"SK únicos en {dim}.{pk}", False, "Hive no responde")
            continue
        dups = len(res)
        check(f"SK únicos en {dim}.{pk}", dups == 0, f"{dups} duplicados" if dups else "")


# ──────────────────── MAIN ────────────────────

def main():
    load_dotenv()
    print("=== verify_warehouse.py — Validación de Warehouse ===")
    print(f"DB_ENGINE={os.environ.get('DB_ENGINE', 'mongodb')}")

    db = connect_db()
    print(f"Conectado a BD operacional ({db['type']})")

    check_counts(db)
    check_surrogate_keys()
    check_total_amount()
    check_uniqueness()

    close_db(db)

    print(f"\n{'='*50}")
    print(f"RESUMEN: {PASS} PASS, {FAIL} FAIL, {len(CHECKS)} total")
    for name, status, detail in CHECKS:
        print(f"  [{status}] {name}")
    print(f"{'='*50}")

    return 0 if FAIL == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
