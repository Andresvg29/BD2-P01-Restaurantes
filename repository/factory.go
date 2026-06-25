package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	esConn "restaurante/elasticsearch"
	mongoRepo "restaurante/repository/mongo"
	pgRepo "restaurante/repository/postgres"

	"github.com/elastic/go-elasticsearch/v8"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repos groups all initialized repositories ready to inject into handlers.
type Repos struct {
	User        UserRepository
	Restaurant  RestaurantRepository
	Menu        MenuRepository
	Reservation ReservationRepository
	Order       OrderRepository
	RedisClient *RedisClient
	Neo4j       *Neo4jClient
}

// NewRepos creates repositories based on DB_ENGINE ("postgres" or "mongodb").
// Also initializes Redis (if ENABLE_REDIS=true) and Elasticsearch.
// Returns a cleanup closure to release resources on shutdown.
func NewRepos() (*Repos, func(), error) {
	engine := os.Getenv("DB_ENGINE")
	if engine == "" {
		engine = "postgres" // postgres default sino se coloca nada en la env
	}

	var redisClient *RedisClient
	var redisErr error
	enableRedis := os.Getenv("ENABLE_REDIS")
	if enableRedis == "" {
		enableRedis = "true" // enabled by default
	}

	if enableRedis == "true" {
		redisClient, redisErr = NewRedisClient()
		if redisErr != nil {
			log.Printf("Advertencia: No se pudo conectar a Redis: %v. Funcionando sin caché.", redisErr)
			redisClient = nil
		} else {
			log.Printf("Redis conectado exitosamente")
		}
	}
	// Neo4j (graceful degradation: if it does not respond, nil is used)
	var neo4jClient *Neo4jClient
	neo4jClient, neo4jErr := NewNeo4jClient()
	if neo4jErr != nil {
		log.Printf("Advertencia: No se pudo conectar a Neo4j: %v. Rutas de entrega no disponibles.", neo4jErr)
		neo4jClient = nil
	} else {
		log.Printf("Neo4j conectado exitosamente")
	}

	elasticClient := esConn.ESClient
	switch engine {
	case "postgres":
		return newPostgresRepos(redisClient, elasticClient, neo4jClient)
	case "mongodb":
		return newMongoRepos(redisClient, elasticClient, neo4jClient)
	default:
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
		return nil, nil, fmt.Errorf("DB_ENGINE inválido: %q (usa 'postgres' o 'mongodb')", engine)
	}
}

func newPostgresRepos(redisClient *RedisClient, elasticClient *elasticsearch.Client, neo4jClient *Neo4jClient) (*Repos, func(), error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("POSTGRES_USER", "admin"),
		getEnv("POSTGRES_PASSWORD", "password"),
		getEnv("POSTGRES_HOST", "db"),
		getEnv("POSTGRES_PORT", "5432"),
		getEnv("POSTGRES_DB", "restaurante_db"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
		return nil, nil, fmt.Errorf("abriendo postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	// Crear repositorios base
	userRepo := pgRepo.NewUserRepository(db)
	restaurantRepo := pgRepo.NewRestaurantRepository(db, elasticClient)
	menuRepo := pgRepo.NewMenuRepository(db)
	reservationRepo := pgRepo.NewReservationRepository(db)
	orderRepo := pgRepo.NewOrderRepository(db)

	repos := &Repos{
		User:        userRepo,
		Restaurant:  restaurantRepo,
		Menu:        menuRepo,
		Reservation: reservationRepo,
		Order:       orderRepo,
		RedisClient: redisClient,
		Neo4j:       neo4jClient,
	}

	cleanup := func() {
		db.Close()
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
	}

	return repos, cleanup, nil
}

func newMongoRepos(redisClient *RedisClient, elasticClient *elasticsearch.Client, neo4jClient *Neo4jClient) (*Repos, func(), error) {
	uri := getEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getEnv("MONGO_DB", "restaurante_db")

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
		return nil, nil, fmt.Errorf("conectando mongodb: %w", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
		return nil, nil, fmt.Errorf("ping mongodb: %w", err)
	}

	db := client.Database(dbName)

	// Crear repositorios base
	userRepo := mongoRepo.NewUserRepository(db)
	restaurantRepo := mongoRepo.NewRestaurantRepository(db, elasticClient)
	menuRepo := mongoRepo.NewMenuRepository(db)
	reservationRepo := mongoRepo.NewReservationRepository(db)
	orderRepo := mongoRepo.NewOrderRepository(db)

	repos := &Repos{
		User:        userRepo,
		Restaurant:  restaurantRepo,
		Menu:        menuRepo,
		Reservation: reservationRepo,
		Order:       orderRepo,
		RedisClient: redisClient,
		Neo4j:       neo4jClient,
	}

	cleanup := func() {
		client.Disconnect(context.Background())
		if redisClient != nil {
			redisClient.Close()
		}
		if neo4jClient != nil {
			neo4jClient.Close()
		}
	}

	return repos, cleanup, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
