// Constraints
CREATE CONSTRAINT unique_user IF NOT EXISTS FOR (u:User) REQUIRE u.user_id IS UNIQUE;
CREATE CONSTRAINT unique_product IF NOT EXISTS FOR (p:Product) REQUIRE p.product_id IS UNIQUE;
CREATE CONSTRAINT unique_restaurant IF NOT EXISTS FOR (r:Restaurant) REQUIRE r.restaurant_id IS UNIQUE;
CREATE CONSTRAINT unique_order IF NOT EXISTS FOR (o:Order) REQUIRE o.order_id IS UNIQUE;
CREATE CONSTRAINT unique_reservation IF NOT EXISTS FOR (r:Reservation) REQUIRE r.reservation_id IS UNIQUE;

CREATE INDEX idx_user_email IF NOT EXISTS FOR (u:User) ON (u.email);
CREATE INDEX idx_product_categoria IF NOT EXISTS FOR (p:Product) ON (p.categoria);
CREATE INDEX idx_restaurant_nombre IF NOT EXISTS FOR (r:Restaurant) ON (r.nombre);

// Nodes
LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/users.csv' AS row
MERGE (u:User {user_id: row.`user_id:ID(User)`})
SET u.nombre = row.nombre, u.email = row.email;

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/products.csv' AS row
MERGE (p:Product {product_id: row.`product_id:ID(Product)`})
SET p.nombre = row.nombre, p.categoria = row.categoria, p.precio = toFloat(row.`precio:float`);

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/restaurants.csv' AS row
MERGE (r:Restaurant {restaurant_id: row.`restaurant_id:ID(Restaurant)`})
SET r.nombre = row.nombre, r.direccion = row.direccion;

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/orders.csv' AS row
MERGE (o:Order {order_id: row.`order_id:ID(Order)`})
SET o.total_amount = toFloat(row.`total_amount:float`), o.fecha = row.fecha;

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/reservations.csv' AS row
MERGE (r:Reservation {reservation_id: row.`reservation_id:ID(Reservation)`})
SET r.fecha = row.fecha, r.party_size = toInteger(row.`party_size:int`);

// Relationships
LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/placed.csv' AS row
MATCH (u:User {user_id: row.`:START_ID(User)`})
MATCH (o:Order {order_id: row.`:END_ID(Order)`})
MERGE (u)-[:PLACED {fecha: row.fecha}]->(o);

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/includes.csv' AS row
MATCH (o:Order {order_id: row.`:START_ID(Order)`})
MATCH (p:Product {product_id: row.`:END_ID(Product)`})
MERGE (o)-[:INCLUDES {cantidad: toInteger(row.`cantidad:int`), subtotal: toFloat(row.`subtotal:float`)}]->(p);

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/at_order.csv' AS row
MATCH (o:Order {order_id: row.`:START_ID(Order)`})
MATCH (r:Restaurant {restaurant_id: row.`:END_ID(Restaurant)`})
MERGE (o)-[:AT]->(r);

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/made_reservation.csv' AS row
MATCH (u:User {user_id: row.`:START_ID(User)`})
MATCH (r:Reservation {reservation_id: row.`:END_ID(Reservation)`})
MERGE (u)-[:MADE_RESERVATION]->(r);

LOAD CSV WITH HEADERS FROM 'file:///warehouse-import/at_reservation.csv' AS row
MATCH (r:Reservation {reservation_id: row.`:START_ID(Reservation)`})
MATCH (rest:Restaurant {restaurant_id: row.`:END_ID(Restaurant)`})
MERGE (r)-[:AT]->(rest);
