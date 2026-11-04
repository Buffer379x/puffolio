package media

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/store"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
)

const MaxBytes = 8 << 20
const MaxPixels = 24_000_000
const MaxDimension = 65_535

var imageSlots = make(chan struct{}, 2)

type Crop struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type Media struct {
	ID           string `json:"id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Crop         Crop   `json:"crop"`
	URL          string `json:"url"`
	MasterURL    string `json:"masterUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
}
type Service struct{ Store *store.Store }

func New(s *store.Store) *Service { return &Service{Store: s} }
func (s *Service) Upload(ctx context.Context, cid string, r io.Reader) (Media, error) {
	b, e := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if e != nil {
		return Media{}, e
	}
	if len(b) > MaxBytes {
		return Media{}, fault.New(413, "image_too_large", "Bild darf maximal 8 MiB groß sein")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return Media{}, fault.Validation("JPEG, PNG oder WebP erforderlich")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return Media{}, fault.New(413, "image_pixels", "Bild darf maximal 24 Megapixel haben")
	}
	// Bound the decoder's scanline allocations, even for very thin RGBA16 PNGs.
	if cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return Media{}, fault.New(413, "image_dimensions", "Bild darf pro Seite maximal 65.535 Pixel haben")
	}
	select {
	case imageSlots <- struct{}{}:
	case <-ctx.Done():
		return Media{}, ctx.Err()
	}
	defer func() { <-imageSlots }()
	im, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return Media{}, fault.Validation("Bild kann nicht gelesen werden")
	}
	orientation := 1
	if format == "jpeg" {
		orientation = exifOrientation(b)
	}
	im = orient(im, orientation)
	im = resize(im, 1600)
	bounds := im.Bounds()
	crop := centerCrop(bounds.Dx(), bounds.Dy())
	return s.save(ctx, cid, im, crop)
}
func (s *Service) Info(ctx context.Context, cid, id string) (Media, error) {
	var raw string
	e := s.Store.DB.QueryRowContext(ctx, "SELECT data FROM media WHERE collection_id=? AND id=?", cid, id).Scan(&raw)
	if e == sql.ErrNoRows {
		return Media{}, fault.NotFound()
	}
	if e != nil {
		return Media{}, e
	}
	var m Media
	e = json.Unmarshal([]byte(raw), &m)
	return m, e
}
func (s *Service) Crop(ctx context.Context, cid, id string, crop Crop) (Media, error) {
	m, e := s.Info(ctx, cid, id)
	if e != nil {
		return m, e
	}
	if e = ValidateCrop(crop, m.Width, m.Height); e != nil {
		return m, e
	}
	select {
	case imageSlots <- struct{}{}:
	case <-ctx.Done():
		return m, ctx.Err()
	}
	defer func() { <-imageSlots }()
	f, e := os.Open(s.Path(id, "master"))
	if e != nil {
		return m, e
	}
	im, _, e := image.Decode(f)
	f.Close()
	if e != nil {
		return m, e
	}
	return s.save(ctx, cid, im, crop)
}
func (s *Service) Path(id, variant string) string {
	if !validID(id) {
		return ""
	}
	name := "image.jpg"
	switch variant {
	case "master":
		name = "master.jpg"
	case "thumbnail":
		name = "thumbnail.jpg"
	}
	return filepath.Join(s.Store.Dir, "media", id, name)
}
func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for _, r := range id {
		if r != '-' && (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
func validCropBounds(c Crop) error {
	for _, v := range []float64{c.X, c.Y, c.Width, c.Height} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fault.Validation("Ungültiger Ausschnitt")
		}
	}
	if c.X < 0 || c.Y < 0 || c.Width <= 0 || c.Height <= 0 || c.X+c.Width > 1.00000001 || c.Y+c.Height > 1.00000001 {
		return fault.Validation("Ausschnitt außerhalb des Bildes")
	}
	return nil
}

// ValidateCrop checks a crop to be applied now: images are square.
func ValidateCrop(c Crop, w, h int) error {
	if e := validCropBounds(c); e != nil {
		return e
	}
	pw, ph := c.Width*float64(w), c.Height*float64(h)
	if pw <= 0 || ph <= 0 || math.Abs(pw-ph) > 1.5 {
		return fault.Validation("Ausschnitt muss quadratisch sein")
	}
	return nil
}

