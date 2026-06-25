"""
export_neo4j.py — Exporta datos desde PostgreSQL/MongoDB a CSVs para Neo4j.

Genera CSVs de nodos y relaciones compatibles con load_neo4j.cypher.
Salida: directorio configurable via OUTPUT_DIR (default: staging/neo4j/)
"""

import csv
import os
import sys

OUTPUT_DIR = os.environ.get("OUTPUT_DIR", "staging/neo4j")


def get_env_or_fail(key):
    val = os.environ.get(key)
    if not val:
        print(f"ERROR: Variable {key} no definida", file=sys.stderr)
        sys.exit(1)
    return val


def safe(val, default=""):
    if val is None:
        return default
    return str(val)


def write_csv(filename, fieldnames, rows):
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    path = os.path.join(OUTPUT_DIR, filename)
    with open(path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)
    print(f"  Exportados {len(rows)} registros -> {path}")
    return path


def export_postgres():
    import psycopg2
    from psycopg2.extras import RealDictCursor

    conn = psycopg2.connect(
        host=os.environ.get("POSTGRES_HOST", "db"),
        port=int(os.environ.get("POSTGRES_PORT", 5432)),
        user=get_env_or_fail("POSTGRES_USER"),
        password=get_env_or_fail("POSTGRES_PASSWORD"),
        dbname=get_env_or_fail("POSTGRES_DB"),
    )
    print(f"Conectado a PostgreSQL")
    cur = conn.cursor(cursor_factory=RealDictCursor)

    # --- NODOS ---
    cur.execute("SELECT id, nombre, email FROM usuarios")
    rows = [{"user_id:ID(User)": str(r["id"]), "nombre": safe(r["nombre"]),
             "email": safe(r["email"])} for r in cur.fetchall()]
    write_csv("users.csv", ["user_id:ID(User)", "nombre", "email"], rows)

    cur.execute("SELECT id, nombre, categoria, precio FROM platos")
    rows = [{"product_id:ID(Product)": str(r["id"]), "nombre": safe(r["nombre"]),
             "categoria": safe(r["categoria"], "general"),
             "precio:float": str(r["precio"])} for r in cur.fetchall()]
    write_csv("products.csv", ["product_id:ID(Product)", "nombre", "categoria", "precio:float"], rows)

    cur.execute("SELECT id, nombre, direccion FROM restaurantes")
    rows = [{"restaurant_id:ID(Restaurant)": str(r["id"]), "nombre": safe(r["nombre"]),
             "direccion": safe(r["direccion"])} for r in cur.fetchall()]
    write_csv("restaurants.csv", ["restaurant_id:ID(Restaurant)", "nombre", "direccion"], rows)

    cur.execute("SELECT id, total, fecha_creacion FROM pedidos")
    rows = [{"order_id:ID(Order)": str(r["id"]),
             "total_amount:float": str(r["total"]),
             "fecha": str(r["fecha_creacion"] or "")} for r in cur.fetchall()]
    write_csv("orders.csv", ["order_id:ID(Order)", "total_amount:float", "fecha"], rows)

    cur.execute("SELECT id, fecha_reserva, cantidad_personas FROM reservas")
    rows = [{"reservation_id:ID(Reservation)": str(r["id"]),
             "fecha": str(r["fecha_reserva"] or ""),
             "party_size:int": str(r["cantidad_personas"])} for r in cur.fetchall()]
    write_csv("reservations.csv", ["reservation_id:ID(Reservation)", "fecha", "party_size:int"], rows)

    # --- RELACIONES ---
    cur.execute("SELECT id, id_usuario, fecha_creacion FROM pedidos")
    rows = [{"fecha": str(r["fecha_creacion"] or ""),
             ":START_ID(User)": str(r["id_usuario"]),
             ":END_ID(Order)": str(r["id"])} for r in cur.fetchall()]
    write_csv("placed.csv", [":START_ID(User)", ":END_ID(Order)", "fecha"], rows)

    cur.execute("""
        SELECT dp.id_pedido, dp.id_plato, dp.cantidad, dp.precio
        FROM detalles_pedido dp
    """)
    rows = [{"cantidad:int": str(r["cantidad"]), "subtotal:float": str(r["precio"]),
             ":START_ID(Order)": str(r["id_pedido"]),
             ":END_ID(Product)": str(r["id_plato"])} for r in cur.fetchall()]
    write_csv("includes.csv", [":START_ID(Order)", ":END_ID(Product)", "cantidad:int", "subtotal:float"], rows)

    cur.execute("SELECT id, id_restaurante FROM pedidos")
    rows = [{":START_ID(Order)": str(r["id"]),
             ":END_ID(Restaurant)": str(r["id_restaurante"])} for r in cur.fetchall()]
    write_csv("at_order.csv", [":START_ID(Order)", ":END_ID(Restaurant)"], rows)

    cur.execute("SELECT id, id_usuario FROM reservas")
    rows = [{":START_ID(User)": str(r["id_usuario"]),
             ":END_ID(Reservation)": str(r["id"])} for r in cur.fetchall()]
    write_csv("made_reservation.csv", [":START_ID(User)", ":END_ID(Reservation)"], rows)

    cur.execute("SELECT id, id_restaurante FROM reservas")
    rows = [{":START_ID(Reservation)": str(r["id"]),
             ":END_ID(Restaurant)": str(r["id_restaurante"])} for r in cur.fetchall()]
    write_csv("at_reservation.csv", [":START_ID(Reservation)", ":END_ID(Restaurant)"], rows)

    cur.close()
    conn.close()


