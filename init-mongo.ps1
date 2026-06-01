Write-Host "Esperando a que los servicios de MongoDB esten listos..." -ForegroundColor Cyan

# 1. Esperar a que configsvr responda (mongod standalone, responde rapido)
for ($i = 0; $i -lt 30; $i++) {
    $result = docker exec configsvr mongosh --quiet --port 27019 --eval "db.adminCommand('ping')" 2>$null
    if ($result) { break }
    Write-Host "  Esperando configsvr... ($($i+1))"
    Start-Sleep -Seconds 2
}

Write-Host "Configurando Config Server..."
docker exec configsvr mongosh --port 27019 --eval "rs.initiate({_id:'cfgReplSet', configsvr:true, members:[{_id:0, host:'configsvr:27019'}]})" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "  -> Config Server ya inicializado" }

# 2. Esperar a que mongo1 (shard) responda
for ($i = 0; $i -lt 30; $i++) {
    $result = docker exec mongo1 mongosh --quiet --port 27018 --eval "db.adminCommand('ping')" 2>$null
    if ($result) { break }
    Write-Host "  Esperando mongo1... ($($i+1))"
    Start-Sleep -Seconds 2
}

Write-Host "Configurando Shard 1..."
docker exec mongo1 mongosh --port 27018 --eval "rs.initiate({_id:'shard0', members:[{_id:0, host:'mongo1:27018'}, {_id:1, host:'mongo2:27018'}, {_id:2, host:'mongo3:27018'}]})" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "  -> Shard replicaset ya inicializado" }

# 3. Esperar a que el replicaset tenga un primary
Write-Host "Esperando primary del shard..."
for ($i = 0; $i -lt 15; $i++) {
    $primary = docker exec mongo1 mongosh --quiet --port 27018 --eval "rs.isMaster().ismaster" 2>$null
    if ($primary -eq "true") {
        Write-Host "  -> Primary electo"
        break
    }
    Start-Sleep -Seconds 2
}

# 4. AHORA esperar a que mongos responda (ya configsvr esta iniciado)
Write-Host "Esperando a que mongos se conecte al config server..."
for ($i = 0; $i -lt 30; $i++) {
    $result = docker exec mongos mongosh --quiet --port 27017 --eval "db.adminCommand('ping')" 2>$null
    if ($result) { break }
    Write-Host "  Esperando mongos... ($($i+1))"
    Start-Sleep -Seconds 2
}

Write-Host "Servicios listos" -ForegroundColor Green

Write-Host "Agregando Shard al Mongos..."
docker exec mongos mongosh --eval "sh.addShard('shard0/mongo1:27018')" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "  -> Shard ya agregado" }

Write-Host "Habilitando sharding en la base de datos..."
docker exec mongos mongosh --eval "sh.enableSharding('restaurante_db')" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "  -> Sharding ya habilitado" }

# Crear indices necesarios para sharding
Write-Host "Creando indices para shard keys..."
docker exec mongos mongosh --quiet --eval "db.getSiblingDB('restaurante_db').platos.createIndex({ 'id_restaurante': 1 })" 2>$null

Write-Host "Shardeando colecciones..."
docker exec mongos mongosh --eval "sh.shardCollection('restaurante_db.pedidos', { '_id': 'hashed' })" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "  -> pedidos ya shardeado" }

docker exec mongos mongosh --eval "sh.shardCollection('restaurante_db.platos', { 'id_restaurante': 1 })" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "  -> platos ya shardeado" }

# Copiar y ejecutar el script de insercion
Write-Host "Insertando datos de ejemplo..." -ForegroundColor Cyan
docker cp initMongo.js mongos:/initMongo.js
docker exec mongos mongosh --file /initMongo.js

# Reiniciar API para que tenga una conexion limpia a mongos
Write-Host "Reiniciando API para refrescar conexion a mongos..." -ForegroundColor Cyan
docker-compose restart api
Start-Sleep -Seconds 3

Write-Host "============================================" -ForegroundColor Green
Write-Host " Inicializacion completada exitosamente!" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
