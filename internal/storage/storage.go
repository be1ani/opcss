// Package storage to handle operations on files.
package storage


import (
	"context"
	"io"
)


type StorageBackend interface {
	// Upload streams r to the backend under key k and returns the stored byte count or error. 
	Upload(ctx context.Context, key string, r io.Reader) (int64, error)

	// Download reuturns a read-close for the object identified by k.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object identified by k
	Delete(ctx context.Context, key string) error

	// Checks the existence of object k
	Exists(ctx context.Context, key string) (bool, error)
}