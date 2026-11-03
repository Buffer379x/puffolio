package qa

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/dashboard"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"testing"
	"time"
)

func TestFiveThousandLiquidsAndTenWidgets(t *testing.T) {
	if os.Getenv("PUFFOLIO_PERF") != "1" {
		t.Skip("set PUFFOLIO_PERF=1 for5000-liquid timing")
	}
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
	defs := map[string]fields.Definition{}
	for _, typ := range []string{"number", "rating", "date", "tags"} {
		d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: typ, Type: typ}, true)
		if e != nil {
			t.Fatal(e)
		}
		defs[typ] = d
	}
	tag := store.ID()
	tx, e := s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	tx.Exec(`INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,?)`, tag, u.CollectionID, defs["tags"].ID, "Tag", "tag", "teal")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < 5000; i++ {
		id := store.ID()
		name := fmt.Sprintf("Liquid Ä %s %04d", "%_", i)
		if _, e = tx.Exec(`INSERT INTO liquids(id,collection_id,name,normalized_name,favorite,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, u.CollectionID, name, fields.Normalize(name), i%3 == 0, now, now); e != nil {
			t.Fatal(e)
		}
		tx.Exec(`INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_number) VALUES(?,?,?,'number',?)`, u.CollectionID, id, defs["number"].ID, i)
		tx.Exec(`INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_text,value_search) VALUES(?,?,?,'date','2026-01-01','2026-01-01')`, u.CollectionID, id, defs["date"].ID)
		if i%2 == 0 {
			tx.Exec(`INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_number) VALUES(?,?,?,'rating',4)`, u.CollectionID, id, defs["rating"].ID)
			tx.Exec(`INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES(?,?,?,?)`, u.CollectionID, id, defs["tags"].ID, tag)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	p, e := liquids.List(ctx, s.DB, u.CollectionID, liquids.Query{Search: "ä %_", Sort: "field:" + defs["number"].ID, Direction: "desc", Page: 1, PageSize: 96})
	queryElapsed := time.Since(start)
	if e != nil || p.Total != 5000 || len(p.Items) != 96 || string(p.Items[0].Values[defs["number"].ID].Value) != `"4999"` {
		t.Fatal(p.Total, len(p.Items), e)
	}
	widgets := []dashboard.Widget{{ID: "1", Type: "count"}, {ID: "2", Type: "favorites"}, {ID: "3", Type: "sum", FieldID: defs["number"].ID}, {ID: "4", Type: "average", FieldID: defs["rating"].ID}, {ID: "5", Type: "recent", Limit: 5}, {ID: "6", Type: "low", FieldID: defs["number"].ID, Threshold: "5", Inclusive: true}, {ID: "7", Type: "tags", FieldID: defs["tags"].ID}, {ID: "8", Type: "dates", FieldID: defs["date"].ID, From: "2026-01-01", To: "2026-01-01"}, {ID: "9", Type: "dates", FieldID: defs["date"].ID, From: "2027-01-01", To: "2027-01-01"}, {ID: "10", Type: "count"}}
	start = time.Now()
	results, e := dashboard.Query(ctx, s.DB, u.CollectionID, widgets)
	dashboardElapsed := time.Since(start)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range results {
		if r.Error != "" {
			t.Fatal(r.ID, r.Error)
		}
	}
	if results[2].Value != "12497500" || results[3].Value != "4.00" || *results[5].Count != 6 || *results[7].Count != 5000 || *results[8].Count != 0 {
		raw, _ := json.Marshal(results)
		t.Fatal(string(raw))
	}
	if os.Getenv("PUFFOLIO_PERF_TRACE") == "1" {
		for _, w := range widgets {
			start := time.Now()
			_, e := dashboard.Query(ctx, s.DB, u.CollectionID, []dashboard.Widget{w})
			t.Logf("widget %s=%s err=%v", w.Type, time.Since(start), e)
		}
		rows, e := s.DB.Query(`EXPLAIN QUERY PLAN SELECT count(*) FROM liquids l WHERE l.collection_id=? AND l.deleted_at IS NULL AND EXISTS(SELECT 1 FROM field_values fv WHERE fv.collection_id=l.collection_id AND fv.liquid_id=l.id AND fv.field_id=? AND fv.value_text BETWEEN ? AND ?)`, u.CollectionID, defs["date"].ID, "2026-01-01", "2026-01-01")
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
			var a, b, c int
			var detail string
			rows.Scan(&a, &b, &c, &detail)
			t.Log(detail)
		}
		rows.Close()
	}
	if dashboardElapsed > time.Second {
		t.Fatalf("10-widget query exceeds1s reference budget: %s", dashboardElapsed)
	}
	t.Logf("5000 liquids: searched/sorted96-row page=%s;10 widgets=%s", queryElapsed, dashboardElapsed)
}
