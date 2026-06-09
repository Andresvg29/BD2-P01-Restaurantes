"""
export_data.py — Extrae datos desde MongoDB o PostgreSQL a CSV temporales.

Detecta DB_ENGINE del .env para elegir fuente:
  - "mongodb": extrae desde MongoDB (PyMongo)
  - "postgres": extrae desde PostgreSQL (psycopg2)

Colecciones/tablas extraídas: usuarios, restaurantes, platos, pedidos, detalles_pedido, reservas
Salida: CSVs en warehouse/etl/staging/
"""

import csv
import os
import sys


def get_env_or_fail(key):
    val = os.environ.get(key)
    if not val:
        print(f"ERROR: Variable de entorno {key} no definida", file=sys.stderr)
        sys.exit(1)
    return val


def safe(val, default=""):
    if val is None:
        return default
    return str(val)


def write_csv(filename, fieldnames, rows):
    os.makedirs("staging", exist_ok=True)
    path = f"staging/{filename}"
    with open(path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)
    print(f"  Exportados {len(rows)} registros → {path}")
    return path


# ──────────────────────── RAMA MONGODB ────────────────────────

def export_mongo():
    from pymongo import MongoClient

    uri = get_env_or_fail("MONGO_URI")
    db_name = get_env_or_fail("MONGO_DB")
    client = MongoClient(uri, serverSelectionTimeoutMS=5000)
    client.admin.command("ping")
    db = client[db_name]
    print(f"Conectado a MongoDB: {uri}")

    # usuarios
    rows = []
    for doc in db["usuarios"].find():
        rows.append({
            "id": str(doc["_id"]), "nombre": safe(doc.get("nombre")),
            "email": safe(doc.get("email")), "keycloak_id": safe(doc.get("keycloak_id")),
            "id_rol": safe(doc.get("id_rol"), "2"),
            "fecha_creacion": str(doc.get("fecha_creacion", "")),
        })
    write_csv("usuarios.csv",
              ["id", "nombre", "email", "keycloak_id", "id_rol", "fecha_creacion"], rows)

    # restaurantes
    rows = []
    for doc in db["restaurantes"].find():
        rows.append({
            "id": str(doc["_id"]), "nombre": safe(doc.get("nombre")),
            "direccion": safe(doc.get("direccion")), "ciudad": safe(doc.get("ciudad"), ""),
            "telefono": safe(doc.get("telefono")),
            "id_administrador": str(doc.get("id_administrador", "")),
            "capacidad_total": safe(doc.get("capacidad_total"), "0"),
            "fecha_creacion": str(doc.get("fecha_creacion", "")),
        })
    write_csv("restaurantes.csv",
              ["id", "nombre", "direccion", "ciudad", "telefono", "id_administrador",
               "capacidad_total", "fecha_creacion"], rows)

    # platos
    rows = []
    for doc in db["platos"].find():
        rows.append({
            "id": str(doc["_id"]), "id_restaurante": str(doc.get("id_restaurante", "")),
            "nombre": safe(doc.get("nombre")), "descripcion": safe(doc.get("descripcion")),
            "precio": str(doc.get("precio", "0")),
            "esta_disponible": str(doc.get("esta_disponible", "true")),
            "categoria": safe(doc.get("categoria"), "general"),
        })
    write_csv("platos.csv",
              ["id", "id_restaurante", "nombre", "descripcion",
               "precio", "esta_disponible", "categoria"], rows)

    # pedidos + detalles
    pedidos_rows = []
    detalles_rows = []
    for doc in db["pedidos"].find():
        pid = str(doc["_id"])
        pedidos_rows.append({
            "id": pid, "id_usuario": str(doc.get("id_usuario", "")),
            "id_restaurante": str(doc.get("id_restaurante", "")),
            "es_recogida": str(doc.get("es_recogida", "false")),
            "id_estado_pedido": safe(doc.get("id_estado_pedido"), "1"),
            "total": str(doc.get("total", "0")),
            "fecha_creacion": str(doc.get("fecha_creacion", "")),
        })
        for det in doc.get("detalles", []):
            detalles_rows.append({
                "id": str(det.get("_id", "")), "id_pedido": pid,
                "id_plato": str(det.get("id_plato", "")),
                "cantidad": safe(det.get("cantidad"), "1"),
                "precio": str(det.get("precio", "0")),
            })
    write_csv("pedidos.csv",
              ["id", "id_usuario", "id_restaurante", "es_recogida",
               "id_estado_pedido", "total", "fecha_creacion"], pedidos_rows)
    write_csv("detalles_pedido.csv",
              ["id", "id_pedido", "id_plato", "cantidad", "precio"], detalles_rows)

    # reservas
    rows = []
    for doc in db["reservas"].find():
        rows.append({
            "id": str(doc["_id"]), "id_usuario": str(doc.get("id_usuario", "")),
            "id_restaurante": str(doc.get("id_restaurante", "")),
            "fecha_reserva": str(doc.get("fecha_reserva", "")),
            "cantidad_personas": safe(doc.get("cantidad_personas"), "1"),
            "id_estado": safe(doc.get("id_estado"), "1"),
            "fecha_creacion": str(doc.get("fecha_creacion", "")),
        })
    write_csv("reservas.csv",
              ["id", "id_usuario", "id_restaurante", "fecha_reserva",
               "cantidad_personas", "id_estado", "fecha_creacion"], rows)

    client.close()


