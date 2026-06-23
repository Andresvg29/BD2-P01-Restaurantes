"""
generate_oltp_data.py — Genera datos masivos de prueba para OLAP.

Crea cientos de pedidos, reservas, usuarios y restaurantes en la BD operacional.
Soporta tanto PostgreSQL como MongoDB según DB_ENGINE.

Ejecutar DESPUÉS de init.sql / initMongo.js (los datos base deben existir).
"""

import csv
import os
import random
import sys
from datetime import datetime, timedelta

random.seed(42)

# ── Configuración ──────────────────────────────────────────────────────────────

NUEVOS_RESTAURANTES = [
    ("El Chante Tico",       "Centro, 50 m sur del parque",       "Alajuela", "+506 2441-1234"),
    ("Sabor Tico",           "Multiplaza Escazú, local 25",       "Escazú",   "+506 2289-5678"),
    ("Mariscos del Pacífico","Paseo de los Turistas, frente al mar","Puntarenas","+506 2661-4321"),
    ("La Terraza",           "Barrio Escalante, San José",        "San José",  "+506 2253-8765"),
    ("El Rincón Tico",       "Calle 2, Alajuela centro",          "Alajuela", "+506 2442-3456"),
    ("La Casa del Sabor",    "Heredia centro, contiguo al parque","Heredia",   "+506 2261-7890"),
]

CATEGORIAS = ["tipica", "fusion", "mariscos", "asiatica", "italiana", "parrilla"]
NOMBRES_PLATOS = [
    ("Casado tradicional", 4500),
    ("Hamburguesa artesanal", 5500),
    ("Pasta Alfredo", 6200),
    ("Pollo a la plancha", 5200),
    ("Filete de pescado", 7500),
    ("Ceviche mixto", 6500),
    ("Arroz con pollo", 4800),
    ("Tacos al pastor", 5900),
    ("Lasagna", 7200),
    ("Parrillada para dos", 12500),
    ("Ensalada César", 4200),
    ("Sopa de pescado", 5100),
    ("Lomo saltado", 8500),
    ("Pizza margarita", 6800),
    ("Burrito grande", 5500),
]

ESTADOS_PEDIDO_MAP = {1: "pendiente", 2: "en_preparacion", 3: "completado", 4: "entregado"}
ESTADOS_RESERVA_MAP = {1: "pendiente", 2: "confirmada", 3: "cancelada"}


def get_env_or_fail(key):
    val = os.environ.get(key)
    if not val:
        print(f"ERROR: Variable de entorno {key} no definida", file=sys.stderr)
        sys.exit(1)
    return val


def random_date(start, end):
    return start + timedelta(
        seconds=random.randint(0, int((end - start).total_seconds()))
    )


def pick_weighted(options):
    """Pick a state with weighted probability."""
    total = sum(w for _, w in options)
    r = random.uniform(0, total)
    acum = 0
    for val, weight in options:
        acum += weight
        if r <= acum:
            return val
    return options[-1][0]


# ──────────────────────── RAMA POSTGRESQL ────────────────────────────────────

