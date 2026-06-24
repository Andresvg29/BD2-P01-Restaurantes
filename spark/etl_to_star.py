"""
etl_to_star.py — Transforma CSVs a modelo estrella y escribe en Hive.

Lee CSVs de staging/, genera dimensiones con surrogate keys,
mapea fact tables y escribe directamente en Hive.
Ejecutar con: spark-submit --master spark://spark-master:7077 spark/etl_to_star.py
"""

from pyspark.sql import SparkSession
from pyspark.sql import functions as F
from pyspark.sql import types as T
from pyspark.sql.window import Window
import os
import sys

STAGING_DIR = "/opt/staging"


def create_spark():
    return SparkSession.builder \
        .appName("ETL to Star Schema") \
        .config("spark.sql.catalogImplementation", "hive") \
        .config("hive.metastore.uris", "thrift://hive-metastore:9083") \
        .enableHiveSupport() \
        .getOrCreate()


def read_csv(spark, filename):
    path = os.path.join(STAGING_DIR, filename)
    if not os.path.exists(path):
        print(f"  ⚠ {filename} no encontrado, saltando.")
        return None
    return spark.read.option("header", True).option("inferSchema", True).csv(path)


# ── Dimensiones ──────────────────────────────────────────────────────────────

def build_dim_user(spark, df):
    window_spec = Window.orderBy("id")
    role_map = F.create_map(F.lit("1"), F.lit("admin"), F.lit("2"), F.lit("client"))
    return df.select(
        F.row_number().over(window_spec).alias("user_key"),
        F.col("id").alias("user_id"),
        "nombre", "email",
        role_map[F.col("id_rol")].alias("role_name")
    )


def build_dim_restaurant(spark, df):
    window_spec = Window.orderBy("id")
    return df.select(
        F.row_number().over(window_spec).alias("restaurant_key"),
        F.col("id").alias("restaurant_id"),
        "nombre", "direccion", "ciudad", "telefono"
    )


def build_dim_product(spark, df):
    window_spec = Window.orderBy("id")
    return df.select(
        F.row_number().over(window_spec).alias("product_key"),
        F.col("id").alias("product_id"),
        "nombre", "categoria",
        F.col("precio").cast("decimal(10,2)").alias("precio")
    )


def build_dim_date(spark, pedidos_df, reservas_df):
    dates = spark.createDataFrame([], T.StructType([
        T.StructField("full_date", T.DateType())
    ]))
    if pedidos_df is not None:
        d = pedidos_df.select(F.to_date("fecha_creacion").alias("full_date")).filter(F.col("full_date").isNotNull())
        dates = dates.union(d)
    if reservas_df is not None:
        d = reservas_df.select(F.to_date("fecha_reserva").alias("full_date")).filter(F.col("full_date").isNotNull())
        dates = dates.union(d)
    dates = dates.distinct()
    if dates.count() == 0:
        return None
    return dates.select(
        (F.year("full_date") * 10000 + F.month("full_date") * 100 + F.dayofmonth("full_date")).alias("date_key"),
        "full_date",
        F.year("full_date").alias("year"),
        F.quarter("full_date").alias("quarter"),
        F.month("full_date").alias("month"),
        F.dayofmonth("full_date").alias("day"),
        F.date_format("full_date", "EEEE").alias("weekday"),
        (F.dayofweek("full_date").isin([1, 7])).alias("is_weekend"),
    ).orderBy("date_key")


def build_dim_order_status(spark):
    data = [(1, "pendiente"), (2, "en_preparacion"), (3, "completado"), (4, "entregado")]
    return spark.createDataFrame(data, schema=["status_key", "status_name"])


def build_dim_reservation_status(spark):
    data = [(1, "pendiente"), (2, "confirmada"), (3, "cancelada")]
    return spark.createDataFrame(data, schema=["status_key", "status_name"])


# ── Hechos ───────────────────────────────────────────────────────────────────

def build_fact_orders(spark, pedidos_df, detalles_df, dim_date, user_map, rest_map, status_df):
    item_counts = detalles_df.groupBy("id_pedido").agg(
        F.sum(F.col("cantidad").cast("int")).alias("item_count")
    )
    base = pedidos_df.select(
        F.col("id").alias("order_id"),
        F.to_date("fecha_creacion").alias("order_date"),
        F.hour("fecha_creacion").alias("hora"),
        "id_usuario", "id_restaurante",
        F.col("id_estado_pedido").alias("status_id"),
        F.col("total").cast("decimal(10,2)").alias("total_amount"),
        F.year("fecha_creacion").alias("year"),
        F.month("fecha_creacion").alias("month"),
    )
    result = base.join(item_counts, base["order_id"] == item_counts["id_pedido"], "left") \
        .drop("id_pedido") \
        .join(F.broadcast(dim_date.select("full_date", "date_key")),
              base["order_date"] == dim_date["full_date"], "left") \
        .drop(dim_date["full_date"]) \
        .join(F.broadcast(user_map), base["id_usuario"] == user_map["user_id"], "left") \
        .drop("id_usuario", "user_id") \
        .join(F.broadcast(rest_map), base["id_restaurante"] == rest_map["restaurant_id"], "left") \
        .drop("id_restaurante", "restaurant_id")
    return result.select(
        "order_id", "date_key",
        F.col("hora").cast("int").alias("hora"),
        "user_key", "restaurant_key",
        F.col("status_id").cast("int").alias("status_key"),
        "total_amount",
        F.col("item_count").cast("int").alias("item_count"),
        "year", "month"
    )


