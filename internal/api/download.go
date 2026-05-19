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

	"github.com/be1ani/opcss/pkg/checksum"
)

func (h *Handler) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")

	if _, err := h.db.GetFile(r.Context(), fileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errBody("file not found"))
			return
		}
		slog.Error("download file: get file", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}

	chunks, err := h.db.GetChunksByFileID(r.Context(), fileID)
	if err != nil {
		slog.Error("download file: get chunks", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}
	if len(chunks) == 0 {
		writeJSON(w, http.StatusNotFound, errBody("no chunks found"))
		return
	}

	pr, pw := io.Pipe()
	ctx := r.Context()

	go func() {
		defer pw.Close()
		for _, c := range chunks {
			data, err := h.storage.DownloadChunk(ctx, c.StorageKey)
			if err != nil {
				slog.Error("download file: fetch chunk from storage",
					"file_id", fileID, "chunk_index", c.ChunkIndex, "err", err)
				_ = pw.CloseWithError(fmt.Errorf("storage error on chunk %d", c.ChunkIndex))
				return
			}
			if got := checksum.SHA256Bytes(data); got != c.Checksum {
				slog.Error("download file: checksum mismatch",
					"file_id", fileID, "chunk_index", c.ChunkIndex,
					"expected", c.Checksum, "got", got)
				_ = pw.CloseWithError(fmt.Errorf("checksum mismatch on chunk %d", c.ChunkIndex))
				return
			}
			if _, err := pw.Write(data); err != nil {
				return // reader closed (client disconnected)
			}
		}
	}()

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileID))
	w.Header().Set("Content-Type", "application/octet-stream")

	n, copyErr := io.Copy(w, pr)
	// Unblock the goroutine if it is still writing to pw after we stop reading.
	_ = pr.CloseWithError(io.ErrClosedPipe)

	if copyErr != nil {
		slog.Error("download file: stream to client", "file_id", fileID, "err", copyErr)
		if n == 0 {
			// No bytes were sent yet; we can still write a proper error response.
			writeJSON(w, http.StatusInternalServerError, errBody("download failed"))
		}
	}
}

func (h *Handler) handleDownloadChunk(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")
	indexStr := chi.URLParam(r, "index")

	chunkIndex, err := strconv.Atoi(indexStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("index must be an integer"))
		return
	}

	chunk, err := h.db.GetChunkByIndex(r.Context(), fileID, chunkIndex)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errBody("chunk not found"))
			return
		}
		slog.Error("download chunk: get chunk",
			"file_id", fileID, "chunk_index", chunkIndex, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}

	data, err := h.storage.DownloadChunk(r.Context(), chunk.StorageKey)
	if err != nil {
		slog.Error("download chunk: fetch from storage",
			"file_id", fileID, "chunk_index", chunkIndex, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("storage error"))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Chunk-Checksum", chunk.Checksum)
	_, _ = w.Write(data)
}
