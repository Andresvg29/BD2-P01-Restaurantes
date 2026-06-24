CREATE TABLE IF NOT EXISTS fact_order_details (
    detail_id     STRING
  , order_id      STRING
  , product_key   INT
  , quantity      INT
  , unit_price    DECIMAL(10,2)
  , subtotal      DECIMAL(10,2)
)
PARTITIONED BY (year INT, month INT)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/fact_order_details'
TBLPROPERTIES ("skip.header.line.count"="1");
