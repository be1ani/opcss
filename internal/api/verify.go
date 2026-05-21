package api

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/pkg/checksum"
)

type verifyResponse struct {
	Status         string    `json:"status"`
	FileChecksum   string    `json:"file_checksum,omitempty"`
	ChunksVerified int       `json:"chunks_verified"`
	VerifiedAt     time.Time `json:"verified_at"`
	FailedChunks   []int     `json:"failed_chunks,omitempty"`
}

func (h *Handler) handleVerifyFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if _, err := h.db.GetFile(ctx, fileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errBody("file not found"))
			return
		}
		slog.Error("verify: get file", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}

	chunks, err := h.db.GetChunksByFileID(ctx, fileID)
	if err != nil {
		slog.Error("verify: get chunks", "file_id", fileID, "err", err)
		writeJSON(w, http.StatusInternalServerError, errBody("database error"))
		return
	}

	var (
		failedChunks []int
		// storedDigests are used for the merkle file checksum regardless of
		// whether individual chunks pass — the file checksum is stable and
		// depends only on what was recorded at upload time.
		storedDigests []string
	)

	for _, c := range chunks {
		// Download and verify one chunk at a time — never hold all chunks in memory.
		data, err := h.storage.DownloadChunk(ctx, c.StorageKey)
		if err != nil {
			slog.Error("verify: download chunk",
				"file_id", fileID, "chunk_index", c.ChunkIndex, "err", err)
			writeJSON(w, http.StatusInternalServerError, errBody("storage error"))
			return
		}

		got := checksum.SHA256Bytes(data)
		if got != c.Checksum {
			slog.Error("verify: checksum mismatch",
				"file_id", fileID,
				"chunk_index", c.ChunkIndex,
				"expected", c.Checksum,
				"got", got,
			)
			failedChunks = append(failedChunks, c.ChunkIndex)
		}

		storedDigests = append(storedDigests, c.Checksum)
	}

	now := time.Now().UTC()
	status := db.VerificationStatusOK
	if len(failedChunks) > 0 {
		status = db.VerificationStatusCorrupted
	}

	fileDigest := checksum.FileChecksum(storedDigests)

	if err := h.db.UpdateFileVerification(ctx, db.UpdateFileVerificationParams{
		ID:                 fileID,
		VerifiedAt:         now,
		FileChecksum:       fileDigest,
		VerificationStatus: status,
	}); err != nil {
		// Non-fatal: the caller still gets the verification result.
		slog.Error("verify: update verification record", "file_id", fileID, "err", err)
	}

	resp := verifyResponse{
		Status:         string(status),
		ChunksVerified: len(chunks),
		VerifiedAt:     now,
	}
	if len(failedChunks) > 0 {
		resp.FailedChunks = failedChunks
	} else {
		resp.FileChecksum = fileDigest
	}

	writeJSON(w, http.StatusOK, resp)
}
