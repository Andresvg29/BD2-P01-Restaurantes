"""
etl_pipeline.py — DAG de ETL completo para restaurantes.

Ejecución diaria:
  1. extract_data          → export_data.py (MongoDB/PostgreSQL → CSVs)
  2. etl_to_star           → Spark: transformación a modelo estrella → Hive
  3. analisis_tendencias   → Spark: tendencias de consumo (paralelo)
  4. analisis_horarios     → Spark: horarios pico (paralelo)
  5. analisis_crecimiento  → Spark: crecimiento mensual (paralelo)
  6. reindex_elasticsearch → HTTP POST condicional (solo si platos.csv cambió)
"""

from airflow import DAG
from airflow.operators.bash import BashOperator
from airflow.providers.apache.spark.operators.spark_submit import SparkSubmitOperator
from airflow.operators.python import BranchPythonOperator, PythonOperator
from datetime import datetime, timedelta
import hashlib
import os
import requests

default_args = {
    "owner": "airflow",
    "depends_on_past": False,
    "email_on_failure": False,
    "email_on_retry": False,
    "retries": 2,
    "retry_delay": timedelta(minutes=5),
}

STAGING_DIR = "/opt/warehouse/etl/staging"
PLATOS_CSV = os.path.join(STAGING_DIR, "platos.csv")
HASH_FILE = os.path.join(STAGING_DIR, ".last_platos_hash")

with DAG(
    "etl_pipeline",
    default_args=default_args,
    description="Pipeline ETL diario: extraer → transformar → analizar → reindexar",
    schedule="@daily",
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["etl", "restaurantes"],
    doc_md=__doc__,
) as dag:

    extract_data = BashOperator(
        task_id="extract_data",
        bash_command="python /opt/warehouse/etl/export_data.py",
        cwd="/opt/warehouse/etl",
    )

    etl_to_star = SparkSubmitOperator(
        task_id="etl_to_star",
        application="/opt/scripts/etl_to_star.py",
        conn_id="spark_default",
        verbose=True,
        name="ETL-to-Star",
        conf={
            "spark.hadoop.hive.metastore.uris": "thrift://hive-metastore:9083",
        },
    )

    analisis_tendencias = SparkSubmitOperator(
        task_id="analisis_tendencias",
        application="/opt/scripts/analisis_tendencias.py",
        conn_id="spark_default",
        verbose=True,
        name="Analisis-Tendencias",
        conf={
            "spark.hadoop.hive.metastore.uris": "thrift://hive-metastore:9083",
        },
    )

    analisis_horarios = SparkSubmitOperator(
        task_id="analisis_horarios",
        application="/opt/scripts/analisis_horarios.py",
        conn_id="spark_default",
        verbose=True,
        name="Analisis-Horarios",
        conf={
            "spark.hadoop.hive.metastore.uris": "thrift://hive-metastore:9083",
        },
    )

    analisis_crecimiento = SparkSubmitOperator(
        task_id="analisis_crecimiento",
        application="/opt/scripts/analisis_crecimiento.py",
        conn_id="spark_default",
        verbose=True,
        name="Analisis-Crecimiento",
        conf={
            "spark.hadoop.hive.metastore.uris": "thrift://hive-metastore:9083",
        },
    )

    def _should_reindex(**context):
        if not os.path.exists(PLATOS_CSV):
            return "skip_reindex"

        md5 = hashlib.md5()
        with open(PLATOS_CSV, "rb") as f:
            for chunk in iter(lambda: f.read(8192), b""):
                md5.update(chunk)
        current_hash = md5.hexdigest()

        previous_hash = ""
        if os.path.exists(HASH_FILE):
            with open(HASH_FILE) as f:
                previous_hash = f.read().strip()

        if current_hash == previous_hash:
            return "skip_reindex"

        with open(HASH_FILE, "w") as f:
            f.write(current_hash)

        return "reindex_elasticsearch"

    check_reindex = BranchPythonOperator(
        task_id="check_reindex",
        python_callable=_should_reindex,
    )

    def _do_reindex(**context):
        resp = requests.post(
            "http://search:8001/search/reindex",
            timeout=60,
        )
        resp.raise_for_status()
        result = resp.json()
        print(f"Reindexación exitosa: {result}")
        return result

    reindex_elasticsearch = PythonOperator(
        task_id="reindex_elasticsearch",
        python_callable=_do_reindex,
    )

    def _skip_reindex(**context):
        print("Reindex omitido: no hay cambios en platos.csv")

    skip_reindex = PythonOperator(
        task_id="skip_reindex",
        python_callable=_skip_reindex,
    )

    extract_data >> etl_to_star
    etl_to_star >> [analisis_tendencias, analisis_horarios, analisis_crecimiento]
    [analisis_tendencias, analisis_horarios, analisis_crecimiento] >> check_reindex
    check_reindex >> [reindex_elasticsearch, skip_reindex]
