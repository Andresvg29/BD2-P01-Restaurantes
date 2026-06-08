"""
run_etl.py — Orquestador del pipeline ETL (ejecución local).

Ejecuta secuencialmente:
  1. export_data.py       (MongoDB/PostgreSQL → CSVs en staging/)
  2. spark-submit etl_to_star.py (Spark → Hive, opcional si Spark está disponible)

En producción, Airflow reemplaza a este orquestador.
"""

import subprocess
import sys
import os
from datetime import datetime


ETL_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.abspath(os.path.join(ETL_DIR, "..", ".."))

STEPS = [
    ("Extracción desde base de datos", "warehouse/etl/export_data.py"),
]


def log(msg):
    ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{ts}] {msg}")


def run_python_step(description, script_path):
    log(f"▶ {description}...")
    if not os.path.exists(script_path):
        log(f"ERROR: {script_path} no encontrado")
        return False
    result = subprocess.run(
        [sys.executable, script_path],
        cwd=ETL_DIR,
        capture_output=False,
    )
    if result.returncode == 0:
        log(f"✓ {description} completado")
        return True
    else:
        log(f"✗ {description} FALLÓ (código {result.returncode})")
        return False


def run_spark_job(job_name, job_path):
    """Ejecuta un job PySpark vía spark-submit (si está disponible)."""
    spark_home = os.environ.get("SPARK_HOME", "")
    if not spark_home:
        log(f"  ⚠ SPARK_HOME no definido. Omitiendo {job_name}.")
        log(f"  Ejecuta manualmente: spark-submit {job_path}")
        return True

    submit = os.path.join(spark_home, "bin", "spark-submit")
    if not os.path.exists(submit):
        log(f"  ⚠ {submit} no encontrado. Omitiendo {job_name}.")
        return True

    log(f"▶ {description}...")
    result = subprocess.run(
        [submit, "--master", "spark://spark-master:7077", job_path],
        capture_output=False,
    )
    if result.returncode == 0:
        log(f"✓ {job_name} completado")
        return True
    else:
        log(f"✗ {job_name} FALLÓ (código {result.returncode})")
        return False


def main():
    log("=== Inicio del Pipeline ETL ===")
    start = datetime.now()

    # Paso 1: Exportar datos
    script = os.path.join(PROJECT_DIR, "warehouse", "etl", "export_data.py")
    ok = run_python_step("Extracción desde base de datos", script)
    if not ok:
        log("Pipeline detenido por error en extracción.")
        sys.exit(1)

    # Paso 2: Transformar con Spark (si SPARK_HOME está disponible)
    print()
    spark_job = os.path.join(PROJECT_DIR, "spark", "etl_to_star.py")
    if os.path.exists(spark_job):
        ok = run_spark_job("Transformación a modelo estrella (Spark → Hive)", spark_job)
        if not ok:
            log("Pipeline detenido por error en transformación Spark.")
            sys.exit(1)
    else:
        log(f"  ⚠ {spark_job} no encontrado. Omitiendo transformación Spark.")

    elapsed = (datetime.now() - start).total_seconds()
    log(f"✓ Pipeline ETL completado en {elapsed:.1f}s")


if __name__ == "__main__":
    main()
