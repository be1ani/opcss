package api

import (
	"encoding/json"
	"net/http"
	"github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

// Main router - Chi Multiplexer
func NewRouter() http.Handler {
	mux := chi.NewRouter()
	
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)

	mux.Get("/healthz", handleHealthz)

	return mux
}

// Endpoint for health checks
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status":"ok"})
}