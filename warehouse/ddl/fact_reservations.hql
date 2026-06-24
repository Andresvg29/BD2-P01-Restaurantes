CREATE TABLE IF NOT EXISTS fact_reservations (
    reservation_id  STRING
  , date_key        INT
  , user_key        INT
  , restaurant_key  INT
  , status_key      INT
  , party_size      INT
)
PARTITIONED BY (year INT, month INT)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/fact_reservations'
TBLPROPERTIES ("skip.header.line.count"="1");
