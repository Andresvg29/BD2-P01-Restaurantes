"""
analisis_tendencias.py — Tendencias de consumo.

Carga fact_orders + dim_date + dim_product + fact_order_details desde Hive
y genera análisis de tendencias de ventas.
Output: CSV en spark/output/tendencias/
"""

from pyspark.sql import SparkSession
from pyspark.sql import functions as F
import os

OUTPUT_DIR = "/opt/spark/output/tendencias"


def create_spark():
    return SparkSession.builder \
        .appName("Analisis Tendencias") \
        .config("spark.sql.catalogImplementation", "hive") \
        .enableHiveSupport() \
        .getOrCreate()


def main():
    print("=== Análisis de Tendencias ===")
    spark = create_spark()

    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # --- 1. Ingresos por mes ---
    print("  Calculando ingresos por mes...")
    df = spark.sql("""
        SELECT
            d.year,
            d.month,
            COUNT(DISTINCT fo.order_id) AS total_pedidos,
            ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
            ROUND(AVG(fo.total_amount), 2) AS ticket_promedio
        FROM fact_orders fo
        JOIN dim_date d ON fo.date_key = d.date_key
        GROUP BY d.year, d.month
        ORDER BY d.year, d.month
    """)
    df.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/ingresos_mensuales")
    print(f"    {df.count()} registros → {OUTPUT_DIR}/ingresos_mensuales")

    # --- 2. Top 10 productos más vendidos ---
    print("  Calculando top 10 productos...")
    top = spark.sql("""
        SELECT
            dp.nombre AS producto,
            dp.categoria,
            SUM(fod.quantity) AS total_unidades,
            ROUND(SUM(fod.subtotal), 2) AS ingresos_totales,
            ROUND(AVG(fod.unit_price), 2) AS precio_promedio
        FROM fact_order_details fod
        JOIN dim_product dp ON fod.product_key = dp.product_key
        GROUP BY dp.nombre, dp.categoria
        ORDER BY ingresos_totales DESC
        LIMIT 10
    """)
    top.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/top_10_productos")
    print(f"    {top.count()} registros → {OUTPUT_DIR}/top_10_productos")

    # --- 3. Categoría más popular por trimestre ---
    print("  Calculando categorías por trimestre...")
    cats = spark.sql("""
        SELECT
            d.year,
            d.quarter,
            dp.categoria,
            SUM(fod.quantity) AS total_unidades,
            ROUND(SUM(fod.subtotal), 2) AS ingresos_totales,
            RANK() OVER (PARTITION BY d.year, d.quarter ORDER BY SUM(fod.subtotal) DESC) AS ranking
        FROM fact_order_details fod
        JOIN dim_product dp ON fod.product_key = dp.product_key
        JOIN fact_orders fo ON fod.order_id = fo.order_id
        JOIN dim_date d ON fo.date_key = d.date_key
        GROUP BY d.year, d.quarter, dp.categoria
        ORDER BY d.year, d.quarter, ranking
    """)
    cats.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/categorias_trimestre")
    print(f"    {cats.count()} registros → {OUTPUT_DIR}/categorias_trimestre")

    print("✓ Análisis de tendencias completado.")


if __name__ == "__main__":
    main()
