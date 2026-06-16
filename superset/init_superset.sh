#!/bin/bash
set -e

echo "=== Superset Initialization ==="

superset fab create-admin \
    --username admin \
    --firstname Admin \
    --lastname User \
    --email admin@restaurante.com \
    --password admin

superset db upgrade

superset init

echo "=== Superset initialization complete ==="
echo "Login at http://localhost:8088 with admin / admin"
