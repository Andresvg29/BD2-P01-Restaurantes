package repository

import (
	"context"
	"fmt"
	"time"

	"restaurante/models"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jClient encapsulates the connection to Neo4j (Bolt driver).
type Neo4jClient struct {
	Driver neo4j.DriverWithContext
}

// NewNeo4jClient creates a new connection to Neo4j.
func NewNeo4jClient() (*Neo4jClient, error) {
	uri := getEnv("NEO4J_URI", "bolt://neo4j:7687")
	user := getEnv("NEO4J_USER", "neo4j")
	password := getEnv("NEO4J_PASSWORD", "password")

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j driver: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutSec(5))
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("neo4j ping: %w", err)
	}

	return &Neo4jClient{Driver: driver}, nil
}

// Close closes the connection to Neo4j.
func (c *Neo4jClient) Close() {
	if c.Driver != nil {
		c.Driver.Close(context.Background())
	}
}

// GetAllLocations returns all locations from the graph.
func (c *Neo4jClient) GetAllLocations(ctx context.Context) ([]models.Location, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (l:Location) RETURN l.location_id AS id, l.lat AS lat, l.lon AS lon, l.address AS address ORDER BY l.address",
		nil)
	if err != nil {
		return nil, fmt.Errorf("neo4j query locations: %w", err)
	}

	var locations []models.Location
	for result.Next(ctx) {
		rec := result.Record()
		id, _ := rec.Get("id")
		lat, _ := rec.Get("lat")
		lon, _ := rec.Get("lon")
		addr, _ := rec.Get("address")
		locations = append(locations, models.Location{
			ID:      toString(id),
			Lat:     toFloat64(lat),
			Lon:     toFloat64(lon),
			Address: toString(addr),
		})
	}
	return locations, result.Err()
}

// GetLocationByRestaurant returns the location of a restaurant.
func (c *Neo4jClient) GetLocationByRestaurant(ctx context.Context, restaurantID string) (*models.Location, error) {
	locID := "loc_" + restaurantID
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		`MATCH (l:Location {location_id: $locID}) RETURN l.lat AS lat, l.lon AS lon, l.address AS address`,
		map[string]interface{}{"locID": locID})
	if err != nil {
		return nil, fmt.Errorf("neo4j location by restaurant: %w", err)
	}

	if result.Next(ctx) {
		rec := result.Record()
		lat, _ := rec.Get("lat")
		lon, _ := rec.Get("lon")
		addr, _ := rec.Get("address")
		return &models.Location{
			ID:      locID,
			Lat:     toFloat64(lat),
			Lon:     toFloat64(lon),
			Address: toString(addr),
		}, nil
	}
	return nil, fmt.Errorf("ubicacion no encontrada para restaurante %s", restaurantID)
}

// GetLocationByOrder returns the location of the restaurant associated with an order.
func (c *Neo4jClient) GetLocationByOrder(ctx context.Context, orderID string) (*models.Location, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		`MATCH (o:Order {order_id: $oid})-[:AT]->(r:Restaurant)-[:LOCATED_AT]->(l:Location)
		 RETURN l.location_id AS id, l.lat AS lat, l.lon AS lon, l.address AS address`,
		map[string]interface{}{"oid": orderID})
	if err != nil {
		return nil, fmt.Errorf("neo4j location by order: %w", err)
	}

	if result.Next(ctx) {
		rec := result.Record()
		id, _ := rec.Get("id")
		lat, _ := rec.Get("lat")
		lon, _ := rec.Get("lon")
		addr, _ := rec.Get("address")
		return &models.Location{
			ID:      toString(id),
			Lat:     toFloat64(lat),
			Lon:     toFloat64(lon),
			Address: toString(addr),
		}, nil
	}
	return nil, fmt.Errorf("ubicacion no encontrada para orden %s", orderID)
}

// GetShortestPath calculates the shortest path between two locations in the routes graph.
// Returns the sequence of locations and the total distance in km.
func (c *Neo4jClient) GetShortestPath(ctx context.Context, origenID, destinoID string) ([]models.Location, float64, error) {
	session := c.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		`MATCH (a:Location {location_id: $origen}),
			   (b:Location {location_id: $destino}),
			   p = shortestPath((a)-[:ROAD*]-(b))
		 RETURN [n IN nodes(p) | {id: n.location_id, lat: n.lat, lon: n.lon, address: n.address}] AS ruta,
				reduce(acc = 0.0, r IN relationships(p) | acc + r.distance_km) AS distancia_total`,
		map[string]interface{}{
			"origen":  origenID,
			"destino": destinoID,
		})
	if err != nil {
		return nil, 0, fmt.Errorf("neo4j shortestPath: %w", err)
	}

	if result.Next(ctx) {
		rec := result.Record()
		rutaRaw, _ := rec.Get("ruta")
		distRaw, _ := rec.Get("distancia_total")
		dist := toFloat64(distRaw)

		var ruta []models.Location
		if nodes, ok := rutaRaw.([]interface{}); ok {
			for _, n := range nodes {
				if nodeMap, ok := n.(map[string]interface{}); ok {
					ruta = append(ruta, models.Location{
						ID:      toString(nodeMap["id"]),
						Lat:     toFloat64(nodeMap["lat"]),
						Lon:     toFloat64(nodeMap["lon"]),
						Address: toString(nodeMap["address"]),
					})
				}
			}
		}
		return ruta, dist, nil
	}
	return nil, 0, fmt.Errorf("no hay ruta entre %s y %s", origenID, destinoID)
}

// --- helpers internos ---

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

func timeoutSec(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
