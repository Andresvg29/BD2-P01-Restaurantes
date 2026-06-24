"""
analisis_crecimiento.py — Análisis de crecimiento mensual.

Carga fact_orders + dim_date + dim_order_status desde Hive y calcula
crecimiento mes a mes con window functions LAG().
Output: CSV en spark/output/crecimiento/
"""

from pyspark.sql import SparkSession
from pyspark.sql import functions as F
import os

OUTPUT_DIR = "/opt/spark/output/crecimiento"


def create_spark():
    return SparkSession.builder \
        .appName("Analisis Crecimiento") \
        .config("spark.sql.catalogImplementation", "hive") \
        .enableHiveSupport() \
        .getOrCreate()


def main():
    print("=== Análisis de Crecimiento Mensual ===")
    spark = create_spark()

    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # --- Crecimiento mensual con LAG() ---
    print("  Calculando crecimiento mensual...")
    df = spark.sql("""
        SELECT
            d.year,
            d.month,
            COUNT(DISTINCT fo.order_id) AS total_pedidos,
            ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
            COUNT(DISTINCT fo.user_key) AS usuarios_activos,
            SUM(CASE WHEN dos.status_name IN ('Cancelado', 'Cancelled') THEN 1 ELSE 0 END) AS pedidos_cancelados,
            ROUND(
                SUM(CASE WHEN dos.status_name IN ('Cancelado', 'Cancelled') THEN 1 ELSE 0 END) * 100.0
                / NULLIF(COUNT(DISTINCT fo.order_id), 0),
                2
            ) AS tasa_cancelacion_pct,
            ROUND(
                SUM(fo.total_amount)
                - LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month),
                2
            ) AS diferencia_ingresos_mom,
            ROUND(
                (SUM(fo.total_amount) - LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month))
                * 100.0 / NULLIF(LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month), 0),
                2
            ) AS crecimiento_ingresos_pct,
            ROUND(
                (COUNT(DISTINCT fo.order_id) - LAG(COUNT(DISTINCT fo.order_id)) OVER (ORDER BY d.year, d.month))
                * 100.0 / NULLIF(LAG(COUNT(DISTINCT fo.order_id)) OVER (ORDER BY d.year, d.month), 0),
                2
            ) AS crecimiento_pedidos_pct
        FROM fact_orders fo
        JOIN dim_date d ON fo.date_key = d.date_key
        JOIN dim_order_status dos ON fo.status_key = dos.status_key
        GROUP BY d.year, d.month
        ORDER BY d.year, d.month
    """)
    df.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/crecimiento_mensual")
    total = df.count()
    print(f"    {total} registros → {OUTPUT_DIR}/crecimiento_mensual")

    # --- Resumen: mes con mayor crecimiento ---
    print("  Calculando resumen de crecimiento...")
    summary = spark.sql("""
        SELECT
            'Mayor crecimiento' AS metrica,
            CONCAT(year, '-', LPAD(month, 2, '0')) AS mes,
            ROUND(MAX(crecimiento_ingresos_pct), 2) AS valor
        FROM (
            SELECT
                d.year, d.month,
                (SUM(fo.total_amount) - LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month))
                * 100.0 / NULLIF(LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month), 0) AS crecimiento_ingresos_pct
            FROM fact_orders fo
            JOIN dim_date d ON fo.date_key = d.date_key
            GROUP BY d.year, d.month
        ) t
        WHERE crecimiento_ingresos_pct IS NOT NULL
        GROUP BY year, month
        ORDER BY valor DESC
        LIMIT 1
    """)
    summary.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/resumen_crecimiento")
    print(f"    → {OUTPUT_DIR}/resumen_crecimiento")

    print("✓ Análisis de crecimiento completado.")


if __name__ == "__main__":
    main()
