package api

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/pkg/checksum"
)

type chunkUploadResponse struct {
	ChunkIndex int    `json:"chunk_index"`
	StorageKey string `json:"storage_key"`
	Checksum   string `json:"checksum"`
}

func (h *Handler) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")

	// 32 MiB in-memory budget; larger chunks spill to disk automatically.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		slog.Error("chunk upload: parse multipart", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusBadRequest, errBody("invalid multipart form"))
		return
	}

	chunkIndex, err := strconv.Atoi(r.FormValue("chunk_index"))
	if err != nil {
		slog.Error("chunk upload: parse chunk_index", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusBadRequest, errBody("chunk_index must be an integer"))
		return
	}

	totalChunks, err := strconv.Atoi(r.FormValue("total_chunks"))
	if err != nil {
		slog.Error("chunk upload: parse total_chunks", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusBadRequest, errBody("total_chunks must be an integer"))
		return
	}

	if chunkIndex >= totalChunks {
		slog.Error("chunk upload: index out of range",
			"file_id", fileID, "chunk_index", chunkIndex, "total_chunks", totalChunks)
		writeJSON(w, http.StatusBadRequest, errBody("chunk_index must be less than total_chunks"))
		return
	}

	f, _, err := r.FormFile("chunk_data")
	if err != nil {
		slog.Error("chunk upload: read chunk_data field", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusBadRequest, errBody("chunk_data field required"))
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("chunk upload: close form file", "file_id", fileID, "err", err)
		}
	}()

	data, err := io.ReadAll(f)
	if err != nil {
		slog.Error("chunk upload: read chunk bytes", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("failed to read chunk data"))
		return
	}

	// Verify the file exists before storing anything.
	if _, err := h.db.GetFile(r.Context(), fileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Error("chunk upload: file not found", "file_id", fileID)
			writeJSON(w, http.StatusNotFound, errBody("file not found"))
			return
		}
		slog.Error("chunk upload: db get file", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}

	digest := checksum.SHA256Bytes(data)
	storageKey := fmt.Sprintf("%s/chunks/%d", fileID, chunkIndex)

	if err := h.storage.UploadChunk(r.Context(), storageKey, data); err != nil {
		slog.Error("chunk upload: upload to storage", "file_id", fileID, "key", storageKey, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("storage error"))
		return
	}

	if err := h.db.InsertChunk(r.Context(), db.InsertChunkParams{
		FileID:      fileID,
		ChunkIndex:  chunkIndex,
		TotalChunks: totalChunks,
		Size:        int64(len(data)),
		Checksum:    digest,
		StorageKey:  storageKey,
	}); err != nil {
		slog.Error("chunk upload: insert chunk", "file_id", fileID, "chunk_index", chunkIndex, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}

	count, err := h.db.CountChunksForFile(r.Context(), fileID)
	if err != nil {
		slog.Error("chunk upload: count chunks", "file_id", fileID, "err", err)
		// Non-fatal: chunk is persisted; completion check is best-effort.
	} else if count == totalChunks {
		if err := h.db.MarkFileComplete(r.Context(), fileID); err != nil {
			slog.Error("chunk upload: mark file complete", "file_id", fileID, "err", err)
		}
	}

	writeJSON(w, http.StatusCreated, chunkUploadResponse{
		ChunkIndex: chunkIndex,
		StorageKey: storageKey,
		Checksum:   digest,
	})
}
