package backup

import (
	"context"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLargeStreamingRoundtrip(t *testing.T) {
	if os.Getenv("PUFFOLIO_LARGE_IO") != "1" {
		t.Skip("set PUFFOLIO_LARGE_IO=1; exercises >100MiB collection under 512MiB runtime")
	}
	ctx := context.Background()
	dataDir := t.TempDir()
	artifactDir := os.Getenv("PUFFOLIO_LARGE_ARTIFACTS_DIR")
	if artifactDir != "" {
		dataDir = filepath.Join(artifactDir, "data")
		if e := os.MkdirAll(artifactDir, 0700); e != nil {
			t.Fatal(e)
		}
	}
	s, e := store.Open(ctx, config.Config{DataDir: dataDir})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "alice", "Alice", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	defs := []fields.Definition{}
	for i := 0; i < 3; i++ {
		d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Type: "longtext", Label: "Note"}, true)
		if e != nil {
			t.Fatal(e)
		}
		defs = append(defs, d)
	}
	s.DB.Exec(`INSERT INTO user_preferences(user_id,data) VALUES(?,?)`, u.ID, `{"theme":"system","timezone":"UTC","view":"grid","images":true,"listFields":[],"gridFields":[],"roles":{}}`)
	s.DB.Exec(`INSERT INTO dashboards(user_id,data) VALUES(?,'[]')`, u.ID)
	tx, e := s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	note := strings.Repeat("x", 10000)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < 4000; i++ {
		id := store.ID()
		if _, e = tx.Exec(`INSERT INTO liquids(id,collection_id,name,normalized_name,created_at,updated_at) VALUES(?,?,?,?,?,?)`, id, u.CollectionID, "Large", "large", now, now); e != nil {
			t.Fatal(e)
		}
		for _, d := range defs {
			if _, e = tx.Exec(`INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_text,value_search) VALUES(?,?,?,'longtext',?,?)`, u.CollectionID, id, d.ID, note, note); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "large.zip")
	if artifactDir != "" {
		p = filepath.Join(artifactDir, "collection.zip")
		if e := os.WriteFile(filepath.Join(artifactDir, "collection-id"), []byte(u.CollectionID), 0600); e != nil {
			t.Fatal(e)
		}
	}
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = Export(ctx, s, u.CollectionID, f, "test"); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if _, e = Preview(p, WebLimits()); e == nil {
		t.Fatal("web limit accepted >100MiB")
	}
	counts, e := Import(ctx, s, u.CollectionID, p, Limits{MaxExpandedBytes: 256 << 20, MaxEntries: 20000, MaxImages: 10000})
	if e != nil || counts.Liquids != 4000 {
		t.Fatal(counts, e)
	}
	var n, total int
	if e = s.DB.QueryRow(`SELECT count(*),sum(length(value_text)) FROM field_values`).Scan(&n, &total); e != nil || n != 24000 || total != 240000000 {
		t.Fatal(n, total, e)
	}
	t.Log("streamed >100MiB JSON, imported4000 liquids, verified24000 values/240000000 characters")
}
