package backup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func syntheticArchive(t *testing.T, kind string, schema int, payload []byte, symlink bool) string {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	h := &zip.FileHeader{Name: "collection.json", Method: zip.Deflate}
	h.SetMode(0600)
	if symlink {
		h.SetMode(os.ModeSymlink | 0600)
	}
	w, _ := z.CreateHeader(h)
	w.Write(payload)
	hash := sha256.Sum256(payload)
	m := Manifest{Kind: kind, SchemaVersion: schema, Files: map[string]FileDigest{"collection.json": {hex.EncodeToString(hash[:]), int64(len(payload))}}}
	raw, _ := json.Marshal(m)
	w, _ = z.Create("manifest.json")
	w.Write(raw)
	z.Close()
	p := filepath.Join(t.TempDir(), "synthetic.zip")
	os.WriteFile(p, b.Bytes(), 0600)
	return p
}
func TestArchiveRejectsBombSymlinkAndUnknownSchema(t *testing.T) {
	p := syntheticArchive(t, "collection", 1, bytes.Repeat([]byte("a"), 2<<20), false)
	if _, e := Preview(p, Limits{1024, 10, 10}); e == nil {
		t.Fatal("expanded limit ignored")
	}
	p = syntheticArchive(t, "collection", 1, []byte(`{}`), true)
	if _, e := Preview(p, WebLimits()); e == nil {
		t.Fatal("symlink accepted")
	}
	p = syntheticArchive(t, "collection", 999, []byte(`{}`), false)
	if _, e := Preview(p, WebLimits()); e == nil {
		t.Fatal("newer schema accepted")
	}
}
func TestArchiveRejectsChecksumMismatch(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("collection.json")
	w.Write([]byte(`{}`))
	m := Manifest{Kind: "collection", SchemaVersion: 1, Files: map[string]FileDigest{"collection.json": {SHA256: "0000000000000000000000000000000000000000000000000000000000000000", Size: 2}}}
	raw, _ := json.Marshal(m)
	w, _ = z.Create("manifest.json")
	w.Write(raw)
	z.Close()
	p := filepath.Join(t.TempDir(), "bad.zip")
	os.WriteFile(p, b.Bytes(), 0600)
	if _, e := Preview(p, WebLimits()); e == nil {
		t.Fatal("checksum mismatch accepted")
	}
}
