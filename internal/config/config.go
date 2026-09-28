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

	// Параметры пула БД
	DBMaxConns        int32
	DBMinConns        int32
	DBConnectTimeout  time.Duration
	DBQueryTimeout    time.Duration
	DBMaxConnLifetime time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		LogLevel:          getEnv("LOG_LEVEL", "debug"),
		ShutdownTimeout:   getDurationEnv("SHUTDOWN_TIMEOUT", 5*time.Second),
		DBMaxConns:        getInt32Env("DATABASE_MAX_CONNS", 10),
		DBMinConns:        getInt32Env("DATABASE_MIN_CONNS", 2),
		DBConnectTimeout:  getDurationEnv("DATABASE_CONNECT_TIMEOUT", 3*time.Second),
		DBQueryTimeout:    getDurationEnv("DATABASE_QUERY_TIMEOUT", 5*time.Second),
		DBMaxConnLifetime: getDurationEnv("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute),
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