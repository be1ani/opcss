// Package db provides database access for OPC file metadata.
package db

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetFile(ctx context.Context, id string) (*File, error) {
	const q = `SELECT id, status, created_at, verified_at, file_checksum, verification_status FROM files WHERE id = $1`
	var (
		f                  File
		verificationStatus *string
	)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&f.ID, &f.Status, &f.CreatedAt,
		&f.VerifiedAt, &f.FileChecksum, &verificationStatus,
	)
	if err != nil {
		return nil, err
	}
	if verificationStatus != nil {
		vs := VerificationStatus(*verificationStatus)
		f.VerificationStatus = &vs
	}
	return &f, nil
}

func (s *Store) InsertChunk(ctx context.Context, p InsertChunkParams) error {
	const q = `
		INSERT INTO chunks (file_id, chunk_index, total_chunks, size, checksum, storage_key)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.ExecContext(ctx, q, p.FileID, p.ChunkIndex, p.TotalChunks, p.Size, p.Checksum, p.StorageKey)
	return err
}

func (s *Store) CountChunksForFile(ctx context.Context, fileID string) (int, error) {
	const q = `SELECT COUNT(*) FROM chunks WHERE file_id = $1`
	var n int
	err := s.db.QueryRowContext(ctx, q, fileID).Scan(&n)
	return n, err
}

func (s *Store) MarkFileComplete(ctx context.Context, fileID string) error {
	const q = `UPDATE files SET status = 'complete' WHERE id = $1`
	_, err := s.db.ExecContext(ctx, q, fileID)
	return err
}

func (s *Store) GetChunksByFileID(ctx context.Context, fileID string) ([]Chunk, error) {
	const q = `
		SELECT id, file_id, chunk_index, total_chunks, size, checksum, storage_key, created_at
		FROM chunks WHERE file_id = $1
		ORDER BY chunk_index`
	rows, err := s.db.QueryContext(ctx, q, fileID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("db: close rows", "err", err)
		}
	}()
	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.FileID, &c.ChunkIndex, &c.TotalChunks, &c.Size, &c.Checksum, &c.StorageKey, &c.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (s *Store) UpdateFileVerification(ctx context.Context, p UpdateFileVerificationParams) error {
	const q = `UPDATE files SET verified_at = $2, file_checksum = $3, verification_status = $4 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, q, p.ID, p.VerifiedAt, p.FileChecksum, string(p.VerificationStatus))
	return err
}

func (s *Store) GetFilesNeedingVerification(ctx context.Context, olderThan time.Time) ([]File, error) {
	const q = `
		SELECT id, status, created_at, verified_at, file_checksum, verification_status
		FROM files
		WHERE status = 'complete'
		  AND (verified_at IS NULL OR verified_at < $1)`
	rows, err := s.db.QueryContext(ctx, q, olderThan)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("db: close rows", "err", err)
		}
	}()
	var files []File
	for rows.Next() {
		var (
			f                  File
			verificationStatus *string
		)
		if err := rows.Scan(&f.ID, &f.Status, &f.CreatedAt, &f.VerifiedAt, &f.FileChecksum, &verificationStatus); err != nil {
			return nil, err
		}
		if verificationStatus != nil {
			vs := VerificationStatus(*verificationStatus)
			f.VerificationStatus = &vs
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

func (s *Store) GetChunkByIndex(ctx context.Context, fileID string, chunkIndex int) (*Chunk, error) {
	const q = `
		SELECT id, file_id, chunk_index, total_chunks, size, checksum, storage_key, created_at
		FROM chunks WHERE file_id = $1 AND chunk_index = $2`
	var c Chunk
	err := s.db.QueryRowContext(ctx, q, fileID, chunkIndex).Scan(
		&c.ID, &c.FileID, &c.ChunkIndex, &c.TotalChunks, &c.Size, &c.Checksum, &c.StorageKey, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
