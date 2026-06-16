#!/usr/bin/env python3
"""Setup Superset dashboards for the Restaurant OLAP Warehouse.

Creates Hive database connection, SQL datasets, charts, and dashboards
via the Superset REST API.

Usage:
    python setup_dashboards.py

Environment variables:
    SUPERSET_URL   (default: http://localhost:8088)
    SUPERSET_USER  (default: admin)
    SUPERSET_PASS  (default: admin)

Requirements:
    - Superset running at SUPERSET_URL
    - Hive data warehouse with star schema tables populated
    - pip install requests
"""

import json
import os
import sys
import time

try:
    import requests
except ImportError:
    print("ERROR: requests library not found. Run: pip install requests")
    sys.exit(1)

SUPERSET_URL = os.environ.get("SUPERSET_URL", "http://localhost:8088")
USERNAME = os.environ.get("SUPERSET_USER", "admin")
PASSWORD = os.environ.get("SUPERSET_PASSWORD", "admin")

DB_NAME = "Hive Warehouse"
DB_URI = "hive://hive-server:10000/default"

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
EXPORT_DIR = os.path.join(BASE_DIR, "dashboards")

session = requests.Session()
session.verify = False


# ---------------------------------------------------------------------------
# API Helpers
# ---------------------------------------------------------------------------

def api_get(path, **kwargs):
    resp = session.get(f"{SUPERSET_URL}{path}", **kwargs)
    if resp.status_code >= 400:
        print(f"  GET {path} [{resp.status_code}]: {resp.text[:150]}")
    return resp


def api_post(path, data=None):
    resp = session.post(f"{SUPERSET_URL}{path}", json=data or {})
    if resp.status_code >= 400:
        print(f"  POST {path} [{resp.status_code}]: {resp.text[:150]}")
    return resp


def api_put(path, data=None):
    resp = session.put(f"{SUPERSET_URL}{path}", json=data or {})
    if resp.status_code >= 400:
        print(f"  PUT {path} [{resp.status_code}]: {resp.text[:150]}")
    return resp


# ---------------------------------------------------------------------------
# Auth
# ---------------------------------------------------------------------------

def login():
    print("  Logging in as admin...")
    resp = api_post("/api/v1/security/login", {
        "username": USERNAME,
        "password": PASSWORD,
        "provider": "db",
        "refresh": True,
    })
    if resp.status_code != 200:
        print(f"  FATAL: Login failed: {resp.text}")
        sys.exit(1)
    token = resp.json()["access_token"]
    session.headers.update({"Authorization": f"Bearer {token}"})
    print("  Login OK")


# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

def create_database():
    resp = api_get("/api/v1/database/",
                   params={"q": "(page:0,page_size:100)"})
    resp.raise_for_status()
    for db in resp.json()["result"]:
        if db["database_name"] == DB_NAME:
            print(f"  Database '{DB_NAME}' already exists (id={db['id']})")
            return db["id"]

    resp = api_post("/api/v1/database/", {
        "database_name": DB_NAME,
        "sqlalchemy_uri": DB_URI,
        "expose_in_sqllab": True,
        "allow_dml": False,
        "allow_run_async": False,
        "extra": json.dumps({
            "allows_virtual_table_explore": True,
            "metadata_params": {},
            "engine_params": {},
        }),
    })
    if resp.status_code not in (200, 201):
        print(f"  FATAL: Failed to create database: {resp.text}")
        sys.exit(1)
    db_id = resp.json()["id"]
    print(f"  Database created (id={db_id})")
    return db_id


# ---------------------------------------------------------------------------
# Datasets (virtual SQL datasets)
# ---------------------------------------------------------------------------

