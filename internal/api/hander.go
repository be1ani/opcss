package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/internal/storage"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db      *db.Store
	storage storage.StorageBackend
}

// NewRouter builds the Chi mux and wires all routes.
func NewRouter(store *db.Store, backend storage.StorageBackend) http.Handler {
	h := &Handler{db: store, storage: backend}

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)

	mux.Get("/healthz", handleHealthz)

	mux.Route("/api/v1", func(r chi.Router) {
		r.Post("/files/{id}/chunks", h.handleUploadChunk)
	})

	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}
