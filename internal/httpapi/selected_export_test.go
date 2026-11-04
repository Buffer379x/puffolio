package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/media"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedExportIsPrivateBoundedAndImportable(t *testing.T) {
	c, s := setupClient(t)
	ctx := context.Background()
	me := c.do("GET", "/api/v1/me", nil, 200)
	cid := me["collectionId"].(string)
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	svc := media.New(s)
	images := []media.Media{}
	for i := 0; i < 3; i++ {
		m, e := svc.Upload(ctx, cid, bytes.NewReader(pngData.Bytes()))
		if e != nil {
			t.Fatal(e)
		}
		images = append(images, m)
	}
	defaultID, _ := json.Marshal(images[0].ID)
	field, e := fields.Save(ctx, s.DB, cid, fields.Definition{Label: "Subset image", Type: "image", Config: fields.Config{DefaultValue: defaultID}}, true)
	if e != nil {
		t.Fatal(e)
	}
	selected := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Selected", "values": map[string]any{field.ID: map[string]any{"type": "image", "value": images[1].ID}}}, 201)
	omitted := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Omitted", "values": map[string]any{field.ID: map[string]any{"type": "image", "value": images[2].ID}}}, 201)
	export := func(body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "http://localhost/api/v1/exports", bytes.NewReader(raw))
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", c.csrf)
		r.AddCookie(c.cookie)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w
	}
	w := export(map[string]any{"ids": []string{selected["id"].(string)}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	z, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	var collection backup.Collection
	for _, entry := range z.File {
		if entry.Name == "collection.json" {
			r, e := entry.Open()
			if e != nil {
				t.Fatal(e)
			}
			e = json.NewDecoder(r).Decode(&collection)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	if len(collection.Liquids) != 1 || collection.Liquids[0].ID != selected["id"] {
		t.Fatalf("selection ignored: %+v", collection.Liquids)
	}
	if len(collection.Media) != 2 {
		t.Fatalf("want selected and default media only, got %+v", collection.Media)
	}
	for _, entry := range z.File {
		if bytes.Contains([]byte(entry.Name), []byte(images[2].ID)) {
			t.Fatal("unselected media included", entry.Name)
		}
	}
	path := filepath.Join(t.TempDir(), "subset.zip")
	if e = os.WriteFile(path, w.Body.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	target, e := auth.New(s).CreateUser(ctx, "target", "Target", "user", "Target-long-password")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = backup.PreviewForCollection(ctx, s, target.CollectionID, path, backup.WebLimits()); e != nil {
		t.Fatal(e)
	}
	if _, e = backup.Import(ctx, s, target.CollectionID, path, backup.WebLimits()); e != nil {
		t.Fatal(e)
	}
	imported, e := liquids.List(ctx, s.DB, target.CollectionID, liquids.Query{Page: 1, PageSize: 24})
	if e != nil || len(imported.Items) != 1 || imported.Items[0].Name != "Selected" {
		t.Fatal(imported, e)
	}
	other, _ := secondAccount(t, c, s, "other")
	foreignLiquid := other.do("POST", "/api/v1/liquids", map[string]any{"name": "Foreign"}, 201)
	for _, ids := range [][]string{{}, {selected["id"].(string), foreignLiquid["id"].(string)}, {selected["id"].(string), "missing"}, {selected["id"].(string), selected["id"].(string)}, make([]string, 501)} {
		bad := export(map[string]any{"ids": ids})
		if bad.Code < 400 || bad.Header().Get("Content-Type") == "application/zip" {
			t.Fatalf("invalid selection returned archive: %d", bad.Code)
		}
	}
	for _, badBody := range []any{map[string]any{"ids": nil}, map[string]any{"ids": "one"}, map[string]any{"unrecognized": true}} {
		if bad := export(badBody); bad.Code < 400 {
			t.Fatal("invalid export body accepted", badBody, bad.Code)
		}
	}
	full := export(map[string]any{})
	if full.Code != 200 {
		t.Fatal(full.Code)
	}
	fullZip, e := zip.NewReader(bytes.NewReader(full.Body.Bytes()), int64(full.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range fullZip.File {
		if entry.Name == "collection.json" {
			r, _ := entry.Open()
			raw, _ := io.ReadAll(r)
			r.Close()
			if !bytes.Contains(raw, []byte(omitted["id"].(string))) {
				t.Fatal("full collection export lost omitted liquid")
			}
		}
	}
}
