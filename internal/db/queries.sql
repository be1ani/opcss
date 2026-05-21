-- name: GetFile :one
SELECT id, status, created_at, verified_at, file_checksum, verification_status
FROM files
WHERE id = $1;

-- name: InsertChunk :exec
INSERT INTO chunks (file_id, chunk_index, total_chunks, size, checksum, storage_key)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountChunksForFile :one
SELECT COUNT(*)
FROM chunks
WHERE file_id = $1;

-- name: MarkFileComplete :exec
UPDATE files
SET status = 'complete'
WHERE id = $1;

-- name: GetChunksByFileID :many
SELECT id, file_id, chunk_index, total_chunks, size, checksum, storage_key, created_at
FROM chunks
WHERE file_id = $1
ORDER BY chunk_index;

-- name: GetChunkByIndex :one
SELECT id, file_id, chunk_index, total_chunks, size, checksum, storage_key, created_at
FROM chunks
WHERE file_id = $1 AND chunk_index = $2;

-- name: UpdateFileVerification :exec
UPDATE files
SET verified_at = $2, file_checksum = $3, verification_status = $4
WHERE id = $1;

-- name: GetFilesNeedingVerification :many
SELECT id, status, created_at, verified_at, file_checksum, verification_status
FROM files
WHERE status = 'complete'
  AND (verified_at IS NULL OR verified_at < $1);
