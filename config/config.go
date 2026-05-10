package config

import (
	"fmt"
	"os"
)

// Config holds all environment-based configuration for the API server.
type Config struct {
	MongoURI string
	DBName   string
	Port     string
}

// Load reads configuration from environment variables and fails fast on missing required values.
func Load() (*Config, error) {
	mongoURI, err := requireEnv("MONGO_URI")
	if err != nil {
		return nil, err
	}
	dbName, err := requireEnv("DB_NAME")
	if err != nil {
		return nil, err
	}
	port, err := requireEnv("PORT")
	if err != nil {
		return nil, err
	}
	return &Config{
		MongoURI: mongoURI,
		DBName:   dbName,
		Port:     port,
	}, nil
}

func requireEnv(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return val, nil
}
