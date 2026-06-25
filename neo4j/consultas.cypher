// ============================================================
// consultas.cypher — Consultas de analisis sobre el grafo Neo4j
// ============================================================
// Requiere: datos cargados via load_schema.cypher
// Ejecutar: cypher-shell -u neo4j -p password -f consultas.cypher
// ============================================================


// -----------------------------------------------------------
// CONSULTA 1: Co-compra — Top 5 productos mas comprados juntos
// -----------------------------------------------------------
// Descripcion: Encuentra los pares de productos que aparecen juntos
// con mayor frecuencia en un mismo pedido. Util para recomendaciones
// cruzadas ("los clientes que compraron X tambien compraron Y").
// Resultado esperado: pares (producto A, producto B, frecuencia)
// con las combinaciones mas recurrentes.

MATCH (p1:Product)<-[:INCLUDES]-(o:Order)-[:INCLUDES]->(p2:Product)
WHERE id(p1) < id(p2)
RETURN p1.nombre AS producto_a,
       p2.nombre AS producto_b,
       count(DISTINCT o) AS veces_juntos
ORDER BY veces_juntos DESC
LIMIT 5;


// -----------------------------------------------------------
// CONSULTA 2: Recomendacion entre usuarios por afinidad
// -----------------------------------------------------------
// Descripcion: Usuarios que compran en el mismo restaurante y en la
// misma categoria de producto, indicando preferencias similares.
// Util para sistema de recomendacion social ("a usuarios como tu
// les gusta...").
// Resultado esperado: pares (usuario A, usuario B, afinidad)
// con mayor numero de coincidencias.

MATCH (u1:User)-[:PLACED]->(o1:Order)-[:AT]->(r:Restaurant),
      (u2:User)-[:PLACED]->(o2:Order)-[:AT]->(r)
WHERE u1 <> u2
WITH u1, u2, count(DISTINCT r) AS restaurantes_compartidos
OPTIONAL MATCH (u1)-[:PLACED]->(:Order)-[:INCLUDES]->(p:Product)<-[:INCLUDES]-(:Order)<-[:PLACED]-(u2)
WITH u1, u2, restaurantes_compartidos, count(DISTINCT p) AS productos_compartidos
WHERE restaurantes_compartidos > 0 OR productos_compartidos > 0
RETURN u1.nombre AS usuario_a,
       u2.nombre AS usuario_b,
       restaurantes_compartidos,
       productos_compartidos,
       3 * restaurantes_compartidos + 1 * productos_compartidos AS afinidad_score
ORDER BY afinidad_score DESC
LIMIT 10;


// -----------------------------------------------------------
// CONSULTA 3: Camino minimo entre ubicaciones (ShortestPath)
// -----------------------------------------------------------
// Descripcion: Calcula la ruta mas corta entre dos ubicaciones
// usando la relacion :ROAD con distancia en km. Util para planificar
// rutas de reparto eficientes.
// Nota: Requiere que existan nodos (:Location) y relaciones (:ROAD).
// Si aun no se han generado, ejecutar primero el script de datos
// geoespaciales (Seccion 6.1).
// Variante A usa parametros; Variante B usa nombre de restaurante.
// -----------------------------------------------------------
// Variante A — Camino minimo con parametros:
//   :param origen => "loc_7faf45d6-aab8-42f1-a731-5ac11ab991ab";
//   :param destino => "loc_13573677-b6fd-4710-b25a-7a647a29bc09";
//
// MATCH (a:Location {location_id: $origen}),
//       (b:Location {location_id: $destino}),
//       p = shortestPath((a)-[:ROAD*]-(b))
// RETURN [n IN nodes(p) | n.address] AS ruta,
//        reduce(acc = 0.0, r IN relationships(p) | acc + r.distance_km) AS distancia_total_km;

// Variante B — Camino minimo desde un restaurante hacia todos los demas
MATCH (r1:Restaurant {nombre: "La Fonda Criolla"})-[:LOCATED_AT]->(l1:Location),
      (r2:Restaurant)-[:LOCATED_AT]->(l2:Location)
WHERE r1 <> r2
MATCH p = shortestPath((l1)-[:ROAD*]-(l2))
RETURN r1.nombre AS origen,
       r2.nombre AS destino,
       [n IN nodes(p) | n.address] AS ruta,
       reduce(acc = 0.0, r IN relationships(p) | acc + r.distance_km) AS distancia_km
ORDER BY distancia_km ASC;


// -----------------------------------------------------------
// CONSULTA 4: Usuarios influyentes (PageRank con GDS)
// -----------------------------------------------------------
// Descripcion: Calcula PageRank sobre el grafo de usuarios, pedidos
// y productos para identificar los usuarios mas influyentes.
// Util para programas de fidelidad segmentados ("clientes VIP").
// Requiere: Neo4j Graph Data Science (GDS) plugin instalado
// (incluido via NEO4J_PLUGINS=["apoc"] en docker-compose).
// Nota: GDS requiere instalacion separada del plugin. Si no esta
// disponible, se provee alternativa con Centrality basica.
// Resultado esperado: top 5 usuarios con mayor score de influencia.

// Opcion A: PageRank con GDS (si el plugin esta disponible)
// CALL gds.graph.project('user-product-graph', ['User', 'Order', 'Product'], ['PLACED', 'INCLUDES'])
// YIELD graphName, nodeCount, relationshipCount;
//
// CALL gds.pageRank.stream('user-product-graph')
// YIELD nodeId, score
// RETURN gds.util.asNode(nodeId).nombre AS usuario, score
// ORDER BY score DESC LIMIT 5;
//
// CALL gds.graph.drop('user-product-graph') YIELD graphName;

// Opcion B: Centralidad por grado (alternativa sin GDS)
// Usuarios con mas pedidos y conexiones en el grafo
MATCH (u:User)-[:PLACED]->(o:Order)-[:INCLUDES]->(p:Product)
RETURN u.nombre AS usuario,
       count(DISTINCT o) AS total_pedidos,
       count(DISTINCT p) AS productos_distintos,
       count(DISTINCT o) * 1.0 + count(DISTINCT p) * 0.5 AS influencia_score
ORDER BY influencia_score DESC
LIMIT 5;