def generate_postgres():
    import psycopg2
    from psycopg2.extras import RealDictCursor

    conn = psycopg2.connect(
        host=os.environ.get("POSTGRES_HOST", "db"),
        port=int(os.environ.get("POSTGRES_PORT", 5432)),
        user=get_env_or_fail("POSTGRES_USER"),
        password=get_env_or_fail("POSTGRES_PASSWORD"),
        dbname=get_env_or_fail("POSTGRES_DB"),
    )
    conn.autocommit = False
    cur = conn.cursor()

    # ── Leer datos existentes ──────────────────────────────────────────────
    cur.execute("SELECT id, nombre FROM restaurantes")
    rest_existentes = {row[1]: row[0] for row in cur.fetchall()}
    print(f"Restaurantes existentes: {len(rest_existentes)}")

    cur.execute("SELECT id, nombre FROM usuarios WHERE id_rol = 2")
    clientes_existentes = {row[1]: row[0] for row in cur.fetchall()}
    print(f"Clientes existentes: {len(clientes_existentes)}")

    cur.execute("SELECT id, id_restaurante, nombre, precio FROM platos")
    platos_existentes = [{"id": r[0], "id_restaurante": r[1], "nombre": r[2], "precio": float(r[3])} for r in cur.fetchall()]
    platos_por_restaurante = {}
    for p in platos_existentes:
        platos_por_restaurante.setdefault(p["id_restaurante"], []).append(p)
    print(f"Platos existentes: {len(platos_existentes)}")

    try:
        # ── 1. Nuevos restaurantes ────────────────────────────────────────
        nuevos_rest_ids = {}
        print("\n>>> Creando nuevos restaurantes...")
        for nombre, direccion, ciudad, telefono in NUEVOS_RESTAURANTES:
            if nombre in rest_existentes:
                print(f"  Restaurante '{nombre}' ya existe, saltando.")
                nuevos_rest_ids[nombre] = rest_existentes[nombre]
                continue

            # Necesito un admin (usar un admin existente al azar)
            cur.execute("SELECT id FROM usuarios WHERE id_rol = 1 LIMIT 1")
            admin_id = cur.fetchone()[0]

            cur.execute(
                """INSERT INTO restaurantes (nombre, direccion, ciudad, telefono, id_administrador, capacidad_total)
                   VALUES (%s, %s, %s, %s, %s, %s) RETURNING id""",
                (nombre, direccion, ciudad, telefono, admin_id, random.randint(40, 120))
            )
            rest_id = cur.fetchone()[0]
            nuevos_rest_ids[nombre] = rest_id
            print(f"  Creado restaurante: {nombre} ({ciudad})")

            # Crear 5-8 platos para este restaurante
            platos_elegidos = random.sample(NOMBRES_PLATOS, random.randint(5, 8))
            for pnombre, pprecio in platos_elegidos:
                cat = random.choice(CATEGORIAS)
                cur.execute(
                    """INSERT INTO platos (id_restaurante, nombre, descripcion, precio, categoria)
                       VALUES (%s, %s, %s, %s, %s)""",
                    (rest_id, pnombre, f"{pnombre} delicioso", pprecio, cat)
                )
            print(f"  Creados {len(platos_elegidos)} platos para {nombre}")

        conn.commit()

        # ── Recargar platos para incluir los nuevos ───────────────────────
        cur.execute("SELECT id, id_restaurante, nombre, precio FROM platos")
        platos_todos = [{"id": r[0], "id_restaurante": r[1], "nombre": r[2], "precio": float(r[3])} for r in cur.fetchall()]
        platos_por_rest_id = {}
        for p in platos_todos:
            platos_por_rest_id.setdefault(p["id_restaurante"], []).append(p)

        # ── 2. Nuevos usuarios ─────────────────────────────────────────────
        nuevos_clientes_ids = []
        print("\n>>> Creando nuevos usuarios...")
        nombres = [
            "Sofía Méndez Rojas", "Andrés Campos Ureña", "Valeria Rojas Aguilar",
            "Diego Vega Solano", "Gabriela Sanabria Cordero", "Esteban Madrigal Porras",
            "Paola Zúñiga Quesada", "Cristian Carranza Mora", "María Leiva Chacón",
            "Jorge Villalobos Vargas", "Karla Campos Navarro", "Luis Conejo Hernández",
            "Ana Acuña Bolaños", "Pedro Obando Soto", "Rocío Salas Vargas",
        ]
        for nombre in nombres:
            email = nombre.lower().replace(" ", ".") + "@email.com"
            keycloak_id = "kc_gen_" + nombre.split()[0].lower() + str(random.randint(100, 999))
            cur.execute(
                """INSERT INTO usuarios (nombre, email, keycloak_id, id_rol)
                   VALUES (%s, %s, %s, 2) RETURNING id""",
                (nombre, email, keycloak_id)
            )
            uid = cur.fetchone()[0]
            nuevos_clientes_ids.append(uid)
        conn.commit()
        todos_clientes = clientes_existentes.values() + nuevos_clientes_ids
        print(f"  Creados {len(nuevos_clientes_ids)} nuevos clientes")

        # ── Recargar restaurantes completos ────────────────────────────────
        cur.execute("SELECT id FROM restaurantes")
        todos_rest_ids = [r[0] for r in cur.fetchall()]

        # ── 3. Generar pedidos (500+) ──────────────────────────────────────
        total_pedidos = 550
        fecha_fin = datetime(2026, 6, 1)
        fecha_ini = fecha_fin - timedelta(days=365)

        print(f"\n>>> Generando {total_pedidos} pedidos ({fecha_ini.date()} a {fecha_fin.date()})...")
        pedidos_creados = 0
        for i in range(total_pedidos):
            cliente_id = random.choice(todos_clientes)
            rest_id = random.choice(todos_rest_ids)
            platos_rest = platos_por_rest_id.get(rest_id, [])
            if not platos_rest:
                continue

            fecha = random_date(fecha_ini, fecha_fin)
            estado = pick_weighted([(3, 50), (4, 25), (1, 15), (2, 10)])  # mayormente completados

            # 1-5 items por pedido
            items = random.sample(platos_rest, min(random.randint(1, 5), len(platos_rest)))
            total = sum(p["precio"] * random.randint(1, 3) for p in items)
            total = round(total, 2)

            cur.execute(
                """INSERT INTO pedidos (id_usuario, id_restaurante, es_recogida, id_estado_pedido, total, fecha_creacion)
                   VALUES (%s, %s, %s, %s, %s, %s) RETURNING id""",
                (cliente_id, rest_id, random.choice([True, False]), estado, total, fecha)
            )
            pedido_id = cur.fetchone()[0]

            for p in items:
                cantidad = random.randint(1, 3)
                cur.execute(
                    """INSERT INTO detalles_pedido (id_pedido, id_plato, cantidad, precio)
                       VALUES (%s, %s, %s, %s)""",
                    (pedido_id, p["id"], cantidad, p["precio"])
                )

            pedidos_creados += 1
            if (i + 1) % 100 == 0:
                conn.commit()
                print(f"  ...{i+1} pedidos creados")

        conn.commit()
        print(f"  Total pedidos creados: {pedidos_creados}")

        # ── 4. Generar reservas (200+) ─────────────────────────────────────
        total_reservas = 250
        print(f"\n>>> Generando {total_reservas} reservas...")
        reservas_creadas = 0
        for _ in range(total_reservas):
            cliente_id = random.choice(todos_clientes)
            rest_id = random.choice(todos_rest_ids)
            fecha = random_date(fecha_ini, fecha_fin)
            # Poner la reserva en horario de restaurante (11:00-14:00 o 18:00-21:00)
            if random.random() < 0.6:
                fecha = fecha.replace(hour=random.randint(11, 14), minute=random.randint(0, 59))
            else:
                fecha = fecha.replace(hour=random.randint(18, 21), minute=random.randint(0, 59))

            estado = pick_weighted([(2, 50), (3, 30), (1, 20)])  # mayormente confirmadas
            personas = random.randint(1, 8)

            cur.execute(
                """INSERT INTO reservas (id_usuario, id_restaurante, fecha_reserva, cantidad_personas, id_estado, fecha_creacion)
                   VALUES (%s, %s, %s, %s, %s, %s)""",
                (cliente_id, rest_id, fecha, personas, estado, fecha)
            )
            reservas_creadas += 1

        conn.commit()
        print(f"  Total reservas creadas: {reservas_creadas}")

        # ── Resumen ────────────────────────────────────────────────────────
        cur.execute("SELECT count(*) FROM restaurantes")
        print(f"\n=== Resumen PostgreSQL ===")
        print(f"  Restaurantes: {cur.fetchone()[0]}")
        cur.execute("SELECT count(*) FROM usuarios")
        print(f"  Usuarios: {cur.fetchone()[0]}")
        cur.execute("SELECT count(*) FROM platos")
        print(f"  Platos: {cur.fetchone()[0]}")
        cur.execute("SELECT count(*) FROM pedidos")
        print(f"  Pedidos: {cur.fetchone()[0]}")
        cur.execute("SELECT count(*) FROM detalles_pedido")
        print(f"  Detalles de pedido: {cur.fetchone()[0]}")
        cur.execute("SELECT count(*) FROM reservas")
        print(f"  Reservas: {cur.fetchone()[0]}")

    except Exception as e:
        conn.rollback()
        print(f"ERROR: {e}", file=sys.stderr)
        raise
    finally:
        cur.close()
        conn.close()


