#!/bin/bash
set -euo pipefail

echo "Esperando a que los servicios de MongoDB estén listos..."

# 1. Esperar a que configsvr responda (mongod standalone, responde rapido)
for i in $(seq 1 30); do
    if docker exec configsvr mongosh --quiet --port 27019 --eval "db.adminCommand('ping')" 2>/dev/null; then
        break
    fi
    echo "  Esperando configsvr... ($i)"
    sleep 2
done

echo "Configurando Config Server..."
docker exec configsvr mongosh --port 27019 --eval "rs.initiate({_id:'cfgReplSet', configsvr:true, members:[{_id:0, host:'configsvr:27019'}]})" 2>/dev/null || echo "  -> Config Server ya inicializado"

# 2. Esperar a que mongo1 (shard) responda
for i in $(seq 1 30); do
    if docker exec mongo1 mongosh --quiet --port 27018 --eval "db.adminCommand('ping')" 2>/dev/null; then
        break
    fi
    echo "  Esperando mongo1... ($i)"
    sleep 2
done

echo "Configurando Shard 1..."
docker exec mongo1 mongosh --port 27018 --eval "rs.initiate({_id:'shard0', members:[{_id:0, host:'mongo1:27018'}, {_id:1, host:'mongo2:27018'}, {_id:2, host:'mongo3:27018'}]})" 2>/dev/null || echo "  -> Shard replicaset ya inicializado"

# Esperar a que el replicaset tenga un primary
echo "Esperando primary del shard..."
for i in $(seq 1 15); do
    primary=$(docker exec mongo1 mongosh --quiet --port 27018 --eval "rs.isMaster().ismaster" 2>/dev/null)
    if [ "$primary" = "true" ]; then
        echo "  -> Primary electo"
        break
    fi
    sleep 2
done

# 3. AHORA esperar a que mongos responda (ya configsvr esta iniciado como replicaset)
echo "Esperando a que mongos se conecte al config server..."
for i in $(seq 1 30); do
    if docker exec mongos mongosh --quiet --port 27017 --eval "db.adminCommand('ping')" 2>/dev/null; then
        break
    fi
    echo "  Esperando mongos... ($i)"
    sleep 2
done

echo "✅ Servicios listos"

echo "Agregando Shard al Mongos..."
docker exec mongos mongosh --eval "sh.addShard('shard0/mongo1:27018')" 2>/dev/null || echo "  -> Shard ya agregado"

echo "Habilitando sharding en la base de datos..."
docker exec mongos mongosh --eval "sh.enableSharding('restaurante_db')" 2>/dev/null || echo "  -> Sharding ya habilitado"

# Crear indices necesarios para sharding (por si la coleccion ya tenia datos)
echo "Creando indices para shard keys..."
docker exec mongos mongosh --quiet --eval "db.getSiblingDB('restaurante_db').platos.createIndex({ 'id_restaurante': 1 })" 2>/dev/null || true

echo "Shardeando colecciones..."
docker exec mongos mongosh --eval "sh.shardCollection('restaurante_db.pedidos', { '_id': 'hashed' })" 2>/dev/null || echo "  -> pedidos ya shardeado"
docker exec mongos mongosh --eval "sh.shardCollection('restaurante_db.platos', { 'id_restaurante': 1 })" 2>/dev/null || echo "  -> platos ya shardeado"

# Copiar y ejecutar el script de insercion
echo "Insertando datos de ejemplo..."
docker cp initMongo.js mongos:/initMongo.js
docker exec mongos mongosh --file /initMongo.js

# Reiniciar API para que tenga una conexion limpia a mongos
echo "Reiniciando API para refrescar conexión a mongos..."
docker-compose restart api
sleep 3

echo "============================================"
echo " Inicialización completada exitosamente!"
echo "============================================"
