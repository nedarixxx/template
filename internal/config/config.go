package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        string
	ShutdownTimeout time.Duration

	DBMaxConns        int32
	DBMinConns        int32
	DBConnectTimeout  time.Duration
	DBQueryTimeout    time.Duration
	DBMaxConnLifetime time.Duration

	PositionCheckInterval time.Duration
	PositionStaleAfter    time.Duration
	PositionWorkers       int
	PositionQueueSize     int
	PositionBatchSize     int
	PushHTTPURL           string
	PushRequestTimeout    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:              getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:       getDurationEnv("SHUTDOWN_TIMEOUT", 10*time.Second),
		DBMaxConns:            getInt32Env("DATABASE_MAX_CONNS", 10),
		DBMinConns:            getInt32Env("DATABASE_MIN_CONNS", 2),
		DBConnectTimeout:      getDurationEnv("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
		DBQueryTimeout:        getDurationEnv("DATABASE_QUERY_TIMEOUT", 3*time.Second),
		DBMaxConnLifetime:     getDurationEnv("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute),
		PositionCheckInterval: getDurationEnv("POSITION_CHECK_INTERVAL", 30*time.Second),
		PositionStaleAfter:    getDurationEnv("POSITION_STALE_AFTER", 60*time.Second),
		PositionWorkers:       getIntEnv("POSITION_WORKERS", 4),
		PositionQueueSize:     getIntEnv("POSITION_QUEUE_SIZE", 100),
		PositionBatchSize:     getIntEnv("POSITION_BATCH_SIZE", 100),
		PushHTTPURL:           getEnv("PUSH_HTTP_URL", "http://localhost:8090"),
		PushRequestTimeout:    getDurationEnv("PUSH_REQUEST_TIMEOUT", 1*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("environment variable DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getInt32Env(key string, defaultVal int32) int32 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseInt(valStr, 10, 32)
	if err != nil {
		return defaultVal
	}
	return int32(val)
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
