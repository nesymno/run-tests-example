package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/nesymno/run-tests-example/app"
)

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database connections
	app, err := initApp()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	defer app.DB.Close()
	defer app.Rds.Close()

	// Setup HTTP handlers
	http.HandleFunc("/health", app.HealthHandler)
	http.HandleFunc("/api/data", app.DataHandler)
	http.HandleFunc("/api/cache", app.CacheHandler)
	http.HandleFunc("/", app.RootHandler)

	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initApp() (*app.App, error) {
	// PostgreSQL connection
	postgresHost := os.Getenv("POSTGRES_HOST")
	if postgresHost == "" {
		postgresHost = "postgres"
	}
	postgresPort := os.Getenv("POSTGRES_PORT")
	if postgresPort == "" {
		postgresPort = "5432"
	}
	postgresUser := os.Getenv("POSTGRES_USER")
	if postgresUser == "" {
		postgresUser = "postgres"
	}
	postgresPass := os.Getenv("POSTGRES_PASSWORD")
	if postgresPass == "" {
		postgresPass = "postgres"
	}
	postgresDB := os.Getenv("POSTGRES_DB")
	if postgresDB == "" {
		postgresDB = "testdb"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		postgresHost, postgresPort, postgresUser, postgresPass, postgresDB)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %v", err)
	}

	// Initialize database schema
	if err := initDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to init database: %v", err)
	}

	// Redis connection
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "redis"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: "",
		DB:       0,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %v", err)
	}

	return &app.App{DB: db, Rds: rdb}, nil
}

func initDatabase(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_data (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}
