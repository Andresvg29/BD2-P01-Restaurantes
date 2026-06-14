"""
analisis_horarios.py — Análisis de horarios pico.

Carga fact_orders + dim_date desde Hive, agrupa por hora y día de semana
para identificar ventanas de alta demanda.
Output: CSV en spark/output/horarios/
"""

from pyspark.sql import SparkSession
from pyspark.sql import functions as F
import os

OUTPUT_DIR = "/opt/bitnami/spark/output/horarios"


def create_spark():
    return SparkSession.builder \
        .appName("Analisis Horarios") \
        .config("spark.sql.catalogImplementation", "hive") \
        .enableHiveSupport() \
        .getOrCreate()


def main():
    print("=== Análisis de Horarios Pico ===")
    spark = create_spark()

    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # --- 1. Pedidos por hora y día de semana ---
    print("  Calculando pedidos por hora y día de semana...")
    df = spark.sql("""
        SELECT
            d.weekday,
            d.is_weekend,
            fo.hora,
            COUNT(DISTINCT fo.order_id) AS total_pedidos,
            ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
            COUNT(DISTINCT fo.user_key) AS usuarios_unicos,
            RANK() OVER (PARTITION BY d.weekday ORDER BY COUNT(DISTINCT fo.order_id) DESC) AS ranking_hora
        FROM fact_orders fo
        JOIN dim_date d ON fo.date_key = d.date_key
        GROUP BY d.weekday, d.is_weekend, fo.hora
        ORDER BY d.weekday, ranking_hora
    """)
    df.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/horarios_pico")
    total = df.count()
    print(f"    {total} registros → {OUTPUT_DIR}/horarios_pico")

    # --- 2. Ventanas de alta demanda (horas con más pedidos) ---
    print("  Identificando ventanas de alta demanda...")
    peak = spark.sql("""
        SELECT
            weekday,
            collect_list(CASE WHEN ranking_hora <= 3 THEN hora END) AS horas_pico,
            MAX(total_pedidos) AS max_pedidos
        FROM (
            SELECT
                d.weekday,
                fo.hora,
                COUNT(DISTINCT fo.order_id) AS total_pedidos,
                RANK() OVER (PARTITION BY d.weekday ORDER BY COUNT(DISTINCT fo.order_id) DESC) AS ranking_hora
            FROM fact_orders fo
            JOIN dim_date d ON fo.date_key = d.date_key
            GROUP BY d.weekday, fo.hora
        ) t
        WHERE ranking_hora <= 3
        GROUP BY weekday
        ORDER BY max_pedidos DESC
    """)
    peak.coalesce(1).write.mode("overwrite").option("header", True).csv(f"{OUTPUT_DIR}/ventanas_alta_demanda")
    print(f"    → {OUTPUT_DIR}/ventanas_alta_demanda")

    print("✓ Análisis de horarios completado.")


if __name__ == "__main__":
    main()
