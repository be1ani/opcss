// Package jobs contains long-running background workers.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/internal/storage"
	"github.com/be1ani/opcss/pkg/checksum"
)

// verifierStore is the minimal DB surface the Verifier needs.
type verifierStore interface {
	GetFilesNeedingVerification(ctx context.Context, olderThan time.Time) ([]db.File, error)
	GetChunksByFileID(ctx context.Context, fileID string) ([]db.Chunk, error)
	UpdateFileVerification(ctx context.Context, p db.UpdateFileVerificationParams) error
}

// Verifier periodically checks every complete file whose last verification is
// older than staleness, re-downloading each chunk and comparing checksums.
type Verifier struct {
	store     verifierStore
	backend   storage.StorageBackend
	staleness time.Duration // files not verified within this window are re-checked
}

func NewVerifier(store verifierStore, backend storage.StorageBackend, staleness time.Duration) *Verifier {
	return &Verifier{store: store, backend: backend, staleness: staleness}
}

// Run starts the verification loop, ticking every interval. It blocks until
// ctx is cancelled. Intended to be called in a goroutine:
//
//	go jobs.NewVerifier(store, backend, 7*24*time.Hour).Run(ctx, 24*time.Hour)
func (v *Verifier) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately on start so there is no blind spot between
	// process restart and the first tick.
	v.runOnce(ctx)

	for {
		select {
		case <-ticker.C:
			v.runOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (v *Verifier) runOnce(ctx context.Context) {
	threshold := time.Now().UTC().Add(-v.staleness)
	files, err := v.store.GetFilesNeedingVerification(ctx, threshold)
	if err != nil {
		slog.Error("bg verifier: list files", "err", err)
		return
	}
	if len(files) == 0 {
		return
	}
	slog.Info("bg verifier: starting pass", "files", len(files))
	for _, f := range files {
		v.verifyOne(ctx, f.ID)
	}
}

func (v *Verifier) verifyOne(ctx context.Context, fileID string) {
	chunks, err := v.store.GetChunksByFileID(ctx, fileID)
	if err != nil {
		slog.Error("bg verifier: get chunks", "file_id", fileID, "err", err)
		return
	}

	var (
		failedCount   int
		storedDigests []string
	)

	for _, c := range chunks {
		// Download one chunk at a time — never accumulate all chunks in memory.
		data, err := v.backend.DownloadChunk(ctx, c.StorageKey)
		if err != nil {
			slog.Error("bg verifier: download chunk",
				"file_id", fileID, "chunk_index", c.ChunkIndex, "err", err)
			return // abort this file; will retry on next pass
		}

		got := checksum.SHA256Bytes(data)
		if got != c.Checksum {
			slog.Error("bg verifier: checksum mismatch",
				"file_id", fileID,
				"chunk_index", c.ChunkIndex,
				"expected", c.Checksum,
				"got", got,
			)
			failedCount++
		}

		storedDigests = append(storedDigests, c.Checksum)
	}

	status := db.VerificationStatusOK
	if failedCount > 0 {
		status = db.VerificationStatusCorrupted
	}

	now := time.Now().UTC()
	if err := v.store.UpdateFileVerification(ctx, db.UpdateFileVerificationParams{
		ID:                 fileID,
		VerifiedAt:         now,
		FileChecksum:       checksum.FileChecksum(storedDigests),
		VerificationStatus: status,
	}); err != nil {
		slog.Error("bg verifier: update verification record", "file_id", fileID, "err", err)
		return
	}

	slog.Info("bg verifier: file verified",
		"file_id", fileID,
		"status", string(status),
		"chunks_checked", len(chunks),
		"failed_chunks", failedCount,
	)
}
