package checksum_test

import (
	"bytes"
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
