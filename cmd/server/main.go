// Package main is the entrypoint for opcss.
package main

import (
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	_ "github.com/lib/pq"

	"github.com/be1ani/opcss/internal/api"
	"github.com/be1ani/opcss/internal/config"
	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/internal/storage"
)

func main() {
	cfg := config.Load()

	sqlDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[!] open db: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("[!] ping db: %v", err)
	}
	store := db.New(sqlDB)

	useSSL, _ := strconv.ParseBool(os.Getenv("STORAGE_USE_SSL"))
	backend, err := storage.NewMinioBackend(
		cfg.StorageEndpoint,
		cfg.StorageAccessKey,
		cfg.StorageSecretKey,
		cfg.StorageBucket,
		useSSL,
	)
	if err != nil {
		log.Fatalf("[!] init storage: %v", err)
	}

	router := api.NewRouter(store, backend)
	slog.Info("opcss listening", "addr", cfg.Addr)

	if err := http.ListenAndServe(cfg.Addr, router); err != nil {
		log.Fatalf("[!] server exited: %v", err)
	}
}
