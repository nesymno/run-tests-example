package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nesymno/run-tests-example/types"
)

type PostgresConfig struct {
	Host string
	Port string
	User string
	Pass string
	DB   string
}

type RedisConfig struct {
	Host string
	Port string
	DB   int
}

func TestApp(t *testing.T) {
	ctx := context.Background()

	postgresConfig := PostgresConfig{
		Host: os.Getenv("POSTGRES_HOST"),
		Port: os.Getenv("POSTGRES_PORT"),
		User: os.Getenv("POSTGRES_USER"),
		Pass: os.Getenv("POSTGRES_PASSWORD"),
		DB:   os.Getenv("POSTGRES_DB"),
	}

	redisConfig := RedisConfig{
		Host: os.Getenv("REDIS_HOST"),
		Port: os.Getenv("REDIS_PORT"),
		DB:   0,
	}

	appHost := os.Getenv("APP_HOST")
	if appHost == "" {
		appHost = "localhost"
	}
	appPort := os.Getenv("PORT")
	if appPort == "" {
		appPort = "8080"
	}

	t.Run("PostgreSQL Tests", func(t *testing.T) {
		t.Log("=== STARTING POSTGRESQL TEST ===")
		t.Log("About to call cleanupTestData...")
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Cleanup function panicked: %v", r)
			}
		}()
		cleanupTestData(t, ctx, postgresConfig, redisConfig)
		t.Log("=== CLEANUP COMPLETED, STARTING TEST ===")
		testPGWithConfig(t, ctx, postgresConfig)
	})

	t.Run("Redis Tests", func(t *testing.T) {
		t.Log("=== STARTING REDIS TEST ===")
		cleanupTestData(t, ctx, postgresConfig, redisConfig)
		t.Log("=== CLEANUP COMPLETED, STARTING TEST ===")
		testRedisWithConfig(t, ctx, redisConfig)
	})

	t.Run("Application Integration Tests", func(t *testing.T) {
		testAppIntegration(t, ctx, fmt.Sprintf("http://%s:%s", appHost, appPort))
	})
}

// cleanupTestData cleans up any existing test data from previous runs
func cleanupTestData(t *testing.T, ctx context.Context, postgresConfig PostgresConfig, redisConfig RedisConfig) {
	t.Log("=== CLEANUP FUNCTION CALLED ===")
	t.Log("Starting test data cleanup...")

	// Simple test to see if we can log
	t.Log("Cleanup function is executing...")

	// Clean up PostgreSQL
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		postgresConfig.Host, postgresConfig.Port, postgresConfig.User, postgresConfig.Pass, postgresConfig.DB)

	t.Logf("Connecting to PostgreSQL at %s:%s", postgresConfig.Host, postgresConfig.Port)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Logf("Error: Could not open PostgreSQL connection: %v", err)
		return
	}
	defer db.Close()

	// Test connection with retry
	var pingErr error
	for i := 0; i < 5; i++ {
		pingErr = db.Ping()
		if pingErr == nil {
			break
		}
		t.Logf("PostgreSQL ping attempt %d failed: %v", i+1, pingErr)
		time.Sleep(time.Second)
	}

	if pingErr != nil {
		t.Logf("Error: Could not ping PostgreSQL after 5 attempts: %v", pingErr)
		return
	}

	t.Log("PostgreSQL connection successful")

	// Clear test data table
	result, err := db.ExecContext(ctx, "DELETE FROM test_data")
	if err != nil {
		t.Logf("Error: Could not clear PostgreSQL test data: %v", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		t.Logf("Cleared %d rows from PostgreSQL test_data table", rowsAffected)
	}

	// Clean up Redis
	t.Logf("Connecting to Redis at %s:%s", redisConfig.Host, redisConfig.Port)
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisConfig.Host, redisConfig.Port),
		Password: "",
		DB:       redisConfig.DB,
	})
	defer rdb.Close()

	// Test Redis connection with retry
	var redisPingErr error
	for i := 0; i < 5; i++ {
		redisPingErr = rdb.Ping(ctx).Err()
		if redisPingErr == nil {
			break
		}
		t.Logf("Redis ping attempt %d failed: %v", i+1, redisPingErr)
		time.Sleep(time.Second)
	}

	if redisPingErr != nil {
		t.Logf("Error: Could not ping Redis after 5 attempts: %v", redisPingErr)
		return
	}

	t.Log("Redis connection successful")

	// Clear all test keys
	keys := []string{"key1", "key2", "key3", "test_list", "test_hash", "test_data_cache", "test_key"}
	clearedCount := 0
	for _, key := range keys {
		if rdb.Del(ctx, key).Val() > 0 {
			clearedCount++
		}
	}
	t.Logf("Cleared %d keys from Redis", clearedCount)

	t.Log("Test data cleanup completed")
}

