CREATE TABLE IF NOT EXISTS dim_date (
    date_key        INT
  , full_date       DATE
  , year            INT
  , quarter         INT
  , month           INT
  , day             INT
  , weekday         STRING
  , is_weekend      BOOLEAN
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_date'
TBLPROPERTIES ("skip.header.line.count"="1");