# ──────────────────────── RAMA POSTGRESQL ────────────────────────

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
    print(f"Conectado a PostgreSQL: {conn.dsn}")

    cur = conn.cursor(cursor_factory=RealDictCursor)

    # usuarios
    cur.execute("SELECT id, nombre, email, keycloak_id, id_rol, fecha_creacion FROM usuarios")
    rows = [{"id": str(r["id"]), "nombre": safe(r["nombre"]), "email": safe(r["email"]),
             "keycloak_id": safe(r["keycloak_id"]), "id_rol": str(r["id_rol"]),
             "fecha_creacion": str(r["fecha_creacion"] or "")} for r in cur.fetchall()]
    write_csv("usuarios.csv",
              ["id", "nombre", "email", "keycloak_id", "id_rol", "fecha_creacion"], rows)

    # restaurantes
    cur.execute("SELECT id, nombre, direccion, ciudad, telefono, id_administrador, capacidad_total FROM restaurantes")
    rows = [{"id": str(r["id"]), "nombre": safe(r["nombre"]), "direccion": safe(r["direccion"]),
             "ciudad": safe(r["ciudad"], ""),
             "telefono": safe(r["telefono"]), "id_administrador": str(r["id_administrador"]),
             "capacidad_total": str(r["capacidad_total"]), "fecha_creacion": ""} for r in cur.fetchall()]
    write_csv("restaurantes.csv",
              ["id", "nombre", "direccion", "ciudad", "telefono", "id_administrador",
               "capacidad_total", "fecha_creacion"], rows)

    # platos
    cur.execute("SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria FROM platos")
    rows = [{"id": str(r["id"]), "id_restaurante": str(r["id_restaurante"]),
             "nombre": safe(r["nombre"]), "descripcion": safe(r["descripcion"]),
             "precio": str(r["precio"]), "esta_disponible": str(r["esta_disponible"]),
             "categoria": safe(r["categoria"], "general")} for r in cur.fetchall()]
    write_csv("platos.csv",
              ["id", "id_restaurante", "nombre", "descripcion",
               "precio", "esta_disponible", "categoria"], rows)

    # pedidos
    cur.execute("""
        SELECT id, id_usuario, id_restaurante, es_recogida, id_estado_pedido, total, fecha_creacion
        FROM pedidos
    """)
    pedidos_rows = [{"id": str(r["id"]), "id_usuario": str(r["id_usuario"]),
                     "id_restaurante": str(r["id_restaurante"]),
                     "es_recogida": str(r["es_recogida"]),
                     "id_estado_pedido": str(r["id_estado_pedido"]),
                     "total": str(r["total"]),
                     "fecha_creacion": str(r["fecha_creacion"] or "")} for r in cur.fetchall()]
    write_csv("pedidos.csv",
              ["id", "id_usuario", "id_restaurante", "es_recogida",
               "id_estado_pedido", "total", "fecha_creacion"], pedidos_rows)

    # detalles_pedido
    cur.execute("SELECT id, id_pedido, id_plato, cantidad, precio FROM detalles_pedido")
    detalles_rows = [{"id": str(r["id"]), "id_pedido": str(r["id_pedido"]),
                      "id_plato": str(r["id_plato"]),
                      "cantidad": str(r["cantidad"]),
                      "precio": str(r["precio"])} for r in cur.fetchall()]
    write_csv("detalles_pedido.csv",
              ["id", "id_pedido", "id_plato", "cantidad", "precio"], detalles_rows)

    # reservas
    cur.execute("SELECT id, id_usuario, id_restaurante, fecha_reserva, cantidad_personas, id_estado, fecha_creacion FROM reservas")
    rows = [{"id": str(r["id"]), "id_usuario": str(r["id_usuario"]),
             "id_restaurante": str(r["id_restaurante"]),
             "fecha_reserva": str(r["fecha_reserva"] or ""),
             "cantidad_personas": str(r["cantidad_personas"]),
             "id_estado": str(r["id_estado"]),
             "fecha_creacion": str(r["fecha_creacion"] or "")} for r in cur.fetchall()]
    write_csv("reservas.csv",
              ["id", "id_usuario", "id_restaurante", "fecha_reserva",
               "cantidad_personas", "id_estado", "fecha_creacion"], rows)

    cur.close()
    conn.close()


# ──────────────────────── MAIN ────────────────────────

def main():
    engine = os.environ.get("DB_ENGINE", "mongodb").lower()
    print(f"=== ETL: Extracción desde {engine} ===")

    if engine == "postgres" or engine == "postgresql":
        export_postgres()
    else:
        export_mongo()

    print("\n✓ Extracción completada.")


if __name__ == "__main__":
    main()
