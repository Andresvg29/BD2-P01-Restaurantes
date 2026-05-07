package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	mongoRepo "restaurante/repository/mongodb"
	pgRepo "restaurante/repository/postgres"

	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// agrupa todos los repositorios listos para inyectar en los handlers
type Repos struct {
	User        UserRepository
	Restaurant  RestaurantRepository
	Menu        MenuRepository
	Reservation ReservationRepository
	Order       OrderRepository
}

// lee la variable de entorno DB_ENGINE ("postgres" o "mongodb")
func NewRepos() (*Repos, func(), error) {
	engine := os.Getenv("DB_ENGINE")
	if engine == "" {
		engine = "postgres" // postgres default sino se coloca nada en la env
	}

	var redisClient *RedisClient
	var redisErr error
	enableRedis := os.Getenv("ENABLE_REDIS")
	if enableRedis == "" {
		enableRedis = "true" // por defecto habilitado
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

	switch engine {
	case "postgres":
		return newPostgresRepos(redisClient)
	case "mongodb":
		return newMongoRepos(redisClient)
	default:
		if redisClient != nil {
			redisClient.Close()
		}
		return nil, nil, fmt.Errorf("DB_ENGINE inválido: %q (usa 'postgres' o 'mongodb')", engine)
	}
}

func newPostgresRepos(redisClient *RedisClient) (*Repos, func(), error) {
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
		return nil, nil, fmt.Errorf("abriendo postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	// Crear repositorios base
	var userRepo UserRepository = pgRepo.NewUserRepository(db)
	var restaurantRepo RestaurantRepository = pgRepo.NewRestaurantRepository(db)
	var menuRepo MenuRepository = pgRepo.NewMenuRepository(db)
	reservationRepo := pgRepo.NewReservationRepository(db)
	orderRepo := pgRepo.NewOrderRepository(db)

	// Aplicar decoradores Redis si está disponible
	if redisClient != nil {
		userRepo = NewUserRedisDecorator(userRepo, redisClient.Client)
		restaurantRepo = NewRestaurantRedisDecorator(restaurantRepo, redisClient.Client)
		menuRepo = NewMenuRedisDecorator(menuRepo, redisClient.Client)
	}

	repos := &Repos{
		User:        userRepo,
		Restaurant:  restaurantRepo,
		Menu:        menuRepo,
		Reservation: reservationRepo,
		Order:       orderRepo,
	}

	cleanup := func() {
		db.Close()
		if redisClient != nil {
			redisClient.Close()
		}
	}

	return repos, cleanup, nil
}

func newMongoRepos(redisClient *RedisClient) (*Repos, func(), error) {
	uri := getEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getEnv("MONGO_DB", "restaurante_db")

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		return nil, nil, fmt.Errorf("conectando mongodb: %w", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		return nil, nil, fmt.Errorf("ping mongodb: %w", err)
	}

	db := client.Database(dbName)

	// Crear repositorios base
	var userRepo UserRepository = mongoRepo.NewUserRepository(db)
	var restaurantRepo RestaurantRepository = mongoRepo.NewRestaurantRepository(db)
	var menuRepo MenuRepository = mongoRepo.NewMenuRepository(db)
	reservationRepo := mongoRepo.NewReservationRepository(db)
	orderRepo := mongoRepo.NewOrderRepository(db)

	// Aplicar decoradores Redis si está disponible
	if redisClient != nil {
		userRepo = NewUserRedisDecorator(userRepo, redisClient.Client)
		restaurantRepo = NewRestaurantRedisDecorator(restaurantRepo, redisClient.Client)
		menuRepo = NewMenuRedisDecorator(menuRepo, redisClient.Client)
	}

	repos := &Repos{
		User:        userRepo,
		Restaurant:  restaurantRepo,
		Menu:        menuRepo,
		Reservation: reservationRepo,
		Order:       orderRepo,
	}

	cleanup := func() {
		client.Disconnect(context.Background())
		if redisClient != nil {
			redisClient.Close()
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
