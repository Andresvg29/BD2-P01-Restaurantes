"""
generate_geo.py — Genera coordenadas geoespaciales ficticias pero realistas
para restaurantes en San José, Costa Rica.

Crea nodos (:Location) y relaciones [:ROAD] en Neo4j con distancias Haversine.
Almacena campo `ubicacion` en MongoDB/PostgreSQL para cada restaurante.

Output:
  - neo4j/staging/locations.csv    → nodos Location
  - neo4j/staging/located_at.csv   → (:Restaurant)-[:LOCATED_AT]->(:Location)
  - neo4j/staging/roads.csv        → (:Location)-[:ROAD {distance_km}]->(:Location)
  - Actualiza BD operacional con campo ubicacion
"""

import csv
import hashlib
import math
import os
import sys

# Rangos realistas para San José, Costa Rica
LAT_MIN, LAT_MAX = 9.92, 9.94
LON_MIN, LON_MAX = -84.10, -84.08
ROAD_MAX_KM = 5.0


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


def deterministic_coords(seed_str):
    h = hashlib.md5(seed_str.encode()).hexdigest()
    lat_seed = int(h[:8], 16) % 10000
    lon_seed = int(h[8:16], 16) % 10000
    lat = LAT_MIN + (lat_seed / 10000.0) * (LAT_MAX - LAT_MIN)
    lon = LON_MIN + (lon_seed / 10000.0) * (LON_MAX - LON_MIN)
    return round(lat, 6), round(lon, 6)


def haversine(lat1, lon1, lat2, lon2):
    R = 6371.0
    dlat = math.radians(lat2 - lat1)
    dlon = math.radians(lon2 - lon1)
    a = (math.sin(dlat / 2) ** 2 +
         math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) *
         math.sin(dlon / 2) ** 2)
    return R * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))


def write_csv(filename, fieldnames, rows):
    out_dir = os.path.join(os.path.dirname(__file__), "..", "neo4j", "staging")
    os.makedirs(out_dir, exist_ok=True)
    path = os.path.join(out_dir, filename)
    with open(path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)
    print(f"  Exportados {len(rows)} registros -> {path}")
    return path


# ------------------------- RAMA MONGODB -------------------------

def generate_mongo():
    from pymongo import MongoClient

    uri = get_env_or_fail("MONGO_URI")
    db_name = get_env_or_fail("MONGO_DB")
    client = MongoClient(uri, serverSelectionTimeoutMS=5000)
    client.admin.command("ping")
    db = client[db_name]
    print(f"Conectado a MongoDB: {uri}")

    restaurants = list(db["restaurantes"].find())
    print(f"Restaurantes encontrados: {len(restaurants)}")

    locations = []
    located_at = []
    for doc in restaurants:
        rid = str(doc["_id"])
        addr = safe(doc.get("direccion"), "San José")
        name = safe(doc.get("nombre"), "Restaurante")
        loc_id = f"loc_{rid}"
        lat, lon = deterministic_coords(rid + name)
        locations.append({
            "location_id:ID(Location)": loc_id,
            "lat:float": str(lat),
            "lon:float": str(lon),
            "address": addr,
        })
        located_at.append({
            ":START_ID(Restaurant)": rid,
            ":END_ID(Location)": loc_id,
        })
        db["restaurantes"].update_one(
            {"_id": doc["_id"]},
            {"$set": {"ubicacion": {"lat": lat, "lon": lon}}}
        )

    write_csv("locations.csv",
              ["location_id:ID(Location)", "lat:float", "lon:float", "address"], locations)
    write_csv("located_at.csv",
              [":START_ID(Restaurant)", ":END_ID(Location)"], located_at)

    # ROAD relationships
    roads = []
    for i, a in enumerate(locations):
        for j, b in enumerate(locations):
            if i >= j:
                continue
            lat_a, lon_a = float(a["lat:float"]), float(a["lon:float"])
            lat_b, lon_b = float(b["lat:float"]), float(b["lon:float"])
            dist = haversine(lat_a, lon_a, lat_b, lon_b)
            if dist <= ROAD_MAX_KM:
                roads.append({
                    ":START_ID(Location)": a["location_id:ID(Location)"],
                    ":END_ID(Location)": b["location_id:ID(Location)"],
                    "distance_km:float": str(round(dist, 1)),
                })

    write_csv("roads.csv",
              [":START_ID(Location)", ":END_ID(Location)", "distance_km:float"], roads)
    client.close()


# ------------------------- RAMA POSTGRESQL -------------------------

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
    print(f"Conectado a PostgreSQL: {conn.dsn}")
    cur = conn.cursor(cursor_factory=RealDictCursor)

    cur.execute("SELECT id, nombre, direccion FROM restaurantes")
    restaurants = cur.fetchall()
    print(f"Restaurantes encontrados: {len(restaurants)}")

    locations = []
    located_at = []
    for doc in restaurants:
        rid = str(doc["id"])
        addr = safe(doc.get("direccion"), "San José")
        name = safe(doc.get("nombre"), "Restaurante")
        loc_id = f"loc_{rid}"
        lat, lon = deterministic_coords(rid + name)
        locations.append({
            "location_id:ID(Location)": loc_id,
            "lat:float": str(lat),
            "lon:float": str(lon),
            "address": addr,
        })
        located_at.append({
            ":START_ID(Restaurant)": rid,
            ":END_ID(Location)": loc_id,
        })
        cur.execute(
            "UPDATE restaurantes SET ubicacion = row_to_json(row(%s, %s)::ubicacion_type) WHERE id = %s",
            (lat, lon, doc["id"])
        )

    conn.commit()
    write_csv("locations.csv",
              ["location_id:ID(Location)", "lat:float", "lon:float", "address"], locations)
    write_csv("located_at.csv",
              [":START_ID(Restaurant)", ":END_ID(Location)"], located_at)

    roads = []
    for i, a in enumerate(locations):
        for j, b in enumerate(locations):
            if i >= j:
                continue
            lat_a, lon_a = float(a["lat:float"]), float(a["lon:float"])
            lat_b, lon_b = float(b["lat:float"]), float(b["lon:float"])
            dist = haversine(lat_a, lon_a, lat_b, lon_b)
            if dist <= ROAD_MAX_KM:
                roads.append({
                    ":START_ID(Location)": a["location_id:ID(Location)"],
                    ":END_ID(Location)": b["location_id:ID(Location)"],
                    "distance_km:float": str(round(dist, 1)),
                })

    write_csv("roads.csv",
              [":START_ID(Location)", ":END_ID(Location)", "distance_km:float"], roads)
    cur.close()
    conn.close()


# ------------------------- MAIN -------------------------

def main():
    engine = os.environ.get("DB_ENGINE", "mongodb").lower()
    print(f"=== Generando datos geoespaciales desde {engine} ===")

    if engine == "postgres" or engine == "postgresql":
        generate_postgres()
    else:
        generate_mongo()

    print("\n? Datos geoespaciales generados correctamente.")


if __name__ == "__main__":
    main()
