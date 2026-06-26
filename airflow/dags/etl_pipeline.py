"""
etl_pipeline.py — DAG de ETL completo para restaurantes.

Ejecución diaria:
  1. extract_data          → export_data.py (MongoDB/PostgreSQL → CSVs)
  2. export_neo4j          → export_neo4j.py (CSVs para Neo4j)
  3. load_neo4j            → load_neo4j.py (carga en Neo4j via HTTP API)
  4. etl_to_star           → Spark: transformación a modelo estrella → Hive
  5. analisis_tendencias   → Spark: tendencias de consumo (paralelo)
  6. analisis_horarios     → Spark: horarios pico (paralelo)
  7. analisis_crecimiento  → Spark: crecimiento mensual (paralelo)
"""


from airflow import DAG
from airflow.operators.bash import BashOperator
from datetime import datetime, timedelta
import os

SPARK_SUBMIT = (
    "docker exec spark-master /opt/spark/bin/spark-submit"
    " --master spark://spark-master:7077"
    " --conf spark.hadoop.hive.metastore.uris=thrift://hive-metastore:9083"
)

default_args = {
    "owner": "airflow",
    "depends_on_past": False,
    "email_on_failure": False,
    "email_on_retry": False,
    "retries": 0,
    "retry_delay": timedelta(minutes=5),
}

with DAG(
    "etl_pipeline",
    default_args=default_args,
    description="Pipeline ETL diario: extraer → transformar → analizar",
    schedule="@daily",
    start_date=datetime(2024, 1, 1),
    catchup=False,
    max_active_runs=1,
    tags=["etl", "restaurantes"],
    doc_md=__doc__,
) as dag:

    extract_data = BashOperator(
        task_id="extract_data",
        bash_command="python /opt/warehouse/etl/export_data.py",
        cwd="/opt/warehouse/etl",
    )

    export_neo4j = BashOperator(
        task_id="export_neo4j",
        bash_command="rm -rf staging/neo4j && python /opt/warehouse/etl/export_neo4j.py",
        cwd="/opt/warehouse/etl",
        env={
            "OUTPUT_DIR": "staging/neo4j",
            "DB_ENGINE": os.environ.get("DB_ENGINE", "postgres"),
            "POSTGRES_HOST": os.environ.get("POSTGRES_HOST", "db"),
            "POSTGRES_PORT": os.environ.get("POSTGRES_PORT", "5432"),
            "POSTGRES_USER": os.environ.get("POSTGRES_USER", ""),
            "POSTGRES_PASSWORD": os.environ.get("POSTGRES_PASSWORD", ""),
            "POSTGRES_DB": os.environ.get("POSTGRES_DB", ""),
            "MONGO_URI": os.environ.get("MONGO_URI", ""),
            "MONGO_DB": os.environ.get("MONGO_DB", ""),
        },
    )

    load_neo4j = BashOperator(
        task_id="load_neo4j",
        bash_command="python /opt/warehouse/etl/load_neo4j.py",
        cwd="/opt/warehouse/etl",
    )

    etl_to_star = BashOperator(
        task_id="etl_to_star",
        bash_command=f"{SPARK_SUBMIT} /opt/scripts/etl_to_star.py",
    )

    analisis_tendencias = BashOperator(
        task_id="analisis_tendencias",
        bash_command=f"{SPARK_SUBMIT} /opt/scripts/analisis_tendencias.py",
    )

    analisis_horarios = BashOperator(
        task_id="analisis_horarios",
        bash_command=f"{SPARK_SUBMIT} /opt/scripts/analisis_horarios.py",
    )

    analisis_crecimiento = BashOperator(
        task_id="analisis_crecimiento",
        bash_command=f"{SPARK_SUBMIT} /opt/scripts/analisis_crecimiento.py",
    )

    extract_data >> export_neo4j >> load_neo4j >> etl_to_star
    etl_to_star >> [analisis_tendencias, analisis_horarios, analisis_crecimiento]