DATASET_DEFS = [
    {
        "id": "d1_ingresos_mensuales_categoria",
        "name": "Ingresos Mensuales por Categoria",
        "sql": """SELECT d.year, d.month, dp.categoria,
  CAST(SUM(fod.subtotal) AS DOUBLE) AS ingresos,
  CAST(SUM(fod.quantity) AS INT) AS unidades
FROM fact_order_details fod
JOIN dim_product dp ON fod.product_key = dp.product_key
JOIN fact_orders fo ON fod.order_id = fo.order_id
JOIN dim_date d ON fo.date_key = d.date_key
GROUP BY d.year, d.month, dp.categoria""",
    },
    {
        "id": "d1_tendencia_ingresos",
        "name": "Tendencia Ingresos Mensuales",
        "sql": """SELECT d.year, d.month,
  CAST(SUM(fo.total_amount) AS DOUBLE) AS ingresos_totales,
  COUNT(DISTINCT fo.order_id) AS total_pedidos
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
GROUP BY d.year, d.month
ORDER BY d.year, d.month""",
    },
    {
        "id": "d1_top_categorias",
        "name": "Top 5 Categorias por Ingresos",
        "sql": """SELECT dp.categoria,
  CAST(SUM(fod.subtotal) AS DOUBLE) AS ingresos_totales,
  SUM(fod.quantity) AS unidades_vendidas
FROM fact_order_details fod
JOIN dim_product dp ON fod.product_key = dp.product_key
GROUP BY dp.categoria
ORDER BY ingresos_totales DESC
LIMIT 5""",
    },
    {
        "id": "d2_reservas_ciudad",
        "name": "Reservas por Ciudad",
        "sql": """SELECT r.ciudad,
  COUNT(DISTINCT fr.reservation_id) AS total_reservas,
  CAST(SUM(fr.party_size) AS INT) AS total_comensales
FROM fact_reservations fr
JOIN dim_restaurant r ON fr.restaurant_key = r.restaurant_key
GROUP BY r.ciudad
ORDER BY total_reservas DESC""",
    },
    {
        "id": "d2_clientes_ciudad",
        "name": "Clientes Activos por Ciudad",
        "sql": """SELECT r.ciudad,
  COUNT(DISTINCT fo.user_key) AS total_clientes
FROM fact_orders fo
JOIN dim_restaurant r ON fo.restaurant_key = r.restaurant_key
GROUP BY r.ciudad
ORDER BY total_clientes DESC""",
    },
    {
        "id": "d2_total_clientes",
        "name": "Total Clientes Activos",
        "sql": "SELECT COUNT(DISTINCT user_key) AS total_clientes_activos FROM fact_orders",
    },
    {
        "id": "d3_estado_mensual",
        "name": "Pedidos por Estado y Mes",
        "sql": """SELECT d.year, d.month, dos.status_name,
  COUNT(*) AS cantidad
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
JOIN dim_order_status dos ON fo.status_key = dos.status_key
GROUP BY d.year, d.month, dos.status_name
ORDER BY d.year, d.month, dos.status_name""",
    },
    {
        "id": "d3_proporcion_estado",
        "name": "Proporcion por Estado",
        "sql": """SELECT dos.status_name, COUNT(*) AS cantidad
FROM fact_orders fo
JOIN dim_order_status dos ON fo.status_key = dos.status_key
GROUP BY dos.status_name""",
    },
    {
        "id": "d3_cancelacion_restaurante",
        "name": "Tasa Cancelacion por Restaurante",
        "sql": """SELECT r.nombre AS restaurante,
  COUNT(*) AS total_pedidos,
  SUM(CASE WHEN dos.status_name IN ('Cancelado', 'Cancelled') THEN 1 ELSE 0 END) AS cancelados,
  ROUND(SUM(CASE WHEN dos.status_name IN ('Cancelado', 'Cancelled') THEN 1 ELSE 0 END) * 100.0
    / NULLIF(COUNT(*), 0), 2) AS tasa_cancelacion_pct
FROM fact_orders fo
JOIN dim_restaurant r ON fo.restaurant_key = r.restaurant_key
JOIN dim_order_status dos ON fo.status_key = dos.status_key
GROUP BY r.nombre
ORDER BY tasa_cancelacion_pct DESC""",
    },
]


def create_datasets(db_id):
    dataset_map = {}
    for dd in DATASET_DEFS:
        resp = api_get(
            "/api/v1/dataset/",
            params={
                "q": '(filters:!((col:table_name,opr:eq,value:\''
                + dd["id"] + "')))"
            },
        )
        if resp.status_code == 200 and resp.json().get("count", 0) > 0:
            ds_id = resp.json()["result"][0]["id"]
            print(f"  Dataset '{dd['id']}' exists (id={ds_id})")
            dataset_map[dd["id"]] = ds_id
            continue

        resp = api_post("/api/v1/dataset/", {
            "database": db_id,
            "schema": "default",
            "table_name": dd["id"],
            "sql": dd["sql"],
            "owners": [1],
            "is_managed_externally": False,
        })
        if resp.status_code not in (200, 201):
            print(f"  SKIP dataset '{dd['id']}': {resp.status_code}")
            continue
        ds_id = resp.json()["id"]
        print(f"  Dataset '{dd['id']}' created (id={ds_id})")
        dataset_map[dd["id"]] = ds_id
        time.sleep(0.3)

    return dataset_map


