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
TBLPROPERTIES ("skip.header.line.count"="1");
