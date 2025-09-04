package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Database  string    `json:"database"`
	Cache     string    `json:"cache"`
}

type TestData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Data string    `json:"data"`
}

type App struct {
	db  *sql.DB
	rdb *redis.Client
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database connections
	app, err := initApp()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	defer app.db.Close()
	defer app.rdb.Close()

	// Setup HTTP handlers
	http.HandleFunc("/health", app.healthHandler)
	http.HandleFunc("/api/test", app.testDataHandler)
	http.HandleFunc("/api/data", app.dataHandler)
	http.HandleFunc("/api/cache", app.cacheHandler)
	http.HandleFunc("/", app.rootHandler)

	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initApp() (*App, error) {
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

	return &App{db: db, rdb: rdb}, nil
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

func (app *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check database health
	dbStatus := "healthy"
	if err := app.db.Ping(); err != nil {
		dbStatus = "unhealthy"
	}

	// Check Redis health
	cacheStatus := "healthy"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.rdb.Ping(ctx).Err(); err != nil {
		cacheStatus = "unhealthy"
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Database:  dbStatus,
		Cache:     cacheStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (app *App) testDataHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get data from database
	rows, err := app.db.QueryContext(ctx, "SELECT id, name, data FROM test_data ORDER BY id")
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []TestData
	for rows.Next() {
		var data TestData
		if err := rows.Scan(&data.ID, &data.Name, &data.Data); err != nil {
			http.Error(w, fmt.Sprintf("Scan error: %v", err), http.StatusInternalServerError)
			return
		}
		results = append(results, data)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Rows error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (app *App) dataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Insert new data
		var data TestData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		_, err := app.db.ExecContext(ctx,
			"INSERT INTO test_data (name, data) VALUES ($1, $2)",
			data.Name, data.Data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Insert error: %v", err), http.StatusInternalServerError)
			return
		}

		// Invalidate cache
		app.rdb.Del(ctx, "test_data_cache")

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
		return
	}

	// GET request - return data with caching
	ctx := context.Background()
	
	// Try to get from cache first
	cached, err := app.rdb.Get(ctx, "test_data_cache").Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	// Cache miss, get from database
	rows, err := app.db.QueryContext(ctx, "SELECT id, name, data FROM test_data ORDER BY id")
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []TestData
	for rows.Next() {
		var data TestData
		if err := rows.Scan(&data.ID, &data.Name, &data.Data); err != nil {
			http.Error(w, fmt.Sprintf("Scan error: %v", err), http.StatusInternalServerError)
			return
		}
		results = append(results, data)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Rows error: %v", err), http.StatusInternalServerError)
		return
	}

	// Cache the result
	if jsonData, err := json.Marshal(results); err == nil {
		app.rdb.Set(ctx, "test_data_cache", jsonData, 5*time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(results)
}

func (app *App) cacheHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	if r.Method == "POST" {
		// Set cache value
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			TTL   int    `json:"ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		ttl := time.Duration(req.TTL) * time.Second
		if ttl == 0 {
			ttl = 5 * time.Minute
		}

		err := app.rdb.Set(ctx, req.Key, req.Value, ttl).Err()
		if err != nil {
			http.Error(w, fmt.Sprintf("Cache set error: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "cached"})
		return
	}

	// GET request - get cache value
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key parameter", http.StatusBadRequest)
		return
	}

	value, err := app.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Cache get error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"key": key, "value": value})
}

func (app *App) rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello from KubeRLy Test App!\n")
	fmt.Fprintf(w, "Available endpoints:\n")
	fmt.Fprintf(w, "- /health - Health check with DB status\n")
	fmt.Fprintf(w, "- /api/test - Test data from database\n")
	fmt.Fprintf(w, "- /api/data - CRUD operations on test data\n")
	fmt.Fprintf(w, "- /api/cache - Redis cache operations\n")
}
