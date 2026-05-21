package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/be1ani/opcss/internal/db"
	"github.com/be1ani/opcss/pkg/checksum"
)

// newVerifyServer builds a test server pre-seeded with a single complete file.
func newVerifyServer(t *testing.T, fileID string) (*httptest.Server, *fakeStore, *fakeStorage) {
	t.Helper()
	store := &fakeStore{files: map[string]*db.File{
		fileID: {ID: fileID, Status: db.FileStatusComplete},
	}}
	backend := &fakeStorage{data: make(map[string][]byte)}
	srv := httptest.NewServer(NewRouter(store, backend))
	t.Cleanup(srv.Close)
	return srv, store, backend
}

func TestVerifyFile_AllChunksOK(t *testing.T) {
	const fileID = "verify-file-ok"
	srv, store, backend := newVerifyServer(t, fileID)

	chunks := [][]byte{
		[]byte("alpha payload — first chunk data"),
		[]byte("beta payload — second chunk data"),
		[]byte("gamma payload — third chunk data"),
	}
	for i, data := range chunks {
		uploadChunk(t, srv, fileID, i, len(chunks), data)
	}

	resp, err := http.Post(srv.URL+"/api/v1/files/"+fileID+"/verify", "", nil)
	if err != nil {
		t.Fatalf("POST verify: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("status: got %q, want %q", body.Status, "ok")
	}
	if body.ChunksVerified != len(chunks) {
		t.Errorf("chunks_verified: got %d, want %d", body.ChunksVerified, len(chunks))
	}
	if body.FileChecksum == "" {
		t.Error("file_checksum must be non-empty on ok result")
	}
	if len(body.FailedChunks) != 0 {
		t.Errorf("failed_chunks: got %v, want empty", body.FailedChunks)
	}
	if body.VerifiedAt.IsZero() {
		t.Error("verified_at must be set")
	}

	// Verify DB was updated — hold the lock only for the reads, then release
	// before making any further HTTP calls that would re-acquire it.
	store.mu.Lock()
	f := store.files[fileID]
	fVS := f.VerificationStatus
	fCS := f.FileChecksum
	fVA := f.VerifiedAt
	var storedDigests []string
	for _, c := range store.chunks {
		if c.FileID == fileID {
			storedDigests = append(storedDigests, c.Checksum)
		}
	}
	store.mu.Unlock()

	if fVS == nil || *fVS != db.VerificationStatusOK {
		t.Errorf("DB verification_status: got %v, want ok", fVS)
	}
	if fCS == nil || *fCS == "" {
		t.Error("DB file_checksum must be set after verification")
	}
	if fVA == nil {
		t.Error("DB verified_at must be set after verification")
	}

	// Verify the returned file_checksum matches independent computation.
	want := checksum.FileChecksum(storedDigests)
	if body.FileChecksum != want {
		t.Errorf("file_checksum: got %s, want %s", body.FileChecksum, want)
	}

	// After verification X-File-Verified must be true on the next download.
	dlResp, err := http.Get(srv.URL + "/api/v1/files/" + fileID)
	if err != nil {
		t.Fatalf("GET file: %v", err)
	}
	defer dlResp.Body.Close()
	if got := dlResp.Header.Get("X-File-Verified"); got != "true" {
		t.Errorf("X-File-Verified: got %q, want %q", got, "true")
	}

	_ = backend // suppress unused warning
}

func TestVerifyFile_CorruptedChunk(t *testing.T) {
	const fileID = "verify-file-corrupt"
	srv, _, backend := newVerifyServer(t, fileID)

	uploadChunk(t, srv, fileID, 0, 2, []byte("intact chunk zero"))
	uploadChunk(t, srv, fileID, 1, 2, []byte("intact chunk one"))

	// Corrupt chunk 1 in storage after it was uploaded.
	backend.mu.Lock()
	backend.data[fileID+"/chunks/1"] = []byte("CORRUPTED BYTES")
	backend.mu.Unlock()

	resp, err := http.Post(srv.URL+"/api/v1/files/"+fileID+"/verify", "", nil)
	if err != nil {
		t.Fatalf("POST verify: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "corrupted" {
		t.Errorf("status: got %q, want %q", body.Status, "corrupted")
	}
	if len(body.FailedChunks) != 1 || body.FailedChunks[0] != 1 {
		t.Errorf("failed_chunks: got %v, want [1]", body.FailedChunks)
	}
	if body.FileChecksum != "" {
		t.Error("file_checksum must be omitted when status is corrupted")
	}
	if body.ChunksVerified != 2 {
		t.Errorf("chunks_verified: got %d, want 2", body.ChunksVerified)
	}
}

func TestVerifyFile_NotFound(t *testing.T) {
	store := &fakeStore{files: map[string]*db.File{}}
	backend := &fakeStorage{data: make(map[string][]byte)}
	srv := httptest.NewServer(NewRouter(store, backend))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/v1/files/no-such-file/verify", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] == "" {
		t.Error("response must have an error field")
	}
}

func TestDownloadFile_XFileVerifiedFalseBeforeVerification(t *testing.T) {
	const fileID = "verify-header-pending"
	srv, _, _ := newVerifyServer(t, fileID)

	uploadChunk(t, srv, fileID, 0, 1, []byte("some data"))

	resp, err := http.Get(srv.URL + "/api/v1/files/" + fileID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-File-Verified"); got != "false" {
		t.Errorf("X-File-Verified before verification: got %q, want %q", got, "false")
	}
}

func TestVerifyFile_MultipleCorruptedChunks(t *testing.T) {
	const fileID = "verify-multi-corrupt"
	srv, _, backend := newVerifyServer(t, fileID)

	for i := range 4 {
		uploadChunk(t, srv, fileID, i, 4, []byte("original chunk data"))
	}

	// Corrupt chunks 0 and 3.
	backend.mu.Lock()
	backend.data[fileID+"/chunks/0"] = []byte("BAD")
	backend.data[fileID+"/chunks/3"] = []byte("BAD")
	backend.mu.Unlock()

	resp, err := http.Post(srv.URL+"/api/v1/files/"+fileID+"/verify", "", nil)
	if err != nil {
		t.Fatalf("POST verify: %v", err)
	}
	defer resp.Body.Close()

	var body verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status != "corrupted" {
		t.Errorf("status: got %q, want corrupted", body.Status)
	}
	if len(body.FailedChunks) != 2 {
		t.Errorf("failed_chunks len: got %d, want 2", len(body.FailedChunks))
	}

	wantFailed := map[int]bool{0: true, 3: true}
	for _, idx := range body.FailedChunks {
		if !wantFailed[idx] {
			t.Errorf("unexpected failed chunk: %d", idx)
		}
	}
}

// TestVerifyFile_IdempotentOK verifies that calling verify twice on the same
// file produces the same file_checksum both times.
func TestVerifyFile_IdempotentOK(t *testing.T) {
	const fileID = "verify-idempotent"
	srv, _, _ := newVerifyServer(t, fileID)

	uploadChunk(t, srv, fileID, 0, 1, []byte("stable payload"))

	var body1, body2 verifyResponse
	for i, dst := range []*verifyResponse{&body1, &body2} {
		resp, err := http.Post(srv.URL+"/api/v1/files/"+fileID+"/verify", "", nil)
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			t.Fatalf("decode call %d: %v", i+1, err)
		}
	}

	if body1.FileChecksum != body2.FileChecksum {
		t.Errorf("file_checksum not stable across calls: %s vs %s",
			body1.FileChecksum, body2.FileChecksum)
	}
	if body1.Status != "ok" || body2.Status != "ok" {
		t.Errorf("expected ok on both calls, got %q and %q", body1.Status, body2.Status)
	}
}

// Ensure fakeStorage compiles — it is used via backend in this file.
var _ = (*fakeStorage)(nil)

func init() {
	// Prevent unused import warnings for packages referenced only via helpers.
	_ = bytes.NewReader
}
