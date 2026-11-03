package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"io"
	"time"
)

type countWriter struct {
	w io.Writer
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, e := w.w.Write(p)
	w.n += int64(n)
	return n, e
}
func addCollection(ctx context.Context, z *zip.Writer, m *Manifest, c Collection, s *store.Store, cid string, ids []string) error {
	header := &zip.FileHeader{Name: "collection.json", Method: zip.Deflate}
	header.SetMode(0600)
	w, e := z.CreateHeader(header)
	if e != nil {
		return e
	}
	h := sha256.New()
	counter := &countWriter{w: io.MultiWriter(w, h)}
	meta := map[string]any{"schemaVersion": c.SchemaVersion, "fields": c.Fields, "tags": c.Tags, "media": c.Media, "preferences": c.Preferences, "dashboard": c.Dashboard, "views": c.Views}
	b, e := json.Marshal(meta)
	if e != nil {
		return e
	}
	if _, e = counter.Write(b[:len(b)-1]); e != nil {
		return e
	}
	if _, e = io.WriteString(counter, `,"liquids":[`); e != nil {
		return e
	}
	enc := json.NewEncoder(counter)
	for i, id := range ids {
		if e = ctx.Err(); e != nil {
			return e
		}
		l, e := liquids.Get(ctx, s.DB, cid, id)
		if e != nil {
			return e
		}
		if i > 0 {
			if _, e = io.WriteString(counter, ","); e != nil {
				return e
			}
		}
		if e = enc.Encode(l); e != nil {
			return e
		}
	}
	if _, e = io.WriteString(counter, "]}"); e != nil {
		return e
	}
	m.Files["collection.json"] = FileDigest{SHA256: hex.EncodeToString(h.Sum(nil)), Size: counter.n}
	return nil
}
func (a *checkedArchive) readCollection(visit func(liquids.Liquid) error) (Collection, error) {
	var c Collection
	if a.manifest.Kind != "collection" {
		return c, fault.Validation("Sammlungsexport erforderlich")
	}
	f, ok := a.files["collection.json"]
	if !ok {
		return c, fault.Validation("Sammlungsdaten fehlen")
	}
	r, e := f.Open()
	if e != nil {
		return c, e
	}
	defer r.Close()
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return c, fault.Validation("Ungültige Sammlungsdaten")
	}
	seen := map[string]bool{}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return c, e
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return c, fault.Validation("Doppelte Sammlungs-Eigenschaft")
		}
		seen[name] = true
		var target any
		switch name {
		case "schemaVersion":
			target = &c.SchemaVersion
		case "fields":
			target = &c.Fields
		case "tags":
			target = &c.Tags
		case "media":
			target = &c.Media
		case "preferences":
			target = &c.Preferences
		case "dashboard":
			target = &c.Dashboard
		case "views":
			target = &c.Views
		case "liquids":
			tok, e = d.Token()
			if e != nil || tok != json.Delim('[') {
				return c, fault.Validation("Liquid-Liste erforderlich")
			}
			for d.More() {
				var l liquids.Liquid
				if e = d.Decode(&l); e != nil {
					return c, fault.Validation("Ungültiges Liquid im Archiv")
				}
				c.liquidCount++
				created, err := time.Parse(time.RFC3339Nano, l.CreatedAt)
				if err != nil {
					return c, fault.Validation("Ungültiger Erstellungszeitpunkt im Archiv")
				}
				if c.liquidCount == 1 || created.After(c.latestLiquidCreatedAt) {
					c.latestLiquidCreatedAt = created
				}
				if c.liquidCount > 1000000 {
					return c, fault.Validation("Zu viele Liquids")
				}
				if visit != nil {
					if e = visit(l); e != nil {
						return c, e
					}
				}
			}
			tok, e = d.Token()
			if e != nil || tok != json.Delim(']') {
				return c, fault.Validation("Liquid-Liste unvollständig")
			}
			continue
		default:
			return c, fault.Validation("Unbekannte Sammlungs-Eigenschaft")
		}
		if e = d.Decode(target); e != nil {
			return c, fault.Validation("Ungültige Sammlungsdaten")
		}
	}
	tok, e = d.Token()
	if e != nil || tok != json.Delim('}') {
		return c, fault.Validation("Unvollständige Sammlungsdaten")
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return c, fault.Validation("Zusätzliche Sammlungsdaten")
	}
	if c.SchemaVersion != store.SchemaVersion || len(c.Fields) > 100 || len(c.Media) > 500000 || !seen["liquids"] {
		return c, fault.Validation("Ungültige Sammlungsstruktur")
	}
	return c, nil
}
func (a *checkedArchive) eachLiquid(visit func(liquids.Liquid) error) error {
	_, e := a.readCollection(visit)
	return e
}
