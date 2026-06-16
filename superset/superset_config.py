import os

SECRET_KEY = os.environ.get("SUPERSET_SECRET_KEY", "cambiame_en_produccion")

SQLALCHEMY_DATABASE_URI = os.environ.get(
    "SUPERSET_DB_URI",
    "postgresql://superset:superset@superset-db:5432/superset"
)

FEATURE_FLAGS = {
    "ALERT_REPORTS": True,
}

# Hive connection (add via UI at Settings > Database Connections)
#   URI: hive://hive-server:10000/default
#
# Spark Thrift Server (future, if configured):
#   URI: hive://spark-master:10000/default