# ---------------------------------------------------------------------------
# Charts
# ---------------------------------------------------------------------------

def _adhoc_metric(col, agg="SUM", label=None):
    return {
        "expressionType": "SIMPLE",
        "column": {"column_name": col, "type": "DOUBLE"},
        "aggregate": agg,
        "label": label or f"{agg}({col})",
    }


def _make_params(viz_type, dataset_id, extras=None):
    params = {
        "datasource": f"{dataset_id}__query",
        "viz_type": viz_type,
        "adhoc_filters": [],
        "row_limit": 100,
        "order_desc": True,
    }
    if extras:
        params.update(extras)
    return json.dumps(params)


CHART_DEFS = [
    # Dashboard 1 - Ingresos por mes y categoria
    {
        "name": "Ingresos Mensuales por Categoria",
        "viz_type": "dist_bar",
        "dataset": "d1_ingresos_mensuales_categoria",
        "params": {
            "metrics": [_adhoc_metric("ingresos")],
            "groupby": ["categoria"],
            "columns": ["month"],
            "granularity_sqla": None,
            "time_range": "No filter",
            "color_scheme": "supersetColors",
        },
    },
    {
        "name": "Tendencia de Ingresos (12 meses)",
        "viz_type": "line",
        "dataset": "d1_tendencia_ingresos",
        "params": {
            "metrics": [_adhoc_metric("ingresos_totales")],
            "groupby": [],
            "granularity_sqla": None,
            "time_range": "No filter",
            "x_axis_label": "Mes",
            "y_axis_label": "Ingresos",
            "color_scheme": "supersetColors",
        },
    },
    {
        "name": "Top 5 Categorias",
        "viz_type": "table",
        "dataset": "d1_top_categorias",
        "params": {
            "columns": ["categoria", "ingresos_totales", "unidades_vendidas"],
            "order_by_cols": ["ingresos_totales DESC"],
            "row_limit": 5,
        },
    },
    # Dashboard 2 - Actividad por zona geografica
    {
        "name": "Reservas por Ciudad",
        "viz_type": "dist_bar",
        "dataset": "d2_reservas_ciudad",
        "params": {
            "metrics": [_adhoc_metric("total_reservas")],
            "groupby": ["ciudad"],
            "granularity_sqla": None,
            "time_range": "No filter",
            "color_scheme": "supersetColors",
        },
    },
    {
        "name": "Clientes Activos por Ciudad",
        "viz_type": "dist_bar",
        "dataset": "d2_clientes_ciudad",
        "params": {
            "metrics": [_adhoc_metric("total_clientes")],
            "groupby": ["ciudad"],
            "granularity_sqla": None,
            "time_range": "No filter",
            "color_scheme": "supersetColors",
        },
    },
    {
        "name": "Total Clientes Activos",
        "viz_type": "big_number_total",
        "dataset": "d2_total_clientes",
        "params": {
            "metric": _adhoc_metric("total_clientes_activos", agg="MAX"),
            "subheader": "Clientes activos",
            "y_axis_format": ",d",
        },
    },
    # Dashboard 3 - Completados vs Cancelados
    {
        "name": "Pedidos por Estado (Mensual)",
        "viz_type": "dist_bar",
        "dataset": "d3_estado_mensual",
        "params": {
            "metrics": [_adhoc_metric("cantidad")],
            "groupby": ["status_name"],
            "columns": ["month"],
            "granularity_sqla": None,
            "time_range": "No filter",
            "color_scheme": "supersetColors",
        },
    },
    {
        "name": "Proporcion Completados vs Cancelados",
        "viz_type": "pie",
        "dataset": "d3_proporcion_estado",
        "params": {
            "metrics": [_adhoc_metric("cantidad")],
            "groupby": ["status_name"],
            "granularity_sqla": None,
            "time_range": "No filter",
        },
    },
    {
        "name": "Tasa de Cancelacion por Restaurante",
        "viz_type": "table",
        "dataset": "d3_cancelacion_restaurante",
        "params": {
            "columns": [
                "restaurante", "total_pedidos", "cancelados",
                "tasa_cancelacion_pct",
            ],
            "order_by_cols": ["tasa_cancelacion_pct DESC"],
            "row_limit": 50,
        },
    },
]