def build_fact_order_details(spark, detalles_df, pedidos_df, product_map):
    base = detalles_df.select(
        F.col("id").alias("detail_id"),
        F.col("id_pedido").alias("order_id"),
        F.col("id_plato").alias("product_id"),
        F.col("cantidad").cast("int").alias("quantity"),
        F.col("precio").cast("decimal(10,2)").alias("unit_price"),
    )
    result = base.withColumn("subtotal", F.col("quantity") * F.col("unit_price"))
    pm = product_map.withColumnRenamed("product_id", "pm_product_id")
    result = result.join(
        F.broadcast(pm), result["product_id"] == pm["pm_product_id"], "left"
    ).drop("product_id", "pm_product_id")
    order_dates = pedidos_df.select(
        F.col("id").alias("oid"),
        F.year("fecha_creacion").alias("year"),
        F.month("fecha_creacion").alias("month"),
    )
    result = result.join(order_dates, result["order_id"] == order_dates["oid"], "left") \
        .drop("oid")
    return result.select("detail_id", "order_id", "product_key", "quantity", "unit_price", "subtotal", "year", "month")


def build_fact_reservations(spark, reservas_df, dim_date, user_map, rest_map, status_df):
    base = reservas_df.select(
        F.col("id").alias("reservation_id"),
        F.to_date("fecha_reserva").alias("res_date"),
        F.col("cantidad_personas").cast("int").alias("party_size"),
        "id_usuario", "id_restaurante",
        F.col("id_estado").alias("status_id"),
        F.year("fecha_reserva").alias("year"),
        F.month("fecha_reserva").alias("month"),
    )
    result = base \
        .join(F.broadcast(dim_date.select("full_date", "date_key")),
              base["res_date"] == dim_date["full_date"], "left") \
        .drop(dim_date["full_date"]) \
        .join(F.broadcast(user_map), base["id_usuario"] == user_map["user_id"], "left") \
        .drop("id_usuario", "user_id") \
        .join(F.broadcast(rest_map), base["id_restaurante"] == rest_map["restaurant_id"], "left") \
        .drop("id_restaurante", "restaurant_id")
    return result.select(
        "reservation_id", "date_key", "user_key", "restaurant_key",
        F.col("status_id").cast("int").alias("status_key"),
        "party_size", "year", "month"
    )


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    print("=== ETL: Transformación a modelo estrella ===")
    spark = create_spark()

    # 1. Leer CSVs
    usuarios = read_csv(spark, "usuarios.csv")
    restaurantes = read_csv(spark, "restaurantes.csv")
    platos = read_csv(spark, "platos.csv")
    pedidos = read_csv(spark, "pedidos.csv")
    detalles = read_csv(spark, "detalles_pedido.csv")
    reservas = read_csv(spark, "reservas.csv")

    if not any([usuarios, restaurantes, platos, pedidos, detalles, reservas]):
        print("  No hay datos para procesar. Abortando.")
        sys.exit(1)

    # 2. Dimensiones
    print("  Generando dim_user...")
    dim_user = build_dim_user(spark, usuarios) if usuarios else None
    print("  Generando dim_restaurant...")
    dim_restaurant = build_dim_restaurant(spark, restaurantes) if restaurantes else None
    print("  Generando dim_product...")
    dim_product = build_dim_product(spark, platos) if platos else None
    print("  Generando dim_date...")
    dim_date = build_dim_date(spark, pedidos, reservas)
    print("  Generando dim_order_status...")
    dim_order_status = build_dim_order_status(spark)
    print("  Generando dim_reservation_status...")
    dim_reservation_status = build_dim_reservation_status(spark)

    # 3. Broadcast mappings
    user_map = dim_user.select("user_id", "user_key") if dim_user else None
    rest_map = dim_restaurant.select("restaurant_id", "restaurant_key") if dim_restaurant else None
    product_map = dim_product.select("product_id", "product_key") if dim_product else None

    # 4. Fact tables
    if pedidos and dim_date and user_map and rest_map:
        print("  Generando fact_orders...")
        fact_orders = build_fact_orders(spark, pedidos, detalles, dim_date, user_map, rest_map, dim_order_status)
        print(f"    Registros: {fact_orders.count()}")
        fact_orders.write.mode("overwrite").partitionBy("year", "month").saveAsTable("fact_orders")
        print("    ✓ Escrito en Hive: fact_orders")

    if detalles and pedidos and product_map:
        print("  Generando fact_order_details...")
        fact_details = build_fact_order_details(spark, detalles, pedidos, product_map)
        print(f"    Registros: {fact_details.count()}")
        fact_details.write.mode("overwrite").partitionBy("year", "month").saveAsTable("fact_order_details")
        print("    ✓ Escrito en Hive: fact_order_details")

    if reservas and dim_date and user_map and rest_map:
        print("  Generando fact_reservations...")
        fact_reservations = build_fact_reservations(spark, reservas, dim_date, user_map, rest_map, dim_reservation_status)
        print(f"    Registros: {fact_reservations.count()}")
        fact_reservations.write.mode("overwrite").partitionBy("year", "month").saveAsTable("fact_reservations")
        print("    ✓ Escrito en Hive: fact_reservations")

    # 5. Escribir dimensiones
    for name, df in [("dim_user", dim_user), ("dim_restaurant", dim_restaurant),
                     ("dim_product", dim_product), ("dim_date", dim_date),
                     ("dim_order_status", dim_order_status),
                     ("dim_reservation_status", dim_reservation_status)]:
        if df is not None:
            df.write.mode("overwrite").saveAsTable(name)
            print(f"  ✓ Escrito en Hive: {name}")

    spark.sql("MSCK REPAIR TABLE fact_orders")
    spark.sql("MSCK REPAIR TABLE fact_order_details")
    spark.sql("MSCK REPAIR TABLE fact_reservations")

    print("✓ ETL completado exitosamente.")


if __name__ == "__main__":
    main()
