package media

import (
	"bytes"
	"context"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestCropRevisionAndBounds(t *testing.T) {
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
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 160, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 160; x++ {
			im.Set(x, y, color.RGBA{uint8(x), uint8(y), 80, 255})
		}
	}
	png.Encode(&b, im)
	svc := New(s)
	m, e := svc.Upload(ctx, u.CollectionID, bytes.NewReader(b.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	next, e := svc.Crop(ctx, u.CollectionID, m.ID, Crop{X: 0, Y: 0, Width: 1, Height: 1})
	if e != nil {
		t.Fatal(e)
	}
	if next.ID == m.ID {
		t.Fatal("crop mutated old revision")
	}
	if _, e = svc.Crop(ctx, u.CollectionID, m.ID, Crop{X: 0.5, Y: 0, Width: 1, Height: 1}); e == nil {
		t.Fatal("out of bounds crop accepted")
	}
	if _, e = svc.Crop(ctx, u.CollectionID, m.ID, Crop{X: 0, Y: 0, Width: 0.5, Height: 1}); e == nil {
		t.Fatal("wrong pixel ratio accepted")
	}
	if _, e = svc.Info(ctx, "other", m.ID); e == nil {
		t.Fatal("foreign media exposed")
	}
	if _, e = svc.Info(ctx, u.CollectionID, m.ID); e != nil {
		t.Fatal("old revision lost", e)
	}
}
func TestUploadLimitsAndNoSVG(t *testing.T) {
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	svc := New(s)
	if _, e = svc.Upload(context.Background(), "none", bytes.NewReader([]byte(`<svg/>`))); e == nil {
		t.Fatal("SVG accepted")
	}
	if _, e = svc.Upload(context.Background(), "none", bytes.NewReader(make([]byte, (8<<20)+1))); e == nil {
		t.Fatal("oversized upload accepted")
	}
}
func TestTinyImageIsAccepted(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "tinyuser", "Tiny", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if _, e = New(s).Upload(ctx, u.CollectionID, &b); e != nil {
		t.Fatal(e)
	}
}
func TestThinImageResizesToNonzeroDimensions(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "thinuser", "Thin", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 5000, 1)))
	m, e := New(s).Upload(ctx, u.CollectionID, &b)
	if e != nil {
		t.Fatal(e)
	}
	if m.Width != 1600 || m.Height != 1 {
		t.Fatal(m.Width, m.Height)
	}
}
func TestCanceledUploadDoesNotPersistMedia(t *testing.T) {
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(context.Background(), "canceluser", "Cancel", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = New(s).Upload(ctx, u.CollectionID, &b); e == nil {
		t.Fatal("canceled upload succeeded")
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM media").Scan(&count)
	if count != 0 {
		t.Fatal("canceled upload persisted media")
	}
}
