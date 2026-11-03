package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectionRoundtripAndVerifiedRestore(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "alice", "Alice", "admin", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	raw := `{"theme":"system","timezone":"Europe/Berlin","view":"grid","images":true,"listFields":[],"gridFields":[],"roles":{}}`
	s.DB.Exec("INSERT INTO user_preferences(user_id,data) VALUES(?,?)", u.ID, raw)
	s.DB.Exec("INSERT INTO dashboards(user_id,data) VALUES(?,'[]')", u.ID)
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Date", Type: "date"}, true)
	if e != nil {
		t.Fatal(e)
	}
	name := "Roundtrip"
	liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "date", Value: json.RawMessage(`"2028-02-29"`)}}}, nil)
	var b bytes.Buffer
	if e = Export(ctx, s, u.CollectionID, &b, "test"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "export.zip")
	os.WriteFile(path, b.Bytes(), 0600)
	p, e := Preview(path, WebLimits())
	if e != nil || p.Counts.Liquids != 1 {
		t.Fatal(p, e)
	}
	if _, e = Import(ctx, s, u.CollectionID, path, WebLimits()); e != nil {
		t.Fatal(e)
	}
	page, e := liquids.List(ctx, s.DB, u.CollectionID, liquids.Query{Page: 1, PageSize: 24})
	if e != nil || page.Total != 2 || page.Items[0].ID == page.Items[1].ID {
		t.Fatal(page, e)
	}
	rawtoken, _ := auth.New(s).StartSession(ctx, u.ID)
	b.Reset()
	if e = Create(ctx, s, &b, "test"); e != nil {
		t.Fatal(e)
	}
	path = filepath.Join(t.TempDir(), "backup.zip")
	os.WriteFile(path, b.Bytes(), 0600)
	dest := filepath.Join(t.TempDir(), "restored")
	if e = Restore(ctx, path, dest); e != nil {
		t.Fatal(e)
	}
	r, e := store.Open(ctx, config.Config{DataDir: dest})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if _, e = auth.New(r).Session(ctx, rawtoken); e == nil {
		t.Fatal("restore preserved session")
	}
	var count int
	r.DB.QueryRow("SELECT count(*) FROM liquids").Scan(&count)
	if count != 2 {
		t.Fatal(count)
	}
}
func TestArchiveRejectsTraversalAndSymlink(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "media/../escape"} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		w, _ := z.Create(name)
		w.Write([]byte("bad"))
		z.Close()
		p := filepath.Join(t.TempDir(), "bad.zip")
		os.WriteFile(p, b.Bytes(), 0600)
		if _, e := Preview(p, WebLimits()); e == nil {
			t.Fatal("accepted", name)
		}
	}
}
