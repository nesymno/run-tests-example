package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestData represents test data structure used in examples
type TestData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Data string `json:"data"`
}

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

	t.Run("PostgreSQL Tests", func(t *testing.T) {
		testPGWithConfig(t, ctx, postgresConfig)
	})

	t.Run("Redis Tests", func(t *testing.T) {
		testRedisWithConfig(t, ctx, redisConfig)
	})
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

	testData := []TestData{
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

	var results []TestData
	for rows.Next() {
		var data TestData
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
