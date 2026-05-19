// Package config handles loading and validating service configuration.
package config


import "os"

type Config struct{
	Addr string // TCP address the HTTP server listens on.
}

// Load configuration from env and yaml config.
func Load() *Config{
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return &Config{Addr: addr}
}