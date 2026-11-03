package media

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	"image/png"
	"os"
	"testing"
	"time"
)

func defaultImageFixture(t *testing.T, archived bool) (*store.Store, string, Media, fields.Definition) {
	t.Helper()
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	u, e := auth.New(s).CreateUser(ctx, "default-user", "Default", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	m, e := New(s).Upload(ctx, u.CollectionID, &b)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(m.ID)
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Default image", Type: "image", Archived: archived, Config: fields.Config{DefaultValue: raw}}, true)
	if e != nil {
		t.Fatal(e)
	}
	return s, u.CollectionID, m, d
}
func TestDeleteRetainsImageDefaultIncludingArchivedFields(t *testing.T) {
	for _, archived := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "archived"}[archived], func(t *testing.T) {
			s, cid, m, _ := defaultImageFixture(t, archived)
			e := New(s).Delete(context.Background(), cid, m.ID)
			if e == nil || fault.Wrap(e).Status != 409 {
				t.Fatalf("default-only media deletion must409, got %v", e)
			}
		})
	}
}
func TestCleanupRetainsDefaultVariantsUntilLastDefaultCleared(t *testing.T) {
	for _, archived := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "archived"}[archived], func(t *testing.T) {
			s, cid, m, d := defaultImageFixture(t, archived)
			ctx := context.Background()
			svc := New(s)
			now := time.Now().UTC()
			if _, e := s.DB.Exec("UPDATE media SET created_at=? WHERE id=?", now.Add(-48*time.Hour).Format(time.RFC3339Nano), m.ID); e != nil {
				t.Fatal(e)
			}
			if e := svc.Cleanup(ctx, now); e != nil {
				t.Fatal(e)
			}
			if _, e := svc.Info(ctx, cid, m.ID); e != nil {
				t.Fatal("cleanup collected persistent default", e)
			}
			for _, variant := range []string{"", "master", "thumbnail"} {
				if _, e := os.Stat(svc.Path(m.ID, variant)); e != nil {
					t.Fatal("default variant lost", variant, e)
				}
			}
			d.Config.DefaultValue = nil
			if _, e := fields.Save(ctx, s.DB, cid, d, false); e != nil {
				t.Fatal(e)
			}
			if e := svc.Cleanup(ctx, now); e != nil {
				t.Fatal(e)
			}
			if _, e := svc.Info(ctx, cid, m.ID); e == nil {
				t.Fatal("cleared default kept unreferenced media alive")
			}
			for _, variant := range []string{"", "master", "thumbnail"} {
				if _, e := os.Stat(svc.Path(m.ID, variant)); !os.IsNotExist(e) {
					t.Fatal("eligible media variant remains", variant, e)
				}
			}
		})
	}
}
