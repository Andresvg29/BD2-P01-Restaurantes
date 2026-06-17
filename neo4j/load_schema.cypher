// ============================================================
// load_schema.cypher
// Crea constraints, indices y carga CSVs desde staging/
// Ejecutar via: cypher-shell -u neo4j -p password -f load_schema.cypher
// ============================================================

// ---------- CONSTRAINTS (unicidad + indices) ----------

CREATE CONSTRAINT unique_user IF NOT EXISTS FOR (u:User) REQUIRE u.user_id IS UNIQUE;
CREATE CONSTRAINT unique_product IF NOT EXISTS FOR (p:Product) REQUIRE p.product_id IS UNIQUE;
CREATE CONSTRAINT unique_restaurant IF NOT EXISTS FOR (r:Restaurant) REQUIRE r.restaurant_id IS UNIQUE;
CREATE CONSTRAINT unique_order IF NOT EXISTS FOR (o:Order) REQUIRE o.order_id IS UNIQUE;
CREATE CONSTRAINT unique_reservation IF NOT EXISTS FOR (r:Reservation) REQUIRE r.reservation_id IS UNIQUE;

// Indices adicionales para busqueda
CREATE INDEX idx_user_email IF NOT EXISTS FOR (u:User) ON (u.email);
CREATE INDEX idx_product_categoria IF NOT EXISTS FOR (p:Product) ON (p.categoria);
CREATE INDEX idx_restaurant_nombre IF NOT EXISTS FOR (r:Restaurant) ON (r.nombre);

// ---------- CARGA DE NODOS ----------

// Users
LOAD CSV WITH HEADERS FROM 'file:///users.csv' AS row
MERGE (u:User {user_id: row.`user_id:ID(User)`})
SET u.nombre = row.nombre,
    u.email = row.email;

// Products
LOAD CSV WITH HEADERS FROM 'file:///products.csv' AS row
MERGE (p:Product {product_id: row.`product_id:ID(Product)`})
SET p.nombre = row.nombre,
    p.categoria = row.categoria,
    p.precio = toFloat(row.`precio:float`);

// Restaurants
LOAD CSV WITH HEADERS FROM 'file:///restaurants.csv' AS row
MERGE (r:Restaurant {restaurant_id: row.`restaurant_id:ID(Restaurant)`})
SET r.nombre = row.nombre,
    r.direccion = row.direccion;

// Orders
LOAD CSV WITH HEADERS FROM 'file:///orders.csv' AS row
MERGE (o:Order {order_id: row.`order_id:ID(Order)`})
SET o.total_amount = toFloat(row.`total_amount:float`),
    o.fecha = row.fecha;

// Reservations
LOAD CSV WITH HEADERS FROM 'file:///reservations.csv' AS row
MERGE (r:Reservation {reservation_id: row.`reservation_id:ID(Reservation)`})
SET r.fecha = row.fecha,
    r.party_size = toInteger(row.`party_size:int`);

// ---------- CARGA DE RELACIONES ----------

// placed (:User)-[:PLACED]->(:Order)
LOAD CSV WITH HEADERS FROM 'file:///placed.csv' AS row
MATCH (u:User {user_id: row.`:START_ID(User)`})
MATCH (o:Order {order_id: row.`:END_ID(Order)`})
MERGE (u)-[:PLACED {fecha: row.fecha}]->(o);

// includes (:Order)-[:INCLUDES]->(:Product)
LOAD CSV WITH HEADERS FROM 'file:///includes.csv' AS row
MATCH (o:Order {order_id: row.`:START_ID(Order)`})
MATCH (p:Product {product_id: row.`:END_ID(Product)`})
MERGE (o)-[:INCLUDES {cantidad: toInteger(row.`cantidad:int`), subtotal: toFloat(row.`subtotal:float`)}]->(p);

// at_order (:Order)-[:AT]->(:Restaurant)
LOAD CSV WITH HEADERS FROM 'file:///at_order.csv' AS row
MATCH (o:Order {order_id: row.`:START_ID(Order)`})
MATCH (r:Restaurant {restaurant_id: row.`:END_ID(Restaurant)`})
MERGE (o)-[:AT]->(r);

// made_reservation (:User)-[:MADE_RESERVATION]->(:Reservation)
LOAD CSV WITH HEADERS FROM 'file:///made_reservation.csv' AS row
MATCH (u:User {user_id: row.`:START_ID(User)`})
MATCH (r:Reservation {reservation_id: row.`:END_ID(Reservation)`})
MERGE (u)-[:MADE_RESERVATION]->(r);

// at_reservation (:Reservation)-[:AT]->(:Restaurant)
LOAD CSV WITH HEADERS FROM 'file:///at_reservation.csv' AS row
MATCH (r:Reservation {reservation_id: row.`:START_ID(Reservation)`})
MATCH (rest:Restaurant {restaurant_id: row.`:END_ID(Restaurant)`})
MERGE (r)-[:AT]->(rest);
