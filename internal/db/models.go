package db

import "time"

type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusComplete FileStatus = "complete"
)

type VerificationStatus string

const (
	VerificationStatusOK        VerificationStatus = "ok"
	VerificationStatusCorrupted VerificationStatus = "corrupted"
)

type File struct {
	ID                 string
	Status             FileStatus
	CreatedAt          time.Time
	VerifiedAt         *time.Time
	FileChecksum       *string
	VerificationStatus *VerificationStatus
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

type UpdateFileVerificationParams struct {
	ID                 string
	VerifiedAt         time.Time
	FileChecksum       string
	VerificationStatus VerificationStatus
}
