package media

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"runtime"
	"testing"

	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/store"
)

// Only small compressed rows are materialized, including for high-bit-depth PNGs.
func rgba16PNG(w, h int, adam7, headerOnly bool) []byte {
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		binary.Write(&out, binary.BigEndian, uint32(len(data)))
		out.WriteString(kind)
		out.Write(data)
		crc := crc32.NewIEEE()
		crc.Write([]byte(kind))
		crc.Write(data)
		binary.Write(&out, binary.BigEndian, crc.Sum32())
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr, uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 16, 6
	if adam7 {
		ihdr[12] = 1
	}
	chunk("IHDR", ihdr)
	if !headerOnly {
		var compressed bytes.Buffer
		zw := zlib.NewWriter(&compressed)
		passes := [][4]int{{0, 0, 1, 1}}
		if adam7 {
			passes = [][4]int{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}}
		}
		for _, p := range passes {
			pw, ph := max(0, (w-p[0]+p[2]-1)/p[2]), max(0, (h-p[1]+p[3]-1)/p[3])
			if pw == 0 || ph == 0 {
				continue
			}
			row := make([]byte, 1+8*pw)
			for x := 0; x < pw; x++ {
				copy(row[1+8*x:], []byte{0x80, 0, 0x20, 0, 0xc0, 0, 0x80, 0})
			}
			for y := 0; y < ph; y++ {
				zw.Write(row)
			}
		}
		zw.Close()
		chunk("IDAT", compressed.Bytes())
	}
	chunk("IEND", nil)
	return out.Bytes()
}

func TestImageAxisBudgetBeforeDecode(t *testing.T) {
	for _, size := range [][2]int{{65536, 1}, {1, 65536}} {
		data := rgba16PNG(size[0], size[1], false, true)
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		// Missing pixel data would produce 422 if full Decode were attempted.
		_, err := New(nil).Upload(context.Background(), "unused", bytes.NewReader(data))
		f := fault.Wrap(err)
		if f.Status != 413 || f.Code != "image_dimensions" {
			t.Fatalf("%v: got %v/%s, want dimension 413 before decode", size, f.Status, f.Code)
		}
	}
}

type uniformRGBA64 struct{ rect image.Rectangle }

func (s uniformRGBA64) Bounds() image.Rectangle { return s.rect }
func (s uniformRGBA64) ColorModel() color.Model { return color.RGBA64Model }
func (s uniformRGBA64) At(x, y int) color.Color { return s.RGBA64At(x, y) }
func (s uniformRGBA64) RGBA64At(x, y int) color.RGBA64 {
	return color.RGBA64{0x4000, 0x1000, 0x6000, 0x8000}
}

func TestLargeResizeScratchBudgetAndPixels(t *testing.T) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	im := resize(uniformRGBA64{image.Rect(0, 0, 6000, 4000)}, 1600)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("24MP resize scratch allocation: %d bytes", allocated)
	if allocated > 160<<20 {
		t.Errorf("resize scratch allocation %d exceeds 160MiB budget", allocated)
	}
	if im.Bounds().Dx() != 1600 || im.Bounds().Dy() != 1067 {
		t.Fatal(im.Bounds())
	}
	got := color.RGBAModel.Convert(im.At(800, 533)).(color.RGBA)
	for i, pair := range [][2]uint8{{got.R, 64}, {got.G, 16}, {got.B, 96}, {got.A, 128}} {
		if math.Abs(float64(pair[0])-float64(pair[1])) > 1 {
			t.Fatalf("channel %d = %d, want %d", i, pair[0], pair[1])
		}
	}
}

type bandsRGBA64 struct{ uniformRGBA64 }

func (s bandsRGBA64) At(x, y int) color.Color { return s.RGBA64At(x, y) }
func (s bandsRGBA64) RGBA64At(x, y int) color.RGBA64 {
	x -= s.rect.Min.X
	if x < s.rect.Dx()/3 {
		return color.RGBA64{0xffff, 0, 0, 0xffff}
	}
	if x < s.rect.Dx()*2/3 {
		return color.RGBA64{0, 0xffff, 0, 0xffff}
	}
	return color.RGBA64{0, 0, 0xffff, 0xffff}
}

