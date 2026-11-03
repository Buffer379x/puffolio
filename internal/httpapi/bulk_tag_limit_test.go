package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestBulkTagResultLimitIsAtomicAndCountsOnlyNewTags(t *testing.T) {
	c, s := setupClient(t)
	me := c.do("GET", "/api/v1/me", nil, 200)
	cid := me["collectionId"].(string)
	defs, e := fields.List(context.Background(), s.DB, cid)
	if e != nil {
		t.Fatal(e)
	}
	fid := ""
	for _, d := range defs {
		if d.Type == "tags" {
			fid = d.ID
		}
	}
	if fid == "" {
		t.Fatal("missing tags field")
	}
	ids := []string{}
	tx, e := s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 201; i++ {
		id := store.ID()
		name := fmt.Sprintf("Tag%03d", i)
		if _, e = tx.Exec("INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,'teal')", id, cid, fid, name, fields.Normalize(name)); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	a := c.do("POST", "/api/v1/liquids", map[string]any{"name": "At limit", "values": map[string]any{fid: map[string]any{"type": "tags", "value": ids[:200]}}}, 201)
	b := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Must stay unchanged"}, 201)
	aid, bid := a["id"].(string), b["id"].(string)
	c.do("POST", "/api/v1/liquids/bulk", map[string]any{"ids": []string{bid, aid}, "action": "tag", "fieldId": fid, "tagIds": ids[200:]}, 422)
	for _, id := range []string{aid, bid} {
		l := c.do("GET", "/api/v1/liquids/"+id, nil, 200)
		if l["version"] != float64(1) {
			t.Fatal("rejected batch changed version", id)
		}
		var count int
		s.DB.QueryRow("SELECT count(*) FROM liquid_tags WHERE collection_id=? AND liquid_id=? AND field_id=?", cid, id, fid).Scan(&count)
		want := 0
		if id == aid {
			want = 200
		}
		if count != want {
			t.Fatal("rejected batch changed assignments", count, want)
		}
	}
	c.do("POST", "/api/v1/liquids/bulk", map[string]any{"ids": []string{aid}, "action": "tag", "fieldId": fid, "tagIds": []string{ids[0], ids[199]}}, 200)
	var count int
	s.DB.QueryRow("SELECT count(*) FROM liquid_tags WHERE collection_id=? AND liquid_id=? AND field_id=?", cid, aid, fid).Scan(&count)
	if count != 200 {
		t.Fatal("already-present tags counted twice", count)
	}
	var archive bytes.Buffer
	if e = backup.Export(context.Background(), s, cid, &archive, "test"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "tags.zip")
	os.WriteFile(path, archive.Bytes(), 0600)
	if _, e = backup.Preview(path, backup.WebLimits()); e != nil {
		t.Fatal("accepted tag set cannot pass own export preview", e)
	}
}
