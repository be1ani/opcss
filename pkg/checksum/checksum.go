// Package checksum provides reusable SHA-256 helpers for verifying OPC file
// integrity during chunked upload reassembly and at-rest validation.
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// returns the hex-encoded SHA-256 digest of stream r.
// The reader is consumed but not closed.
func SHA256Reader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// returns the hex-encoded SHA-256 digest of bytes b.
func SHA256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// FileChecksum computes a whole-file digest by hashing the hex-encoded
// per-chunk SHA-256 digests in chunk_index order. The result is stable
// across verifications because it depends only on the stored per-chunk
// digests, not the raw bytes.
func FileChecksum(chunkDigests []string) string {
	h := sha256.New()
	for _, d := range chunkDigests {
		_, _ = io.WriteString(h, d)
	}
	return hex.EncodeToString(h.Sum(nil))
}
