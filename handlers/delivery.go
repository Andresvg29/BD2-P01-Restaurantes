package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

// GetDeliveryRoute implementa GET /api/delivery/routes
// Query params: desde=restaurant_id, hasta=order_id, algoritmo=nearest|cypher
func GetDeliveryRoute(neo4j *repository.Neo4jClient, restaurants repository.RestaurantRepository, orders repository.OrderRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if neo4j == nil {
			respondError(c, http.StatusServiceUnavailable, "Neo4j no disponible")
			return
		}

		desde := c.Query("desde")
		hasta := c.Query("hasta")
		algoritmo := c.DefaultQuery("algoritmo", "nearest")

		if desde == "" || hasta == "" {
			respondError(c, http.StatusBadRequest, "Parámetros 'desde' (restaurant_id) y 'hasta' (order_id) son requeridos")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get origin location (restaurant)
		origen, err := neo4j.GetLocationByRestaurant(ctx, desde)
		if err != nil {
			respondError(c, http.StatusNotFound, "Restaurante no encontrado o sin ubicación")
			return
		}

		// Get destination location (client via order_id)
		destino, err := neo4j.GetLocationByOrder(ctx, hasta)
		if err != nil {
			respondError(c, http.StatusNotFound, "Orden no encontrada o sin ubicación asociada")
			return
		}

		var ruta []models.RouteStep
		var distanciaTotal float64

		switch algoritmo {
		case "cypher":
			ruta, distanciaTotal, err = cypherRoute(ctx, neo4j, origen.ID, destino.ID)
		default:
			ruta, distanciaTotal = nearestNeighborRoute(ctx, neo4j, origen, destino)
		}

		if err != nil {
			respondError(c, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(c, http.StatusOK, models.RouteResponse{
			Ruta:           ruta,
			DistanciaTotal: math.Round(distanciaTotal*100) / 100,
			Algoritmo:      algoritmo,
		})
	}
}

// nearestNeighborRoute implements greedy TSP: from the origin, always visits
// the nearest unvisited location until reaching the destination.
func nearestNeighborRoute(ctx context.Context, neo4j *repository.Neo4jClient, origen, destino *models.Location) ([]models.RouteStep, float64) {
	allLocations, err := neo4j.GetAllLocations(ctx)
	if err != nil || len(allLocations) == 0 {
		// Fallback: ruta directa entre origen y destino
		dist := haversine(origen.Lat, origen.Lon, destino.Lat, destino.Lon)
		return []models.RouteStep{
			{Direccion: origen.Address, Lat: origen.Lat, Lon: origen.Lon, Orden: 0},
			{Direccion: destino.Address, Lat: destino.Lat, Lon: destino.Lon, Orden: 1},
		}, dist
	}

	// Construir mapa de ubicaciones por ID
	locMap := make(map[string]models.Location)
	for _, l := range allLocations {
		locMap[l.ID] = l
	}
	// Ensure origin and destination are in the map
	locMap[origen.ID] = *origen
	locMap[destino.ID] = *destino

	// TSP greedy: from origin, always go to the nearest unvisited location
	visited := make(map[string]bool)
	var ruta []models.RouteStep
	current := origen.ID
	var totalDist float64
	orden := 0

	ruta = append(ruta, models.RouteStep{
		Direccion: origen.Address,
		Lat:       origen.Lat,
		Lon:       origen.Lon,
		Orden:     orden,
	})
	visited[origen.ID] = true
	orden++

	for current != destino.ID {
		var bestID string
		var bestDist float64 = math.MaxFloat64
		currentLoc := locMap[current]

		for id, loc := range locMap {
			if visited[id] {
				continue
			}
			d := haversine(currentLoc.Lat, currentLoc.Lon, loc.Lat, loc.Lon)
			if d < bestDist {
				bestDist = d
				bestID = id
			}
		}

		if bestID == "" {
			break
		}

		totalDist += bestDist
		visited[bestID] = true
		nextLoc := locMap[bestID]
		ruta = append(ruta, models.RouteStep{
			Direccion: nextLoc.Address,
			Lat:       nextLoc.Lat,
			Lon:       nextLoc.Lon,
			Orden:     orden,
		})
		orden++
		current = bestID
	}

	return ruta, totalDist
}

// cypherRoute delegates route calculation to Neo4j ShortestPath.
func cypherRoute(ctx context.Context, neo4j *repository.Neo4jClient, origenID, destinoID string) ([]models.RouteStep, float64, error) {
	locations, dist, err := neo4j.GetShortestPath(ctx, origenID, destinoID)
	if err != nil {
		return nil, 0, err
	}

	var ruta []models.RouteStep
	for i, loc := range locations {
		ruta = append(ruta, models.RouteStep{
			Direccion: loc.Address,
			Lat:       loc.Lat,
			Lon:       loc.Lon,
			Orden:     i,
		})
	}
	return ruta, dist, nil
}

// assignZones groups locations into 2x2km zones (grid).
// Returns a map of zone_id -> []Location
func assignZones(locations []models.Location) map[string][]models.Location {
	zones := make(map[string][]models.Location)
	for _, loc := range locations {
		zoneX := int(math.Floor(loc.Lon * 50))   // ~2km celdas en longitud
		zoneY := int(math.Floor(loc.Lat * 50))   // ~2km celdas en latitud
		zoneID := fmt.Sprintf("Z%d_%d", zoneX, zoneY)
		zones[zoneID] = append(zones[zoneID], loc)
	}
	// Sort locations within each zone by address
	for zoneID := range zones {
		sort.Slice(zones[zoneID], func(i, j int) bool {
			return zones[zoneID][i].Address < zones[zoneID][j].Address
		})
	}
	return zones
}

// haversine calculates the distance in km between two geographic points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	R := 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
