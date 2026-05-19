// Package config handles loading and validating service configuration.
package config

import "os"

type Config struct {
	Addr             string // TCP address the HTTP server listens on.
	DatabaseURL      string // Postgres DSN.
	StorageEndpoint  string // Minio / Azure Blob S3-compat endpoint.
	StorageAccessKey string
	StorageSecretKey string
	StorageBucket    string
}

func Load() *Config {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return &Config{
		Addr:             addr,
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		StorageEndpoint:  os.Getenv("STORAGE_ENDPOINT"),
		StorageAccessKey: os.Getenv("STORAGE_ACCESS_KEY"),
		StorageSecretKey: os.Getenv("STORAGE_SECRET_KEY"),
		StorageBucket:    os.Getenv("STORAGE_BUCKET"),
	}
}
