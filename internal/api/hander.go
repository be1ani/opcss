package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/internal/storage"
)

// storer abstracts the database operations required by the handlers.
type storer interface {
	GetFile(ctx context.Context, id string) (*db.File, error)
	InsertChunk(ctx context.Context, p db.InsertChunkParams) error
	CountChunksForFile(ctx context.Context, fileID string) (int, error)
	MarkFileComplete(ctx context.Context, fileID string) error
	GetChunksByFileID(ctx context.Context, fileID string) ([]db.Chunk, error)
	GetChunkByIndex(ctx context.Context, fileID string, chunkIndex int) (*db.Chunk, error)
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db      storer
	storage storage.StorageBackend
}

// NewRouter builds the Chi mux and wires all routes.
func NewRouter(store storer, backend storage.StorageBackend) http.Handler {
	h := &Handler{db: store, storage: backend}

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)

	mux.Get("/healthz", handleHealthz)

	mux.Route("/api/v1", func(r chi.Router) {
		r.Post("/files/{id}/chunks", h.handleUploadChunk)
		r.Get("/files/{id}", h.handleDownloadFile)
		r.Get("/files/{id}/chunks/{index}", h.handleDownloadChunk)
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
