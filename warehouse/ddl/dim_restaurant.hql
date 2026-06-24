CREATE TABLE IF NOT EXISTS dim_restaurant (
    restaurant_key  INT
  , restaurant_id   STRING
  , nombre          STRING
  , direccion       STRING
  , ciudad          STRING
  , telefono        STRING
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_restaurant'
TBLPROPERTIES ("skip.header.line.count"="1");
