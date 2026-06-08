CREATE TABLE IF NOT EXISTS dim_reservation_status (
    status_key   INT
  , status_name  STRING
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
TBLPROPERTIES ("skip.header.line.count"="1");
