package checksum_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/be1ani/opcss/pkg/checksum"
)

// Verified with: echo -n "<input>" | sha256sum
var vectors = []struct {
	input  []byte
	digest string
}{
	{[]byte{}, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	{[]byte("hello world"), "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
	{[]byte("opc-chunk-data"), "2edb38518600cc742bfb76cedb8ad8ec60db406f6f0cb9bb292e6765f78897d6"},
}

func TestSHA256Bytes(t *testing.T) {
	t.Parallel()
	for _, v := range vectors {
		got := checksum.SHA256Bytes(v.input)
		if got != v.digest {
			t.Errorf("SHA256Bytes(%q) = %s, want %s", v.input, got, v.digest)
		}
	}
}

func TestSHA256Reader(t *testing.T) {
	t.Parallel()
	for _, v := range vectors {
		got, err := checksum.SHA256Reader(bytes.NewReader(v.input))
		if err != nil {
			t.Fatalf("SHA256Reader(%q) unexpected error: %v", v.input, err)
		}
		if got != v.digest {
			t.Errorf("SHA256Reader(%q) = %s, want %s", v.input, got, v.digest)
		}
	}
}

// fileDigest computes the expected FileChecksum directly so tests don't
// depend on the implementation they are testing.
func fileDigest(t *testing.T, digests []string) string {
	t.Helper()
	h := sha256.New()
	for _, d := range digests {
		if _, err := io.WriteString(h, d); err != nil {
			t.Fatal(err)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestFileChecksum(t *testing.T) {
	t.Parallel()

	d0 := checksum.SHA256Bytes([]byte("chunk-zero"))
	d1 := checksum.SHA256Bytes([]byte("chunk-one"))
	d2 := checksum.SHA256Bytes([]byte("chunk-two"))

	t.Run("empty input equals SHA256 of nothing", func(t *testing.T) {
		got := checksum.FileChecksum(nil)
		want := fileDigest(t, nil)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("single chunk equals SHA256 of that digest string", func(t *testing.T) {
		got := checksum.FileChecksum([]string{d0})
		want := fileDigest(t, []string{d0})
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("three chunks deterministic", func(t *testing.T) {
		in := []string{d0, d1, d2}
		c1 := checksum.FileChecksum(in)
		c2 := checksum.FileChecksum(in)
		if c1 != c2 {
			t.Errorf("not deterministic: %s != %s", c1, c2)
		}
		want := fileDigest(t, in)
		if c1 != want {
			t.Errorf("got %s, want %s", c1, want)
		}
	})

	t.Run("order sensitive", func(t *testing.T) {
		fwd := checksum.FileChecksum([]string{d0, d1, d2})
		rev := checksum.FileChecksum([]string{d2, d1, d0})
		if fwd == rev {
			t.Error("FileChecksum must be order-sensitive but forward==reverse")
		}
	})

	t.Run("different payloads differ", func(t *testing.T) {
		a := checksum.FileChecksum([]string{d0, d1})
		b := checksum.FileChecksum([]string{d0, d2})
		if a == b {
			t.Error("distinct chunk sets produced identical file checksum")
		}
	})
}

// SHA256Reader and SHA256Bytes must always agree.
func TestSHA256ReaderMatchesBytes(t *testing.T) {
	t.Parallel()
	data := []byte("deterministic-opc-payload")
	fromBytes := checksum.SHA256Bytes(data)
	fromReader, err := checksum.SHA256Reader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fromBytes != fromReader {
		t.Errorf("mismatch: SHA256Bytes=%s SHA256Reader=%s", fromBytes, fromReader)
	}
}
