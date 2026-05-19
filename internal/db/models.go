package db

import "time"

type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusComplete FileStatus = "complete"
)

type File struct {
	ID        string
	Status    FileStatus
	CreatedAt time.Time
}

type Chunk struct {
	ID          int64
	FileID      string
	ChunkIndex  int
	TotalChunks int
	Size        int64
	Checksum    string
	StorageKey  string
	CreatedAt   time.Time
}

type InsertChunkParams struct {
	FileID      string
	ChunkIndex  int
	TotalChunks int
	Size        int64
	Checksum    string
	StorageKey  string
}
