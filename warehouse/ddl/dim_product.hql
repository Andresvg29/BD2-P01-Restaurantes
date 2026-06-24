CREATE TABLE IF NOT EXISTS dim_product (
    product_key   INT
  , product_id    STRING
  , nombre        STRING
  , categoria     STRING
  , precio        DECIMAL(10,2)
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_product'
TBLPROPERTIES ("skip.header.line.count"="1");
