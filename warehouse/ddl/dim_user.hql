CREATE TABLE IF NOT EXISTS dim_user (
    user_key    INT
  , user_id     STRING
  , nombre      STRING
  , email       STRING
  , role_name   STRING
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
LOCATION 'hdfs://hadoop-namenode:9000/user/hive/warehouse/dim_user'
TBLPROPERTIES ("skip.header.line.count"="1");
