CREATE TABLE IF NOT EXISTS fact_orders (
    order_id        STRING
  , date_key        INT
  , user_key        INT
  , restaurant_key  INT
  , status_key      INT
  , total_amount    DECIMAL(10,2)
  , item_count      INT
)
PARTITIONED BY (year INT, month INT)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
TBLPROPERTIES ("skip.header.line.count"="1");
