CREATE TABLE IF NOT EXISTS dim_restaurant (
    restaurant_key  INT
  , restaurant_id   STRING
  , nombre          STRING
  , direccion       STRING
  , ciudad          STRING
  , telefono        STRING
)
ROW FORMAT DELIMITED
FIELDS TERMINATED BY ','
STORED AS TEXTFILE
TBLPROPERTIES ("skip.header.line.count"="1");