// ValidateStoredCrop also accepts the 4:3 crops of images created by 0.1.0, so
// their exports stay importable. The app shows them centred in the square slot
// and turns them into a square when the crop is next edited.
func ValidateStoredCrop(c Crop, w, h int) error {
	if e := validCropBounds(c); e != nil {
		return e
	}
	pw, ph := c.Width*float64(w), c.Height*float64(h)
	if pw <= 0 || ph <= 0 || math.Abs(pw-ph) > 1.5 && math.Abs(pw-ph*4/3) > 1.5 {
		return fault.Validation("Ungültiges Seitenverhältnis des Ausschnitts")
	}
	return nil
}
func centerCrop(w, h int) Crop {
	pw, ph := float64(w), float64(h)
	if pw > ph {
		return Crop{X: (pw - ph) / 2 / pw, Width: ph / pw, Height: 1}
	}
	return Crop{Y: (ph - pw) / 2 / ph, Width: 1, Height: pw / ph}
}
func (s *Service) save(ctx context.Context, cid string, im image.Image, c Crop) (m Media, err error) {
	if e := ctx.Err(); e != nil {
		return m, e
	}
	b := im.Bounds()
	if e := ValidateCrop(c, b.Dx(), b.Dy()); e != nil {
		return m, e
	}
	m = Media{ID: store.ID(), Width: b.Dx(), Height: b.Dy(), Crop: c}
	m.URL = "/api/v1/media/" + m.ID
	m.MasterURL = m.URL + "?variant=master"
	m.ThumbnailURL = m.URL + "?variant=thumbnail"
	root := filepath.Join(s.Store.Dir, "media")
	staging, e := os.MkdirTemp(root, ".staging-")
	if e != nil {
		return m, e
	}
	defer os.RemoveAll(staging)
	encode := func(name string, im image.Image) error {
		f, e := os.OpenFile(filepath.Join(staging, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		e = jpeg.Encode(f, flatten(im), &jpeg.Options{Quality: 88})
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e != nil {
			return e
		}
		return closeErr
	}
	if e = encode("master.jpg", im); e != nil {
		return m, e
	}
	r := image.Rect(int(math.Round(c.X*float64(b.Dx()))), int(math.Round(c.Y*float64(b.Dy()))), int(math.Round((c.X+c.Width)*float64(b.Dx()))), int(math.Round((c.Y+c.Height)*float64(b.Dy()))))
	// Valid subpixel crops on one-pixel-wide/tall masters can round both edges
	// to the same coordinate. Keep the nearest source pixel before resampling.
	if r.Dx() == 0 && r.Min.X < b.Max.X {
		r.Max.X = r.Min.X + 1
	}
	if r.Dy() == 0 && r.Min.Y < b.Max.Y {
		r.Max.Y = r.Min.Y + 1
	}
	r = r.Intersect(b)
	if r.Empty() {
		return m, fault.Validation("Ausschnitt leer")
	}
	cropped := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	stdDraw.Draw(cropped, cropped.Bounds(), im, r.Min, stdDraw.Src)
	side := min(r.Dx(), 1600)
	if side < 4 {
		side = 4
	}
	output := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.CatmullRom.Scale(output, output.Bounds(), cropped, cropped.Bounds(), stdDraw.Src, nil)
	if e = encode("image.jpg", output); e != nil {
		return m, e
	}
	if e = encode("thumbnail.jpg", resize(output, 480)); e != nil {
		return m, e
	}
	if e = ctx.Err(); e != nil {
		return m, e
	}
	// Rename and row insert form the publish step a snapshot must not observe halfway.
	if e = s.Store.Writer.LockContext(ctx); e != nil {
		return m, e
	}
	defer s.Store.Writer.Unlock()
	dest := filepath.Join(root, m.ID)
	if e = os.Rename(staging, dest); e != nil {
		return m, e
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dest)
		}
	}()
	raw, _ := json.Marshal(m)
	_, err = s.Store.DB.ExecContext(ctx, "INSERT INTO media(id,collection_id,data,created_at) VALUES(?,?,?,?)", m.ID, cid, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	return m, err
}
func resize(im image.Image, maxEdge int) image.Image {
	b := im.Bounds()
	if b.Dx() <= maxEdge && b.Dy() <= maxEdge {
		return im
	}
	ratio := math.Min(float64(maxEdge)/float64(b.Dx()), float64(maxEdge)/float64(b.Dy()))
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(math.Round(float64(b.Dx())*ratio))), max(1, int(math.Round(float64(b.Dy())*ratio)))))
	// CatmullRom.Scale allocates a float intermediate proportional to destination
	// width * source height. Reduce with allocation-light stages first, keeping
	// its source within twice the final dimensions. Final rounding always uses
	// the oriented original, rather than accumulating rounding across stages.
	for b.Dx() > 2*dst.Bounds().Dx() || b.Dy() > 2*dst.Bounds().Dy() {
		stage := image.NewRGBA(image.Rect(0, 0, max(dst.Bounds().Dx(), (b.Dx()+1)/2), max(dst.Bounds().Dy(), (b.Dy()+1)/2)))
		draw.ApproxBiLinear.Scale(stage, stage.Bounds(), im, b, stdDraw.Src, nil)
		im, b = stage, stage.Bounds()
	}
	draw.CatmullRom.Scale(dst, dst.Bounds(), im, b, stdDraw.Src, nil)
	return dst
}
func flatten(im image.Image) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, im.Bounds().Dx(), im.Bounds().Dy()))
	stdDraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stdDraw.Src)
	stdDraw.Draw(dst, dst.Bounds(), im, im.Bounds().Min, stdDraw.Over)
	return dst
}
func orient(src image.Image, o int) image.Image {
	if o < 2 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch o {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
func exifOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xff || b[1] != 0xd8 {
		return 1
	}
	for p := 2; p+4 <= len(b); {
		if b[p] != 0xff {
			return 1
		}
		marker := b[p+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		n := int(binary.BigEndian.Uint16(b[p+2 : p+4]))
		if n < 2 || p+2+n > len(b) {
			return 1
		}
		seg := b[p+4 : p+2+n]
		if marker == 0xe1 && len(seg) >= 14 && string(seg[:6]) == "Exif\x00\x00" {
			t := seg[6:]
			var order binary.ByteOrder
			if string(t[:2]) == "II" {
				order = binary.LittleEndian
			} else if string(t[:2]) == "MM" {
				order = binary.BigEndian
			} else {
				return 1
			}
			if order.Uint16(t[2:4]) != 42 {
				return 1
			}
			offset := int(order.Uint32(t[4:8]))
			if offset < 8 || offset+2 > len(t) {
				return 1
			}
			count := int(order.Uint16(t[offset : offset+2]))
			for i := 0; i < count; i++ {
				at := offset + 2 + i*12
				if at+12 > len(t) {
					return 1
				}
				if order.Uint16(t[at:at+2]) == 0x112 && order.Uint16(t[at+2:at+4]) == 3 && order.Uint32(t[at+4:at+8]) == 1 {
					v := int(order.Uint16(t[at+8 : at+10]))
					if v >= 1 && v <= 8 {
						return v
					}
				}
			}
			return 1
		}
		p += 2 + n
	}
	return 1
}

// Image defaults are persistent references even while their fields are archived.
// Queries using this predicate bind the authenticated collection through alias m.
const mediaReferencedSQL = `(EXISTS(SELECT 1 FROM field_values v WHERE v.collection_id=m.collection_id AND v.value_media=m.id) OR EXISTS(SELECT 1 FROM field_definitions f WHERE f.collection_id=m.collection_id AND f.type='image' AND json_extract(f.data,'$.config.defaultValue')=m.id))`

func (s *Service) Delete(ctx context.Context, cid, id string) error {
	if _, e := s.Info(ctx, cid, id); e != nil {
		return e
	}
	var count int
	if e := s.Store.DB.QueryRowContext(ctx, "SELECT count(*) FROM media m WHERE m.collection_id=? AND m.id=? AND "+mediaReferencedSQL, cid, id).Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return fault.New(409, "media_referenced", "Bild wird noch verwendet")
	}
	if _, e := s.Store.DB.ExecContext(ctx, "DELETE FROM media WHERE collection_id=? AND id=?", cid, id); e != nil {
		return e
	}
	if e := os.RemoveAll(filepath.Join(s.Store.Dir, "media", id)); e != nil {
		return fmt.Errorf("remove media: %w", e)
	}
	return nil
}
