package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestCropUsesOrientedMaster(t *testing.T) {
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
	im := image.NewRGBA(image.Rect(0, 0, 30, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 30; x++ {
			im.Set(x, y, color.RGBA{200, 80, 0, 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, im, nil)
	original := b.Bytes()
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	segment := []byte{255, 225, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(exif)+2))
	data := append(append(append([]byte{}, original[:2]...), segment...), exif...)
	data = append(data, original[2:]...)
	m, e := New(s).Upload(ctx, u.CollectionID, bytes.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	if m.Width != 40 || m.Height != 30 {
		t.Fatalf("not oriented: %dx%d", m.Width, m.Height)
	}
	if _, e = New(s).Crop(ctx, u.CollectionID, m.ID, Crop{Width: 0.75, Height: 1}); e != nil {
		t.Fatal(e)
	}
}
func TestRejectTwentyFourMegapixelsBeforeDecode(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close() // JPEG SOF advertises 6000x5000 without allocating pixels.
	data := []byte{255, 216, 255, 192, 0, 17, 8, 0x13, 0x88, 0x17, 0x70, 3, 1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0, 255, 217}
	if _, e = New(s).Upload(ctx, "none", bytes.NewReader(data)); e == nil {
		t.Fatal("oversized dimensions accepted")
	}
}
