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
TBLPROPERTIES ("skip.header.line.count"="1");
