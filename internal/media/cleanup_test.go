package media

import (
	"bytes"
	"context"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	"image/png"
	"testing"
	"time"
)

func TestCleanupRetainsTrashMediaUntilExpiry(t *testing.T) {
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
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	svc := New(s)
	m, e := svc.Upload(ctx, u.CollectionID, &b)
	if e != nil {
		t.Fatal(e)
	}
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Image", Type: "image"}, true)
	if e != nil {
		t.Fatal(e)
	}
	name := "Trash"
	l, e := liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "image", Value: []byte(`"` + m.ID + `"`)}}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	liquids.Trash(ctx, s.DB, u.CollectionID, l.ID, 1, false)
	now := time.Now().UTC()
	s.DB.Exec("UPDATE media SET created_at=?", now.Add(-48*time.Hour).Format(time.RFC3339Nano))
	if e = svc.Cleanup(ctx, now); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Info(ctx, u.CollectionID, m.ID); e != nil {
		t.Fatal("deleted referenced trash media", e)
	}
	s.DB.Exec("UPDATE liquids SET deleted_at=?", now.Add(-31*24*time.Hour).Format(time.RFC3339Nano))
	if e = svc.Cleanup(ctx, now); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Info(ctx, u.CollectionID, m.ID); e == nil {
		t.Fatal("expired media retained")
	}
}
func TestPermanentLiquidDeleteRetainsSharedMedia(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "shareduser", "Shared", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	svc := New(s)
	m, e := svc.Upload(ctx, u.CollectionID, &b)
	if e != nil {
		t.Fatal(e)
	}
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Image", Type: "image"}, true)
	if e != nil {
		t.Fatal(e)
	}
	name := "Shared"
	input := liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "image", Value: []byte(`"` + m.ID + `"`)}}}
	a, e := liquids.Save(ctx, s.DB, u.CollectionID, "", input, nil)
	if e != nil {
		t.Fatal(e)
	}
	bLiquid, e := liquids.Save(ctx, s.DB, u.CollectionID, "", input, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e = liquids.Trash(ctx, s.DB, u.CollectionID, a.ID, a.Version, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = liquids.Permanent(ctx, s.DB, u.CollectionID, a.ID, a.Version); e != nil {
		t.Fatal(e)
	}
	if e = svc.Delete(ctx, u.CollectionID, m.ID); e == nil {
		t.Fatal("shared image deleted")
	}
	other, e := liquids.Get(ctx, s.DB, u.CollectionID, bLiquid.ID)
	if e != nil || string(other.Values[d.ID].Value) != `"`+m.ID+`"` {
		t.Fatal("other reference changed", e)
	}
}
