package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nesymno/run-tests-example/types"
)

type App struct {
	DB  *sql.DB
	Rds *redis.Client
}

func (app *App) HealthHandler(w http.ResponseWriter, r *http.Request) {
	// Check database health
	dbStatus := "healthy"
	if err := app.DB.Ping(); err != nil {
		dbStatus = "unhealthy"
	}

	// Check Redis health
	cacheStatus := "healthy"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Rds.Ping(ctx).Err(); err != nil {
		cacheStatus = "unhealthy"
	}

	response := types.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Database:  dbStatus,
		Cache:     cacheStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (app *App) DataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Insert new data
		var data types.TestData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		_, err := app.DB.ExecContext(ctx,
			"INSERT INTO test_data (name, data) VALUES ($1, $2)",
			data.Name, data.Data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Insert error: %v", err), http.StatusInternalServerError)
			return
		}

		// Invalidate cache
		app.Rds.Del(ctx, "test_data_cache")

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
		return
	}

	// GET request - return data with caching
	ctx := context.Background()

	// Try to get from cache first
	cached, err := app.Rds.Get(ctx, "test_data_cache").Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	// Cache miss, get from database
	rows, err := app.DB.QueryContext(ctx, "SELECT id, name, data FROM test_data ORDER BY id")
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []types.TestData
	for rows.Next() {
		var data types.TestData
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
		app.Rds.Set(ctx, "test_data_cache", jsonData, 5*time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(results)
}

func (app *App) CacheHandler(w http.ResponseWriter, r *http.Request) {
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

		err := app.Rds.Set(ctx, req.Key, req.Value, ttl).Err()
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

	value, err := app.Rds.Get(ctx, key).Result()
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

func (app *App) RootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello from KubeRLy Test App!\n")
	fmt.Fprintf(w, "Available endpoints:\n")
	fmt.Fprintf(w, "- /health - Health check with DB status\n")
	fmt.Fprintf(w, "- /api/test - Test data from database\n")
	fmt.Fprintf(w, "- /api/data - CRUD operations on test data\n")
	fmt.Fprintf(w, "- /api/cache - Redis cache operations\n")
}