def create_chart(cd, dataset_map, dashboard_ids):
    name = cd["name"]
    dataset_id = dataset_map.get(cd["dataset"])
    if not dataset_id:
        print(f"  SKIP chart '{name}': dataset '{cd['dataset']}' not found")
        return None

    # Check if chart exists
    resp = api_get(
        "/api/v1/chart/",
        params={"q": f"(filters:!((col:slice_name,opr:eq,value:'{name}')))"},
    )
    if resp.status_code == 200 and resp.json().get("count", 0) > 0:
        chart_id = resp.json()["result"][0]["id"]
        print(f"  Chart '{name}' exists (id={chart_id})")
        return chart_id

    payload = {
        "slice_name": name,
        "viz_type": cd["viz_type"],
        "params": _make_params(cd["viz_type"], dataset_id, cd["params"]),
        "datasource_id": dataset_id,
        "datasource_type": "query",
        "owners": [1],
        "dashboards": dashboard_ids,
    }

    resp = api_post("/api/v1/chart/", payload)
    if resp.status_code not in (200, 201):
        print(f"  ERROR creating chart '{name}': {resp.status_code}")
        return None

    chart_id = resp.json()["id"]
    print(f"  Chart '{name}' created (id={chart_id})")
    time.sleep(0.3)
    return chart_id


# ---------------------------------------------------------------------------
# Dashboards
# ---------------------------------------------------------------------------

def _position_json(title, chart_entries):
    """Build a dashboard position_json from chart entries.

    chart_entries: list of (chart_id, x, y, width, height)
    """
    pos = {
        "DASHBOARD_VERSION_KEY": "v2",
        "ROOT_ID": {
            "children": ["HEADER_ID", "GRID_ID"],
            "id": "ROOT_ID",
            "type": "ROOT",
        },
        "HEADER_ID": {
            "id": "HEADER_ID",
            "type": "HEADER",
            "meta": {"text": title},
        },
        "GRID_ID": {
            "children": [],
            "id": "GRID_ID",
            "type": "GRID",
            "parents": ["ROOT_ID"],
        },
    }

    for i, (cid, x, y, w, h) in enumerate(chart_entries):
        if cid is None:
            continue
        key = f"CHART-{i}"
        pos["GRID_ID"]["children"].append(key)
        pos[key] = {
            "children": [],
            "id": key,
            "meta": {
                "chartId": cid,
                "height": h,
                "width": w,
                "position": {"x": x, "y": y},
            },
            "type": "CHART",
            "parents": ["ROOT_ID", "GRID_ID"],
        }

    return json.dumps(pos)


DASHBOARD_DEFS = [
    {
        "title": "Restaurant - Ingresos por Mes y Categoria",
        "slug": "ingresos",
        "charts": [
            ("Ingresos Mensuales por Categoria", 0, 0, 8, 40),
            ("Tendencia de Ingresos (12 meses)", 8, 0, 4, 40),
            ("Top 5 Categorias", 0, 1, 12, 30),
        ],
    },
    {
        "title": "Restaurant - Actividad por Zona Geografica",
        "slug": "geografico",
        "charts": [
            ("Reservas por Ciudad", 0, 0, 6, 40),
            ("Clientes Activos por Ciudad", 6, 0, 6, 40),
            ("Total Clientes Activos", 0, 1, 12, 20),
        ],
    },
    {
        "title": "Restaurant - Completados vs Cancelados",
        "slug": "cancelaciones",
        "charts": [
            ("Pedidos por Estado (Mensual)", 0, 0, 8, 40),
            ("Proporcion Completados vs Cancelados", 8, 0, 4, 40),
            ("Tasa de Cancelacion por Restaurante", 0, 1, 12, 35),
        ],
    },
]


def create_dashboard(ddef, chart_ids):
    title = ddef["title"]
    slug = ddef["slug"]

    resp = api_get(
        "/api/v1/dashboard/",
        params={
            "q": f"(filters:!((col:dashboard_title,opr:eq,value:'{title}')))"
        },
    )
    if resp.status_code == 200 and resp.json().get("count", 0) > 0:
        dash_id = resp.json()["result"][0]["id"]
        print(f"  Dashboard '{title}' exists (id={dash_id})")
        # Update position_json with chart IDs
        entries = []
        for chart_name, x, y, w, h in ddef["charts"]:
            cid = chart_ids.get(chart_name)
            entries.append((cid, x, y, w, h))
        pos = _position_json(title, entries)
        api_put(f"/api/v1/dashboard/{dash_id}", {"position_json": pos})
        return dash_id

    # Create the dashboard first (without position_json)
    resp = api_post("/api/v1/dashboard/", {
        "dashboard_title": title,
        "slug": slug,
        "owners": [1],
        "published": True,
    })
    if resp.status_code not in (200, 201):
        print(f"  ERROR creating dashboard '{title}': {resp.status_code}")
        return None

    dash_id = resp.json()["id"]
    print(f"  Dashboard '{title}' created (id={dash_id})")
    time.sleep(0.3)

    # Update with position_json
    entries = []
    for chart_name, x, y, w, h in ddef["charts"]:
        cid = chart_ids.get(chart_name)
        entries.append((cid, x, y, w, h))
    pos = _position_json(title, entries)
    api_put(f"/api/v1/dashboard/{dash_id}", {"position_json": pos})

    return dash_id


