// Package db provides database access for OPC file metadata.
package db

import (
	"context"
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetFile(ctx context.Context, id string) (*File, error) {
	const q = `SELECT id, status, created_at FROM files WHERE id = $1`
	var f File
	err := s.db.QueryRowContext(ctx, q, id).Scan(&f.ID, &f.Status, &f.CreatedAt)
	if err != nil {
		return nil, err
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
	defer rows.Close()
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