def export_mongo():
    from pymongo import MongoClient

    uri = get_env_or_fail("MONGO_URI")
    db_name = get_env_or_fail("MONGO_DB")
    client = MongoClient(uri, serverSelectionTimeoutMS=5000)
    client.admin.command("ping")
    db = client[db_name]
    print(f"Conectado a MongoDB")

    rows = []
    for doc in db["usuarios"].find():
        rows.append({"user_id:ID(User)": str(doc["_id"]), "nombre": safe(doc.get("nombre")),
                     "email": safe(doc.get("email"))})
    write_csv("users.csv", ["user_id:ID(User)", "nombre", "email"], rows)

    rows = []
    for doc in db["platos"].find():
        rows.append({"product_id:ID(Product)": str(doc["_id"]), "nombre": safe(doc.get("nombre")),
                     "categoria": safe(doc.get("categoria"), "general"),
                     "precio:float": str(doc.get("precio", "0"))})
    write_csv("products.csv", ["product_id:ID(Product)", "nombre", "categoria", "precio:float"], rows)

    rows = []
    for doc in db["restaurantes"].find():
        rows.append({"restaurant_id:ID(Restaurant)": str(doc["_id"]), "nombre": safe(doc.get("nombre")),
                     "direccion": safe(doc.get("direccion"))})
    write_csv("restaurants.csv", ["restaurant_id:ID(Restaurant)", "nombre", "direccion"], rows)

    rows = []
    for doc in db["pedidos"].find():
        rows.append({"order_id:ID(Order)": str(doc["_id"]),
                     "total_amount:float": str(doc.get("total", "0")),
                     "fecha": str(doc.get("fecha_creacion", ""))})
    write_csv("orders.csv", ["order_id:ID(Order)", "total_amount:float", "fecha"], rows)

    rows = []
    for doc in db["reservas"].find():
        rows.append({"reservation_id:ID(Reservation)": str(doc["_id"]),
                     "fecha": str(doc.get("fecha_reserva", "")),
                     "party_size:int": str(doc.get("cantidad_personas", "1"))})
    write_csv("reservations.csv", ["reservation_id:ID(Reservation)", "fecha", "party_size:int"], rows)

    rows = []
    for doc in db["pedidos"].find():
        rows.append({"fecha": str(doc.get("fecha_creacion", "")),
                     ":START_ID(User)": str(doc.get("id_usuario", "")),
                     ":END_ID(Order)": str(doc["_id"])})
    write_csv("placed.csv", [":START_ID(User)", ":END_ID(Order)", "fecha"], rows)

    rows = []
    for doc in db["pedidos"].find():
        oid = str(doc["_id"])
        for det in doc.get("detalles", []):
            rows.append({"cantidad:int": safe(det.get("cantidad"), "1"),
                         "subtotal:float": str(det.get("precio", "0")),
                         ":START_ID(Order)": oid,
                         ":END_ID(Product)": str(det.get("id_plato", ""))})
    write_csv("includes.csv", [":START_ID(Order)", ":END_ID(Product)", "cantidad:int", "subtotal:float"], rows)

    rows = []
    for doc in db["pedidos"].find():
        rows.append({":START_ID(Order)": str(doc["_id"]),
                     ":END_ID(Restaurant)": str(doc.get("id_restaurante", ""))})
    write_csv("at_order.csv", [":START_ID(Order)", ":END_ID(Restaurant)"], rows)

    rows = []
    for doc in db["reservas"].find():
        rows.append({":START_ID(User)": str(doc.get("id_usuario", "")),
                     ":END_ID(Reservation)": str(doc["_id"])})
    write_csv("made_reservation.csv", [":START_ID(User)", ":END_ID(Reservation)"], rows)

    rows = []
    for doc in db["reservas"].find():
        rows.append({":START_ID(Reservation)": str(doc["_id"]),
                     ":END_ID(Restaurant)": str(doc.get("id_restaurante", ""))})
    write_csv("at_reservation.csv", [":START_ID(Reservation)", ":END_ID(Restaurant)"], rows)

    client.close()


def main():
    engine = os.environ.get("DB_ENGINE", "mongodb").lower()
    print(f"=== Neo4j: Exportando desde {engine} ===")

    if engine == "postgres" or engine == "postgresql":
        export_postgres()
    else:
        export_mongo()

    print(f"Exportacion completada en {OUTPUT_DIR}/")


if __name__ == "__main__":
    main()
