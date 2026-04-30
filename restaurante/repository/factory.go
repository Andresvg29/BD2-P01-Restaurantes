package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	mongoRepo "github.com/andre/restaurante/repository/mongodb"
	pgRepo "github.com/andre/restaurante/repository/postgres"

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
	switch engine {
	case "postgres":
		return newPostgresRepos()
	case "mongodb":
		return newMongoRepos()
	default:
		return nil, nil, fmt.Errorf("DB_ENGINE inválido: %q (usa 'postgres' o 'mongodb')", engine)
	}
}

func newPostgresRepos() (*Repos, func(), error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("POSTGRES_USER", "admin"),
		getEnv("POSTGRES_PASSWORD", "password"),
		getEnv("POSTGRES_HOST", "db"),
		getEnv("POSTGRES_PORT", "5432"),
		getEnv("POSTGRES_DB", "restaurante_db"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("abriendo postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	repos := &Repos{
		User:        pgRepo.NewUserRepository(db),
		Restaurant:  pgRepo.NewRestaurantRepository(db),
		Menu:        pgRepo.NewMenuRepository(db),
		Reservation: pgRepo.NewReservationRepository(db),
		Order:       pgRepo.NewOrderRepository(db),
	}

	cleanup := func() { db.Close() }
	return repos, cleanup, nil
}


func newMongoRepos() (*Repos, func(), error) {
	uri := getEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getEnv("MONGO_DB", "restaurante_db")

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("conectando mongodb: %w", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, nil, fmt.Errorf("ping mongodb: %w", err)
	}

	db := client.Database(dbName)

	repos := &Repos{
		User:        mongoRepo.NewUserRepository(db),
		Restaurant:  mongoRepo.NewRestaurantRepository(db),
		Menu:        mongoRepo.NewMenuRepository(db),
		Reservation: mongoRepo.NewReservationRepository(db),
		Order:       mongoRepo.NewOrderRepository(db),
	}

	cleanup := func() { client.Disconnect(context.Background()) }
	return repos, cleanup, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}