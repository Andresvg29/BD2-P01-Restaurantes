CREATE TABLE IF NOT EXISTS dim_order_status (
    status_key   INT
  , status_name  STRING
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_order_status'
TBLPROPERTIES ("skip.header.line.count"="1");

INSERT INTO dim_order_status VALUES (1, 'completado'), (2, 'cancelado');