# ---------------------------------------------------------------------------
# Export dashboards as JSON
# ---------------------------------------------------------------------------

def export_dashboards():
    os.makedirs(EXPORT_DIR, exist_ok=True)

    resp = api_get("/api/v1/dashboard/",
                   params={"q": "(page:0,page_size:100)"})
    if resp.status_code != 200:
        print(f"  SKIP export: failed to list dashboards")
        return

    for dash in resp.json()["result"]:
        dash_id = dash["id"]
        slug = dash.get("slug") or f"dashboard_{dash_id}"
        export_resp = api_get(f"/api/v1/dashboard/{dash_id}/export/")
        if export_resp.status_code == 200:
            filepath = os.path.join(EXPORT_DIR, f"{slug}.json")
            with open(filepath, "w", encoding="utf-8") as f:
                json.dump(export_resp.json(), f, indent=2)
            print(f"  Exported dashboard '{dash['dashboard_title']}' -> {filepath}")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    print("=" * 55)
    print("  Superset Dashboard Setup - Restaurant OLAP")
    print("=" * 55)

    login()

    print("\n[1/4] Creating database connection...")
    db_id = create_database()

    print("\n[2/4] Creating SQL datasets...")
    dataset_map = create_datasets(db_id)

    print("\n[3/4] Creating charts and dashboards...")
    all_chart_ids = {}
    all_dashboard_ids = {}

    for ddef in DASHBOARD_DEFS:
        print(f"\n  --- Dashboard: {ddef['title']} ---")
        # Create dashboard first (to get its ID)
        dash_id = None
        resp = api_get(
            "/api/v1/dashboard/",
            params={
                "q": "(filters:!((col:dashboard_title,opr:eq,value:'"
                + ddef["title"] + "')))"
            },
        )
        if resp.status_code == 200 and resp.json().get("count", 0) > 0:
            dash_id = resp.json()["result"][0]["id"]
            print(f"  Dashboard exists (id={dash_id})")
        else:
            resp = api_post("/api/v1/dashboard/", {
                "dashboard_title": ddef["title"],
                "slug": ddef["slug"],
                "owners": [1],
                "published": True,
            })
            if resp.status_code in (200, 201):
                dash_id = resp.json()["id"]
                print(f"  Dashboard created (id={dash_id})")
                time.sleep(0.3)

        all_dashboard_ids[ddef["slug"]] = dash_id

        # Create charts for this dashboard
        chart_ids_for_dash = []
        for chart_name, _, _, _, _ in ddef["charts"]:
            # Find chart def
            cd = next((c for c in CHART_DEFS if c["name"] == chart_name), None)
            if not cd:
                print(f"  SKIP unknown chart '{chart_name}'")
                chart_ids_for_dash.append(None)
                continue

            cid = create_chart(cd, dataset_map, [dash_id] if dash_id else [])
            chart_ids_for_dash.append(cid)
            all_chart_ids[chart_name] = cid

        # Update dashboard layout
        if dash_id:
            entries = []
            for i, (chart_name, x, y, w, h) in enumerate(ddef["charts"]):
                cid = all_chart_ids.get(chart_name)
                entries.append((cid, x, y, w, h))
            pos = _position_json(ddef["title"], entries)
            api_put(f"/api/v1/dashboard/{dash_id}", {"position_json": pos})
            print(f"  Dashboard layout updated ({len(ddef['charts'])} charts)")

    print("\n[4/4] Exporting dashboards...")
    export_dashboards()

    print("\n" + "=" * 55)
    print("  Setup complete!")
    print(f"  Open http://localhost:8088 and login with admin / admin")
    print("=" * 55)


if __name__ == "__main__":
    main()
