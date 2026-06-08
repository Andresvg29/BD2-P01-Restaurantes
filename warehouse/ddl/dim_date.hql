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
TBLPROPERTIES ("skip.header.line.count"="1");