# ──────────────────────── RAMA MONGODB ───────────────────────────────────────

def generate_mongo():
    from pymongo import MongoClient

    uri = get_env_or_fail("MONGO_URI")
    db_name = get_env_or_fail("MONGO_DB")
    client = MongoClient(uri, serverSelectionTimeoutMS=5000)
    client.admin.command("ping")
    db = client[db_name]
    print(f"Conectado a MongoDB: {uri}")

    # ── Leer datos existentes ──────────────────────────────────────────────
    rest_existentes = list(db["restaurantes"].find())
    rest_ids = [str(r["_id"]) for r in rest_existentes]
    rest_nombres = {str(r["_id"]): r.get("nombre") for r in rest_existentes}
    print(f"Restaurantes existentes: {len(rest_existentes)}")

    clientes_existentes = list(db["usuarios"].find({"id_rol": 2}))
    clientes_ids = [str(c["_id"]) for c in clientes_existentes]
    print(f"Clientes existentes: {len(clientes_existentes)}")

    platos_existentes = list(db["platos"].find())
    platos_por_rest = {}
    for p in platos_existentes:
        rid = str(p.get("id_restaurante", ""))
        platos_por_rest.setdefault(rid, []).append(p)
    print(f"Platos existentes: {len(platos_existentes)}")

    # ── 1. Nuevos restaurantes ─────────────────────────────────────────────
    print("\n>>> Creando nuevos restaurantes...")
    admin_ids = [str(a["_id"]) for a in db["usuarios"].find({"id_rol": 1})]

    nuevos_rest_ids = []
    for nombre, direccion, ciudad, telefono in NUEVOS_RESTAURANTES:
        if nombre in rest_nombres.values():
            print(f"  Restaurante '{nombre}' ya existe, saltando.")
            continue

        admin_id = random.choice(admin_ids) if admin_ids else "admin_sj_001"
        platos_elegidos = random.sample(NOMBRES_PLATOS, random.randint(5, 8))
        doc = {
            "_id": "rest_gen_" + nombre.lower().replace(" ", "_")[:20],
            "nombre": nombre,
            "direccion": direccion,
            "ciudad": ciudad,
            "telefono": telefono,
            "id_administrador": admin_id,
            "capacidad_total": random.randint(40, 120),
            "fecha_creacion": datetime.now(),
        }
        rest_id = doc["_id"]
        db["restaurantes"].insert_one(doc)

        for pnombre, pprecio in platos_elegidos:
            cat = random.choice(CATEGORIAS)
            db["platos"].insert_one({
                "_id": "plato_gen_" + pnombre.lower().replace(" ", "_")[:20] + "_" + rest_id[-8:],
                "id_restaurante": rest_id,
                "nombre": pnombre,
                "descripcion": f"{pnombre} delicioso",
                "precio": float(pprecio),
                "esta_disponible": True,
                "categoria": cat,
            })

        nuevos_rest_ids.append(rest_id)
        print(f"  Creado restaurante: {nombre} ({ciudad}) con {len(platos_elegidos)} platos")

    # Recargar
    todos_rest = list(db["restaurantes"].find())
    todos_rest_ids = [str(r["_id"]) for r in todos_rest]
    todos_platos = list(db["platos"].find())
    platos_por_rest_id = {}
    for p in todos_platos:
        rid = str(p.get("id_restaurante", ""))
        platos_por_rest_id.setdefault(rid, []).append(p)

    # ── 2. Nuevos usuarios ─────────────────────────────────────────────────
    print("\n>>> Creando nuevos usuarios...")
    nombres = [
        "Sofía Méndez Rojas", "Andrés Campos Ureña", "Valeria Rojas Aguilar",
        "Diego Vega Solano", "Gabriela Sanabria Cordero", "Esteban Madrigal Porras",
        "Paola Zúñiga Quesada", "Cristian Carranza Mora", "María Leiva Chacón",
        "Jorge Villalobos Vargas", "Karla Campos Navarro", "Luis Conejo Hernández",
        "Ana Acuña Bolaños", "Pedro Obando Soto", "Rocío Salas Vargas",
    ]
    nuevos_cliente_ids = []
    for nombre in nombres:
        email = nombre.lower().replace(" ", ".") + "@email.com"
        doc = {
            "_id": "usr_gen_" + nombre.split()[0].lower() + str(random.randint(100, 999)),
            "nombre": nombre,
            "email": email,
            "keycloak_id": "kc_gen_" + nombre.split()[0].lower() + str(random.randint(100, 999)),
            "id_rol": 2,
            "fecha_creacion": datetime.now(),
        }
        db["usuarios"].insert_one(doc)
        nuevos_cliente_ids.append(doc["_id"])

    todos_cliente_ids = clientes_ids + nuevos_cliente_ids
    print(f"  Creados {len(nuevos_cliente_ids)} nuevos clientes")

    # ── 3. Generar pedidos (500+) ──────────────────────────────────────────
    total_pedidos = 550
    fecha_fin = datetime(2026, 6, 1)
    fecha_ini = fecha_fin - timedelta(days=365)

    print(f"\n>>> Generando {total_pedidos} pedidos ({fecha_ini.date()} a {fecha_fin.date()})...")
    pedidos_creados = 0
    for i in range(total_pedidos):
        cliente_id = random.choice(todos_cliente_ids)
        rest_id = random.choice(todos_rest_ids)
        platos_rest = platos_por_rest_id.get(rest_id, [])
        if not platos_rest:
            continue

        fecha = random_date(fecha_ini, fecha_fin)
        estado = pick_weighted([(3, 50), (4, 25), (1, 15), (2, 10)])

        items = random.sample(platos_rest, min(random.randint(1, 5), len(platos_rest)))
        detalles = []
        total = 0.0
        for p in items:
            cantidad = random.randint(1, 3)
            precio = float(p.get("precio", 0))
            subtotal = round(precio * cantidad, 2)
            total += subtotal
            detalles.append({
                "_id": "det_gen_" + str(i) + "_" + str(len(detalles)),
                "id_plato": str(p["_id"]),
                "cantidad": cantidad,
                "precio": precio,
            })
        total = round(total, 2)

        db["pedidos"].insert_one({
            "_id": "ped_gen_" + str(i).zfill(5),
            "id_usuario": cliente_id,
            "id_restaurante": rest_id,
            "es_recogida": random.choice([True, False]),
            "id_estado_pedido": estado,
            "total": total,
            "fecha_creacion": fecha,
            "detalles": detalles,
        })
        pedidos_creados += 1
        if (i + 1) % 100 == 0:
            print(f"  ...{i+1} pedidos creados")

    print(f"  Total pedidos creados: {pedidos_creados}")

    # ── 4. Generar reservas (200+) ─────────────────────────────────────────
    total_reservas = 250
    print(f"\n>>> Generando {total_reservas} reservas...")
    for _ in range(total_reservas):
        cliente_id = random.choice(todos_cliente_ids)
        rest_id = random.choice(todos_rest_ids)
        fecha = random_date(fecha_ini, fecha_fin)
        if random.random() < 0.6:
            fecha = fecha.replace(hour=random.randint(11, 14), minute=random.randint(0, 59))
        else:
            fecha = fecha.replace(hour=random.randint(18, 21), minute=random.randint(0, 59))

        estado = pick_weighted([(2, 50), (3, 30), (1, 20)])
        personas = random.randint(1, 8)

        doc = {
            "_id": "res_gen_" + str(random.randint(0, 999999)).zfill(6),
            "id_usuario": cliente_id,
            "id_restaurante": rest_id,
            "fecha_reserva": fecha,
            "cantidad_personas": personas,
            "id_estado": estado,
            "nombre_estado": ESTADOS_RESERVA_MAP[estado],
            "fecha_creacion": datetime.now(),
        }
        db["reservas"].insert_one(doc)

    print(f"  Total reservas creadas: {total_reservas}")

    # ── Resumen ────────────────────────────────────────────────────────────
    print(f"\n=== Resumen MongoDB ===")
    print(f"  Restaurantes: {db['restaurantes'].count_documents({})}")
    print(f"  Usuarios: {db['usuarios'].count_documents({})}")
    print(f"  Platos: {db['platos'].count_documents({})}")
    print(f"  Pedidos: {db['pedidos'].count_documents({})}")
    print(f"  Reservas: {db['reservas'].count_documents({})}")

    client.close()


# ──────────────────────── MAIN ────────────────────────────────────────────────

def main():
    engine = os.environ.get("DB_ENGINE", "postgres").lower()
    print(f"=== Generador de datos OLTP ({engine}) ===")
    print("Usando semilla determinística para reproducibilidad.")

    if engine == "postgres" or engine == "postgresql":
        generate_postgres()
    else:
        generate_mongo()

    print("\n✓ Datos OLTP generados exitosamente.")


if __name__ == "__main__":
    main()