// testPGWithConfig tests PostgreSQL functionality using PostgresConfig
func testPGWithConfig(t *testing.T, ctx context.Context, config PostgresConfig) {
	require.NotEmpty(t, config.Host, "postgresql host should be set")
	require.NotEmpty(t, config.Port, "postgresql port should be set")
	require.NotEmpty(t, config.User, "postgresql user should be set")
	require.NotEmpty(t, config.Pass, "postgresql password should be set")
	require.NotEmpty(t, config.DB, "postgresql database should be set")

	t.Logf("postgresql connection: %s:%s", config.Host, config.Port)

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Pass, config.DB)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	err = db.Ping()
	require.NoError(t, err, "failed to ping postgresql")

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_data (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err, "failed to create test table")

	testData := []types.TestData{
		{Name: "test1", Data: "data1"},
		{Name: "test2", Data: "data2"},
		{Name: "test3", Data: "data3"},
	}

	for _, data := range testData {
		_, err = db.ExecContext(ctx,
			"INSERT INTO test_data (name, data) VALUES ($1, $2)",
			data.Name, data.Data)
		require.NoError(t, err, "failed to insert test data")
	}

	rows, err := db.QueryContext(ctx, "SELECT id, name, data FROM test_data ORDER BY id")
	require.NoError(t, err, "failed to query test data")
	defer rows.Close()

	var results []types.TestData
	for rows.Next() {
		var data types.TestData
		err := rows.Scan(&data.ID, &data.Name, &data.Data)
		require.NoError(t, err)
		results = append(results, data)
	}

	require.NoError(t, rows.Err())
	assert.Len(t, results, 3, "expected 3 test records")
	assert.Equal(t, "test1", results[0].Name)
	assert.Equal(t, "data1", results[0].Data)

	t.Logf("postgresql test completed successfully - found %d records", len(results))
}

// testRedisWithConfig tests Redis functionality using RedisConfig
func testRedisWithConfig(t *testing.T, ctx context.Context, config RedisConfig) {
	require.NotEmpty(t, config.Host, "redis host should be set")
	require.NotEmpty(t, config.Port, "redis port should be set")

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.Host, config.Port),
		Password: "",
		DB:       config.DB,
	})
	defer rdb.Close()

	_, err := rdb.Ping(ctx).Result()
	require.NoError(t, err, "failed to ping redis")

	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		err = rdb.Set(ctx, key, value, 0).Err()
		require.NoError(t, err, "failed to set redis key")
	}

	for key, expectedValue := range testData {
		value, err := rdb.Get(ctx, key).Result()
		require.NoError(t, err, "failed to get redis key")
		assert.Equal(t, expectedValue, value)
	}

	err = rdb.LPush(ctx, "test_list", "item1", "item2", "item3").Err()
	require.NoError(t, err, "failed to push to redis list")

	listLength, err := rdb.LLen(ctx, "test_list").Result()
	require.NoError(t, err, "failed to get list length")
	assert.Equal(t, int64(3), listLength)

	err = rdb.HSet(ctx, "test_hash", map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}).Err()
	require.NoError(t, err, "failed to set redis hash")

	hashValue, err := rdb.HGet(ctx, "test_hash", "field1").Result()
	require.NoError(t, err, "failed to get redis hash field")
	assert.Equal(t, "value1", hashValue)

	t.Logf("redis test completed successfully")
}

// testAppIntegration tests the application's HTTP endpoints and integration
func testAppIntegration(t *testing.T, ctx context.Context, baseURL string) {
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Health Check", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var health types.HealthResponse
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err)

		assert.Equal(t, "healthy", health.Status)
		assert.Equal(t, "healthy", health.Database)
		assert.Equal(t, "healthy", health.Cache)
		assert.Equal(t, "1.0.0", health.Version)

		t.Logf("health check passed - database: %s, cache: %s", health.Database, health.Cache)
	})

	t.Run("Root Endpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "KubeRLy Test App")
	})

	t.Run("Data CRUD Operations", func(t *testing.T) {
		// Test POST - Create new data
		newData := types.TestData{Name: "integration_test", Data: "test_data"}
		jsonData, err := json.Marshal(newData)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/api/data", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Test GET - Retrieve data (should show cache miss first time)
		resp, err = client.Get(baseURL + "/api/data")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "MISS", resp.Header.Get("X-Cache"))

		// Test GET again - should show cache hit
		resp, err = client.Get(baseURL + "/api/data")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "HIT", resp.Header.Get("X-Cache"))
	})

	t.Run("Cache Operations", func(t *testing.T) {
		// Test POST - Set cache value
		cacheData := map[string]interface{}{
			"key":   "test_key",
			"value": "test_value",
			"ttl":   60,
		}
		jsonData, err := json.Marshal(cacheData)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/api/cache", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Test GET - Retrieve cache value
		resp, err = client.Get(baseURL + "/api/cache?key=test_key")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "test_key", result["key"])
		assert.Equal(t, "test_value", result["value"])
	})

	t.Logf("application integration tests completed successfully")
}
