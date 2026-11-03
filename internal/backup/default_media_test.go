package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/media"
	"image"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestArchivedDefaultImageRoundtripUnarchiveAndCreate(t *testing.T) {
	s := reviewStore(t)
	ctx := context.Background()
	source := reviewUser(t, s, "source-user")
	target := reviewUser(t, s, "target-user")
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	svc := media.New(s)
	m, e := svc.Upload(ctx, source.CollectionID, &b)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(m.ID)
	d, e := fields.Save(ctx, s.DB, source.CollectionID, fields.Definition{Label: "Default image", Type: "image", Archived: true, Config: fields.Config{DefaultValue: raw}}, true)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	s.DB.Exec("UPDATE media SET created_at=? WHERE id=?", now.Add(-48*time.Hour).Format(time.RFC3339Nano), m.ID)
	if e = svc.Cleanup(ctx, now); e != nil {
		t.Fatal(e)
	}
	p := reviewExport(t, s, source.CollectionID)
	if _, e = PreviewForCollection(ctx, s, target.CollectionID, p, WebLimits()); e != nil {
		t.Fatal(e)
	}
	if _, e = Import(ctx, s, target.CollectionID, p, WebLimits()); e != nil {
		t.Fatal(e)
	}
	defs, e := fields.List(ctx, s.DB, target.CollectionID)
	if e != nil || len(defs) != 1 {
		t.Fatal(defs, e)
	}
	imported := defs[0]
	var remappedID string
	if e = json.Unmarshal(imported.Config.DefaultValue, &remappedID); e != nil || remappedID == m.ID || !imported.Archived {
		t.Fatal("default/archival not remapped", e)
	}
	for _, variant := range []string{"", "master", "thumbnail"} {
		original, e := os.ReadFile(svc.Path(m.ID, variant))
		if e != nil {
			t.Fatal(e)
		}
		copied, e := os.ReadFile(svc.Path(remappedID, variant))
		if e != nil || sha256.Sum256(original) != sha256.Sum256(copied) {
			t.Fatal("default image variant changed", variant, e)
		}
	}
	imported.Archived = false
	if _, e = fields.Save(ctx, s.DB, target.CollectionID, imported, false); e != nil {
		t.Fatal(e)
	}
	name := "Default applied"
	l, e := liquids.Save(ctx, s.DB, target.CollectionID, "", liquids.Input{Name: &name}, nil)
	if e != nil || string(l.Values[imported.ID].Value) != `"`+remappedID+`"` {
		t.Fatal("unarchived remapped default unusable", e)
	}
	d.Config.DefaultValue = nil
	if _, e = fields.Save(ctx, s.DB, source.CollectionID, d, false); e != nil {
		t.Fatal(e)
	}
	s.DB.Exec("UPDATE media SET created_at=?", now.Add(-48*time.Hour).Format(time.RFC3339Nano))
	if e = svc.Cleanup(ctx, now); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Info(ctx, source.CollectionID, m.ID); e == nil {
		t.Fatal("cleared last default prevented cleanup")
	}
	if _, e = svc.Info(ctx, target.CollectionID, remappedID); e != nil {
		t.Fatal("cleanup affected another owner's referenced default", e)
	}
	reviewExport(t, s, source.CollectionID)
	reviewExport(t, s, target.CollectionID)
}