func TestProgressiveResizeUsesOriginalAspectAndPreservesContent(t *testing.T) {
	for _, size := range [][4]int{{6000, 3999, 1600, 1066}, {3999, 6000, 1066, 1600}} {
		source := bandsRGBA64{uniformRGBA64{image.Rect(20, 30, 20+size[0], 30+size[1])}}
		im := resize(source, 1600)
		if im.Bounds().Dx() != size[2] || im.Bounds().Dy() != size[3] {
			t.Fatalf("original-aspect rounding: %v", im.Bounds())
		}
		for i, want := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
			got := color.RGBAModel.Convert(im.At(im.Bounds().Dx()*(2*i+1)/6, im.Bounds().Dy()/2)).(color.RGBA)
			if got != want {
				t.Fatalf("band %d pixel %v, want %v", i, got, want)
			}
		}
	}
}

func TestRGBA16AndAdam7UploadPreservesAlphaGeometryAndAxisBoundary(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, err := auth.New(s).CreateUser(ctx, "memoryuser", "Memory", "user", "A-long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		w, h, dw, dh int
		adam7        bool
	}{
		{"rgba16", 3600, 2400, 1600, 1067, false},
		{"adam7", 2400, 1600, 1600, 1067, true},
		{"portrait", 1600, 2400, 1067, 1600, false},
		{"wide_boundary", 65535, 1, 1600, 1, false},
		{"tall_boundary", 1, 65535, 1, 1600, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := New(s).Upload(ctx, u.CollectionID, bytes.NewReader(rgba16PNG(tc.w, tc.h, tc.adam7, false)))
			if err != nil {
				t.Fatal(err)
			}
			if m.Width != tc.dw || m.Height != tc.dh {
				t.Fatalf("dimensions %dx%d want %dx%d", m.Width, m.Height, tc.dw, tc.dh)
			}
			for _, variant := range []string{"master", "image", "thumbnail"} {
				f, err := os.Open(New(s).Path(m.ID, variant))
				if err != nil {
					t.Fatal(err)
				}
				im, err := jpeg.Decode(f)
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
				b := im.Bounds()
				if variant != "master" && b.Dx() != b.Dy() {
					t.Fatal("invalid physical crop", b)
				}
				if variant == "thumbnail" && max(b.Dx(), b.Dy()) > 480 {
					t.Fatal("thumbnail too large", b)
				}
				pixel := color.RGBAModel.Convert(im.At(b.Dx()/2, b.Dy()/2)).(color.RGBA)
				for _, pair := range [][2]uint8{{pixel.R, 191}, {pixel.G, 143}, {pixel.B, 223}} {
					if math.Abs(float64(pair[0])-float64(pair[1])) > 5 {
						t.Fatalf("alpha-to-white pixel %v", pixel)
					}
				}
			}
		})
	}
}

func TestSubpixelCropRetainsCenterPixel(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, err := auth.New(s).CreateUser(ctx, "centeruser", "Center", "user", "A-long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	im := image.NewRGBA(image.Rect(0, 0, 1, 1600))
	for y := 0; y < 1600; y++ {
		im.SetRGBA(0, y, color.RGBA{0, 0, 255, 255})
	}
	for y := 798; y < 803; y++ {
		im.SetRGBA(0, y, color.RGBA{0, 255, 0, 255})
	}
	var source bytes.Buffer
	if err = png.Encode(&source, im); err != nil {
		t.Fatal(err)
	}
	m, err := New(s).Upload(ctx, u.CollectionID, &source)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(New(s).Path(m.ID, "image"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	output, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	pixel := color.RGBAModel.Convert(output.At(2, 1)).(color.RGBA)
	if pixel.G < 240 || pixel.B > 10 {
		t.Fatalf("subpixel crop lost center, got %v", pixel)
	}
}
