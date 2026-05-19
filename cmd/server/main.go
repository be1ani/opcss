// Package main is the entrypoint for opcss.
package main

import (
	"log"
	"net/http"
	"github.com/be1ani/opcss/internal/api"
	"github.com/be1ani/opcss/internal/config"
)

func main() {
	cfg := config.Load()
	router := api.NewRouter()
	log.Printf("[~] opcss is listening on %s", cfg.Addr)

	if err := http.ListenAndServe(cfg.Addr, router); err != nil {
		log.Fatalf("[!] server exited: %v", err)
	}
}