docker exec -it configsvr mongosh --port 27019 --eval "rs.initiate({_id:'cfgReplSet', configsvr:true, members:[{_id:0, host:'configsvr:27019'}]})"
docker exec -it mongo1 mongosh --port 27018 --eval "rs.initiate({_id:'shard0', members:[{_id:0, host:'mongo1:27018'}, {_id:1, host:'mongo2:27018'}, {_id:2, host:'mongo3:27018'}]})"
sleep 4
docker exec -it mongos mongosh --port 27017 --eval "sh.addShard('shard0/mongo1:27018')"
docker exec mongos mongosh --eval "sh.enableSharding('restaurante_db')"
docker exec mongos mongosh --eval "sh.shardCollection('restaurante_db.pedidos', { '_id': 'hashed' })"
docker exec mongos mongosh --eval "sh.shardCollection('restaurante_db.platos', { 'id_restaurante': 1 })"