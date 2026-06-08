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
TBLPROPERTIES ("skip.header.line.count"="1");
