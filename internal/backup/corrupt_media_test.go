package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/media"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestImportRejectsTruncatedImageWithValidDimensionsAndChecksum(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "alice", "Alice", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	s.DB.Exec(`INSERT INTO user_preferences(user_id,data) VALUES(?,?)`, u.ID, `{"theme":"system","timezone":"UTC","view":"grid","images":true,"listFields":[],"gridFields":[],"roles":{}}`)
	s.DB.Exec(`INSERT INTO dashboards(user_id,data) VALUES(?,'[]')`, u.ID)
	var im bytes.Buffer
	png.Encode(&im, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	m, e := media.New(s).Upload(ctx, u.CollectionID, &im)
	if e != nil {
		t.Fatal(e)
	}
	var good bytes.Buffer
	if e = Export(ctx, s, u.CollectionID, &good, "test"); e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(good.Bytes()), int64(good.Len()))
	if e != nil {
		t.Fatal(e)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		r, _ := f.Open()
		files[f.Name], _ = io.ReadAll(r)
		r.Close()
	}
	key := "media/" + m.ID + "/master.jpg"
	original := files[key]
	cut := len(original) - 20
	truncated := original[:cut]
	if _, _, e = image.DecodeConfig(bytes.NewReader(truncated)); e != nil {
		t.Fatal("fixture must retain valid dimensions", e)
	}
	if _, _, e = image.Decode(bytes.NewReader(truncated)); e == nil {
		t.Fatal("fixture must be an undecodable image")
	}
	files[key] = truncated
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	hash := sha256.Sum256(truncated)
	manifest.Files[key] = FileDigest{SHA256: hex.EncodeToString(hash[:]), Size: int64(len(truncated))}
	files["manifest.json"], _ = json.Marshal(manifest)
	var bad bytes.Buffer
	writer := zip.NewWriter(&bad)
	for name, data := range files {
		w, _ := writer.Create(name)
		w.Write(data)
	}
	writer.Close()
	p := filepath.Join(t.TempDir(), "corrupt.zip")
	os.WriteFile(p, bad.Bytes(), 0600)
	if _, e = Preview(p, WebLimits()); e == nil {
		t.Fatal("accepted undecodable image with valid config and checksum")
	}
}
