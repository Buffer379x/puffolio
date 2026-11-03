package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func reviewStore(t *testing.T) *store.Store {
	t.Helper()
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func reviewUser(t *testing.T, s *store.Store, name string) auth.User {
	t.Helper()
	u, e := auth.New(s).CreateUser(context.Background(), name, name, "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(`INSERT INTO user_preferences(user_id,data) VALUES(?,?)`, u.ID, `{"theme":"system","timezone":"UTC","view":"grid","images":true,"listFields":[],"gridFields":[],"roles":{}}`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(`INSERT INTO dashboards(user_id,data) VALUES(?,'[]')`, u.ID); e != nil {
		t.Fatal(e)
	}
	return u
}
func reviewExport(t *testing.T, s *store.Store, cid string) string {
	t.Helper()
	var b bytes.Buffer
	if e := Export(context.Background(), s, cid, &b, "test"); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "collection.zip")
	if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Preview(p, WebLimits()); e != nil {
		t.Fatal("own export invalid", e)
	}
	return p
}
func reviewRequired(t *testing.T, s *store.Store, cid, label, cutoff string) fields.Definition {
	t.Helper()
	d, e := fields.Save(context.Background(), s.DB, cid, fields.Definition{Label: label, Type: "text", Required: true, Visibility: fields.Visibility{Form: true}}, true)
	if e != nil {
		t.Fatal(e)
	}
	d.RequiredFrom = cutoff
	raw, _ := json.Marshal(d)
	if _, e = s.DB.Exec("UPDATE field_definitions SET data=? WHERE collection_id=? AND id=?", string(raw), cid, d.ID); e != nil {
		t.Fatal(e)
	}
	return d
}
func reviewLiquid(t *testing.T, s *store.Store, cid string, d *fields.Definition) liquids.Liquid {
	t.Helper()
	name := "Existing"
	in := liquids.Input{Name: &name}
	if d != nil {
		in.Values = map[string]*fields.Value{d.ID: {Type: d.Type, Value: json.RawMessage(`"Present"`)}}
	}
	l, e := liquids.Save(context.Background(), s.DB, cid, "", in, nil)
	if e != nil {
		t.Fatal(e)
	}
	return l
}
func reviewState(t *testing.T, s *store.Store, u auth.User) string {
	t.Helper()
	state := map[string]any{}
	for _, table := range []string{"field_definitions", "liquids", "field_values", "liquid_tags", "tags", "media"} {
		var n int
		if e := s.DB.QueryRow("SELECT count(*) FROM "+table+" WHERE collection_id=?", u.CollectionID).Scan(&n); e != nil {
			t.Fatal(e)
		}
		state[table] = n
	}
	var views, schema int
	s.DB.QueryRow("SELECT count(*) FROM saved_views WHERE user_id=?", u.ID).Scan(&views)
	s.DB.QueryRow("SELECT schema_version FROM collections WHERE id=?", u.CollectionID).Scan(&schema)
	state["views"], state["schema"] = views, schema
	for _, table := range []string{"user_preferences", "dashboards"} {
		var version int
		var data string
		if e := s.DB.QueryRow("SELECT version,data FROM "+table+" WHERE user_id=?", u.ID).Scan(&version, &data); e != nil {
			t.Fatal(e)
		}
		state[table] = []any{version, data}
	}
	rows, e := s.DB.Query("SELECT id,name,version FROM liquids WHERE collection_id=? ORDER BY id", u.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	records := []any{}
	for rows.Next() {
		var id, name string
		var v int
		rows.Scan(&id, &name, &v)
		records = append(records, []any{id, name, v})
	}
	rows.Close()
	state["records"] = records
	entries, e := os.ReadDir(filepath.Join(s.Dir, "media"))
	if e != nil {
		t.Fatal(e)
	}
	files := []string{}
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	state["mediaPaths"] = files
	raw, _ := json.Marshal(state)
	return string(raw)
}
func reviewRejectImport(t *testing.T, s *store.Store, u auth.User, path string) {
	t.Helper()
	before := reviewState(t, s, u)
	if _, e := PreviewForCollection(context.Background(), s, u.CollectionID, path, WebLimits()); e == nil || fault.Wrap(e).Status != 422 {
		t.Errorf("incompatible preview should422, got %v", e)
	}
	if _, e := Import(context.Background(), s, u.CollectionID, path, WebLimits()); e == nil || fault.Wrap(e).Status != 422 {
		t.Errorf("incompatible import should422, got %v", e)
	}
	if after := reviewState(t, s, u); after != before {
		t.Fatalf("rejected import mutated destination\nbefore=%s\nafter=%s", before, after)
	}
}
func TestRequiredFieldSelfImportRejectsAtomicallyAndFreshTargetWorks(t *testing.T) {
	s := reviewStore(t)
	source := reviewUser(t, s, "source-user")
	d := reviewRequired(t, s, source.CollectionID, "Source required", "2020-01-01T00:00:00Z")
	original := reviewLiquid(t, s, source.CollectionID, &d)
	p := reviewExport(t, s, source.CollectionID)
	reviewRejectImport(t, s, source, p)
	name := "Still editable"
	if _, e := liquids.Save(context.Background(), s.DB, source.CollectionID, original.ID, liquids.Input{Name: &name, Version: original.Version}, nil); e != nil {
		t.Fatal("existing liquid became uneditable", e)
	}
	reviewExport(t, s, source.CollectionID)
	fresh := reviewUser(t, s, "fresh-user")
	if _, e := PreviewForCollection(context.Background(), s, fresh.CollectionID, p, WebLimits()); e != nil {
		t.Fatal(e)
	}
	if _, e := Import(context.Background(), s, fresh.CollectionID, p, WebLimits()); e != nil {
		t.Fatal(e)
	}
	page, e := liquids.List(context.Background(), s.DB, fresh.CollectionID, liquids.Query{Page: 1, PageSize: 24})
	if e != nil || len(page.Items) != 1 {
		t.Fatal(page, e)
	}
	if _, e = liquids.Save(context.Background(), s.DB, fresh.CollectionID, page.Items[0].ID, liquids.Input{Name: &name, Version: 1}, nil); e != nil {
		t.Fatal(e)
	}
	next := reviewExport(t, s, fresh.CollectionID)
	another := reviewUser(t, s, "another-user")
	if _, e = Import(context.Background(), s, another.CollectionID, next, WebLimits()); e != nil {
		t.Fatal("next export not importable into empty target", e)
	}
}
func TestImportRejectsEitherSideOfIncompatibleRequiredRules(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		sourceRequired, targetRequired, backfill bool
	}{{"incoming required", true, false, false}, {"destination required", false, true, false}, {"distinct required", true, true, false}, {"backfilled required", false, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := reviewStore(t)
			source := reviewUser(t, s, "source-user")
			target := reviewUser(t, s, "target-user")
			var sd, td *fields.Definition
			if tc.sourceRequired {
				d := reviewRequired(t, s, source.CollectionID, "Source required", "2020-01-01T00:00:00Z")
				sd = &d
			}
			if tc.targetRequired {
				d := reviewRequired(t, s, target.CollectionID, "Target required", "2020-01-01T00:00:00Z")
				td = &d
			}
			reviewLiquid(t, s, source.CollectionID, sd)
			reviewLiquid(t, s, target.CollectionID, td)
			if tc.backfill {
				if _, e := fields.CreateWithBackfill(context.Background(), s.DB, source.CollectionID, fields.Definition{Label: "Backfilled", Type: "boolean", Required: true, Visibility: fields.Visibility{Form: true}}, json.RawMessage("false")); e != nil {
					t.Fatal(e)
				}
			}
			p := reviewExport(t, s, source.CollectionID)
			reviewRejectImport(t, s, target, p)
			reviewExport(t, s, target.CollectionID)
		})
	}
}
func seedReviewViews(t *testing.T, s *store.Store, u auth.User, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := store.ID()
		v := map[string]any{"id": id, "name": fmt.Sprintf("View%d", i), "search": "", "favorite": false, "sort": "createdAt", "direction": "desc", "version": 1}
		raw, _ := json.Marshal(v)
		if _, e := s.DB.Exec("INSERT INTO saved_views(id,user_id,version,data) VALUES(?,?,1,?)", id, u.ID, string(raw)); e != nil {
			t.Fatal(e)
		}
	}
}
func TestImportSavedViewCombinedLimitAndExactBoundary(t *testing.T) {
	s := reviewStore(t)
	source := reviewUser(t, s, "source-user")
	seedReviewViews(t, s, source, 60)
	p := reviewExport(t, s, source.CollectionID)
	reviewRejectImport(t, s, source, p)
	target := reviewUser(t, s, "target-user")
	seedReviewViews(t, s, target, 40)
	if _, e := PreviewForCollection(context.Background(), s, target.CollectionID, p, WebLimits()); e != nil {
		t.Fatal(e)
	}
	if _, e := Import(context.Background(), s, target.CollectionID, p, WebLimits()); e != nil {
		t.Fatal("exact100 boundary rejected", e)
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM saved_views WHERE user_id=?", target.ID).Scan(&count)
	if count != 100 {
		t.Fatal(count)
	}
	next := reviewExport(t, s, target.CollectionID)
	empty := reviewUser(t, s, "empty-user")
	if _, e := Import(context.Background(), s, empty.CollectionID, next, WebLimits()); e != nil {
		t.Fatal("100-view export not importable into empty target", e)
	}
}
