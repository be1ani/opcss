package api

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/be1ani/opcss/internal/db"
)

// fakeStore is an in-memory storer for tests.
type fakeStore struct {
	mu     sync.Mutex
	files  map[string]*db.File
	chunks []db.Chunk
}

func (f *fakeStore) GetFile(_ context.Context, id string) (*db.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return file, nil
}

func (f *fakeStore) InsertChunk(_ context.Context, p db.InsertChunkParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks = append(f.chunks, db.Chunk{
		FileID:      p.FileID,
		ChunkIndex:  p.ChunkIndex,
		TotalChunks: p.TotalChunks,
		Size:        p.Size,
		Checksum:    p.Checksum,
		StorageKey:  p.StorageKey,
	})
	return nil
}

func (f *fakeStore) CountChunksForFile(_ context.Context, fileID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.chunks {
		if c.FileID == fileID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) MarkFileComplete(_ context.Context, fileID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if file, ok := f.files[fileID]; ok {
		file.Status = db.FileStatusComplete
	}
	return nil
}

func (f *fakeStore) GetChunksByFileID(_ context.Context, fileID string) ([]db.Chunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []db.Chunk
	for _, c := range f.chunks {
		if c.FileID == fileID {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ChunkIndex < result[j].ChunkIndex
	})
	return result, nil
}

func (f *fakeStore) GetChunkByIndex(_ context.Context, fileID string, idx int) (*db.Chunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.chunks {
		if c.FileID == fileID && c.ChunkIndex == idx {
			cc := c
			return &cc, nil
		}
	}
	return nil, sql.ErrNoRows
}

// fakeStorage is an in-memory StorageBackend for tests.
type fakeStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (f *fakeStorage) UploadChunk(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = append([]byte(nil), data...)
	return nil
}

func (f *fakeStorage) DownloadChunk(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return append([]byte(nil), d...), nil
}

func (f *fakeStorage) Upload(_ context.Context, _ string, _ io.Reader) (int64, error) {
	return 0, nil
}

func (f *fakeStorage) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeStorage) Delete(_ context.Context, _ string) error { return nil }

func (f *fakeStorage) Exists(_ context.Context, _ string) (bool, error) { return false, nil }

// uploadChunk posts a single chunk to the test server and fails the test on any error.
func uploadChunk(t *testing.T, srv *httptest.Server, fileID string, index, total int, data []byte) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("chunk_index", strconv.Itoa(index)); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("total_chunks", strconv.Itoa(total)); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("chunk_data", "chunk")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	mw.Close()

	resp, err := http.Post(
		srv.URL+"/api/v1/files/"+fileID+"/chunks",
		mw.FormDataContentType(),
		&body,
	)
	if err != nil {
		t.Fatalf("upload chunk %d: %v", index, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload chunk %d: got status %d, want %d", index, resp.StatusCode, http.StatusCreated)
	}
}

func TestDownloadFile_RoundTrip(t *testing.T) {
	const fileID = "test-file-001"

	chunk0 := []byte("hello from chunk zero — first segment")
	chunk1 := []byte("chunk one carries the middle payload data")
	chunk2 := []byte("final bytes live in chunk two, the last piece")
	original := append(append(append([]byte(nil), chunk0...), chunk1...), chunk2...)

	store := &fakeStore{files: map[string]*db.File{
		fileID: {ID: fileID, Status: db.FileStatusPending},
	}}
	backend := &fakeStorage{data: make(map[string][]byte)}

	srv := httptest.NewServer(NewRouter(store, backend))
	defer srv.Close()

	// Upload 3 chunks via the real handler so the fake store/storage are populated.
	for i, data := range [][]byte{chunk0, chunk1, chunk2} {
		uploadChunk(t, srv, fileID, i, 3, data)
	}

	// Download the full assembled file.
	resp, err := http.Get(srv.URL + "/api/v1/files/" + fileID)
	if err != nil {
		t.Fatalf("download GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/octet-stream")
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("reassembled content mismatch: got %d bytes, want %d bytes", len(got), len(original))
	}
}

func TestDownloadChunk_Single(t *testing.T) {
	const fileID = "test-file-002"

	chunkData := []byte("isolated chunk for single-download test")

	store := &fakeStore{files: map[string]*db.File{
		fileID: {ID: fileID, Status: db.FileStatusPending},
	}}
	backend := &fakeStorage{data: make(map[string][]byte)}

	srv := httptest.NewServer(NewRouter(store, backend))
	defer srv.Close()

	uploadChunk(t, srv, fileID, 0, 1, chunkData)

	resp, err := http.Get(srv.URL + "/api/v1/files/" + fileID + "/chunks/0")
	if err != nil {
		t.Fatalf("chunk GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chunk download: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	checksum := resp.Header.Get("X-Chunk-Checksum")
	if checksum == "" {
		t.Error("X-Chunk-Checksum header is missing")
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !bytes.Equal(got, chunkData) {
		t.Errorf("chunk content mismatch: got %d bytes, want %d bytes", len(got), len(chunkData))
	}
}

func TestDownloadFile_NotFound(t *testing.T) {
	store := &fakeStore{files: map[string]*db.File{}}
	backend := &fakeStorage{data: make(map[string][]byte)}

	srv := httptest.NewServer(NewRouter(store, backend))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/files/no-such-file")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestDownloadChunk_NotFound(t *testing.T) {
	const fileID = "test-file-003"
	store := &fakeStore{files: map[string]*db.File{
		fileID: {ID: fileID, Status: db.FileStatusPending},
	}}
	backend := &fakeStorage{data: make(map[string][]byte)}

	srv := httptest.NewServer(NewRouter(store, backend))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/files/" + fileID + "/chunks/99")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
