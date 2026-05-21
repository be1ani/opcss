// Package storage to handle operations on files.
package storage

import (
	"bytes"
	"context"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type StorageBackend interface {
	// Upload streams r to the backend under key k and returns the stored byte count or error.
	Upload(ctx context.Context, key string, r io.Reader) (int64, error)

	// Download returns a read-closer for the object identified by k.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object identified by k.
	Delete(ctx context.Context, key string) error

	// Exists checks the existence of object k.
	Exists(ctx context.Context, key string) (bool, error)

	// UploadChunk writes a pre-read byte slice to the backend under key.
	UploadChunk(ctx context.Context, key string, data []byte) error

	// DownloadChunk fetches the object identified by key and returns its bytes.
	DownloadChunk(ctx context.Context, key string) ([]byte, error)
}

// MinioBackend implements StorageBackend using the minio-go SDK, compatible
// with Azure Blob Storage via its S3-compatible endpoint.
type MinioBackend struct {
	client *minio.Client
	bucket string
}

func NewMinioBackend(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioBackend, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinioBackend{client: client, bucket: bucket}, nil
}

func (m *MinioBackend) UploadChunk(ctx context.Context, key string, data []byte) error {
	_, err := m.client.PutObject(
		ctx, m.bucket, key,
		bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	return err
}

func (m *MinioBackend) Upload(ctx context.Context, key string, r io.Reader) (int64, error) {
	info, err := m.client.PutObject(
		ctx, m.bucket, key, r, -1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (m *MinioBackend) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
}

func (m *MinioBackend) DownloadChunk(ctx context.Context, key string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := obj.Close(); err != nil {
			slog.Error("storage: close object", "err", err)
		}
	}()
	return io.ReadAll(obj)
}

func (m *MinioBackend) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

func (m *MinioBackend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := m.client.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
