package liquids_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/dashboard"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
)

type rangeFixture struct {
	s             *store.Store
	cid, otherCID string
	defs          map[string]fields.Definition
	byID          map[string]fields.Definition
	tags          []string
	ids           []string
}

func rawValue(v any) json.RawMessage {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func rangeTestFixture(t *testing.T) *rangeFixture {
	t.Helper()
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	f := &rangeFixture{s: s}
	for owner, n := range []int{1000, 10} {
		u, e := auth.New(s).CreateUser(ctx, fmt.Sprintf("range-user%d", owner), "Range", "user", "A-long-test-password")
		if e != nil {
			t.Fatal(e)
		}
		defs := map[string]fields.Definition{}
		ordered := []fields.Definition{}
		for i := 0; i < 30; i++ {
			typ, key := "text", fmt.Sprintf("text%d", i)
			switch i {
			case 0:
				typ, key = "date", "date"
			case 1:
				typ, key = "number", "number"
			case 2:
				typ, key = "rating", "rating"
			case 3, 4, 5:
				typ, key = "tags", fmt.Sprintf("tags%d", i-3)
			}
			d := fields.Definition{Type: typ, Label: key, Config: fields.Config{Filterable: true}}
			if typ == "number" {
				d.Config.Scale = 1
			}
			if typ == "rating" {
				d.Config.Stars = 5
			}
			d, e = fields.Save(ctx, s.DB, u.CollectionID, d, true)
			if e != nil {
				t.Fatal(e)
			}
			defs[key] = d
			ordered = append(ordered, d)
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		tagIDs := map[string][]string{}
		for _, d := range ordered {
			if d.Type != "tags" {
				continue
			}
			for j := 0; j < 2; j++ {
				id := store.ID()
				name := fmt.Sprintf("Tag%d", j)
				if _, e = tx.Exec("INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,'teal')", id, u.CollectionID, d.ID, name, fields.Normalize(name)); e != nil {
					t.Fatal(e)
				}
				tagIDs[d.ID] = append(tagIDs[d.ID], id)
			}
		}
		valueStmt, e := tx.PrepareContext(ctx, "INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_text,value_search,value_number) VALUES(?,?,?,?,?,?,?)")
		if e != nil {
			t.Fatal(e)
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		ids := []string{}
		for i := 0; i < n; i++ {
			id := store.ID()
			ids = append(ids, id)
			name := fmt.Sprintf("Ä %%_ %04d", i)
			if _, e = tx.Exec("INSERT INTO liquids(id,collection_id,name,normalized_name,favorite,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", id, u.CollectionID, name, fields.Normalize(name), i%2 == 0, now, now); e != nil {
				t.Fatal(e)
			}
			for _, d := range ordered {
				switch d.Type {
				case "tags":
					if _, e = tx.Exec("INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES(?,?,?,?)", u.CollectionID, id, d.ID, tagIDs[d.ID][i%2]); e != nil {
						t.Fatal(e)
					}
				case "number":
					_, e = valueStmt.ExecContext(ctx, u.CollectionID, id, d.ID, d.Type, nil, nil, i)
				case "rating":
					_, e = valueStmt.ExecContext(ctx, u.CollectionID, id, d.ID, d.Type, nil, nil, i%5+1)
				case "date":
					_, e = valueStmt.ExecContext(ctx, u.CollectionID, id, d.ID, d.Type, "2026-06-15", "2026-06-15", nil)
				default:
					_, e = valueStmt.ExecContext(ctx, u.CollectionID, id, d.ID, d.Type, "Populated field", "populated field", nil)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		valueStmt.Close()
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
		if owner == 0 {
			f.cid = u.CollectionID
			f.defs = defs
			f.byID = map[string]fields.Definition{}
			for _, d := range ordered {
				f.byID[d.ID] = d
			}
			f.ids = ids
			f.tags = tagIDs[defs["tags0"].ID]
		} else {
			f.otherCID = u.CollectionID
		}
	}
	return f
}
func rangeLeaf(f *rangeFixture, key, operator string, v, v2 any) liquids.Filter {
	leaf := liquids.Filter{FieldID: f.defs[key].ID, Operator: operator}
	if v != nil {
		leaf.Value = rawValue(v)
	}
	if v2 != nil {
		leaf.Value2 = rawValue(v2)
	}
	return leaf
}
func dateWindow(f *rangeFixture) liquids.Filter {
	return rangeLeaf(f, "date", "between", "2026-01-01", "2026-12-31")
}

func TestRangeFilterCountPlanUsesPerLiquidFieldLookup(t *testing.T) {
	f := rangeTestFixture(t)
	ctx := context.Background()
	cases := []struct {
		name   string
		filter liquids.Filter
		count  int
	}{
		{"date/between", dateWindow(f), 1000}, {"date/before", rangeLeaf(f, "date", "before", "2027-01-01", nil), 1000}, {"date/after", rangeLeaf(f, "date", "after", "2025-12-31", nil), 1000},
		{"number/between", rangeLeaf(f, "number", "between", "0.0", "99.9"), 1000}, {"number/gte", rangeLeaf(f, "number", "gte", "99.0", nil), 10}, {"number/lte", rangeLeaf(f, "number", "lte", "0.9", nil), 10},
		{"rating/between", rangeLeaf(f, "rating", "between", 1, 5), 1000}, {"rating/gte", rangeLeaf(f, "rating", "gte", 4, nil), 400}, {"rating/lte", rangeLeaf(f, "rating", "lte", 2, nil), 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clause, args, e := liquids.BuildFilter(tc.filter, f.byID)
			if e != nil {
				t.Fatal(e)
			}
			query := "SELECT count(*) FROM liquids l WHERE l.collection_id=? AND l.deleted_at IS NULL AND (" + clause + ")"
			args = append([]any{f.cid}, args...)
			var count int
			if e = f.s.DB.QueryRowContext(ctx, query, args...).Scan(&count); e != nil {
				t.Fatal(e)
			}
			if count != tc.count {
				t.Fatalf("count=%d expected%d", count, tc.count)
			}
			rows, e := f.s.DB.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
			if e != nil {
				t.Fatal(e)
			}
			bounded := false
			plan := []string{}
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if e = rows.Scan(&id, &parent, &unused, &detail); e != nil {
					rows.Close()
					t.Fatal(e)
				}
				plan = append(plan, detail)
				if strings.Contains(detail, "SEARCH fv") && strings.Contains(detail, "collection_id=? AND liquid_id=? AND field_id=?") {
					bounded = true
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				t.Fatal(e)
			}
			t.Logf("count=%d plan=%v", count, plan)
			if !bounded {
				t.Fatal("range COUNT must look up the outer liquid's single field value; repeated field-range scan remains")
			}
		})
	}
}

func TestNestedRangeFiltersPreserveSearchTagsOwnersAndPagination(t *testing.T) {
	f := rangeTestFixture(t)
	ctx := context.Background()
	name := "Ä %_ missing"
	missing, e := liquids.Save(ctx, f.s.DB, f.cid, "", liquids.Input{Name: &name}, nil)
	if e != nil {
		t.Fatal(e)
	}
	nested := liquids.Filter{Op: "and", Conditions: []liquids.Filter{dateWindow(f), {Op: "or", Conditions: []liquids.Filter{rangeLeaf(f, "number", "gte", "99.5", nil), rangeLeaf(f, "tags0", "any", []string{f.tags[0]}, nil)}}, {FieldID: "favorite", Operator: "eq", Value: rawValue(false)}}}
	p, e := liquids.List(ctx, f.s.DB, f.cid, liquids.Query{Search: "ä %_", Filters: &nested, Sort: "field:" + f.defs["number"].ID, Direction: "asc", Page: 1, PageSize: 96})
	if e != nil {
		t.Fatal(e)
	}
	if p.Total != 3 || len(p.Items) != 3 {
		t.Fatal(p.Total, len(p.Items))
	}
	for i, want := range []string{"Ä %_ 0995", "Ä %_ 0997", "Ä %_ 0999"} {
		if p.Items[i].Name != want {
			t.Fatal("nested/normalized result mismatch", p.Items[i].Name, want)
		}
	}
	cases := []struct {
		name   string
		filter liquids.Filter
		want   int
	}{
		{"zero", rangeLeaf(f, "number", "eq", "0.0", nil), 1}, {"number-empty", rangeLeaf(f, "number", "empty", nil, nil), 1}, {"number-present", rangeLeaf(f, "number", "notEmpty", nil, nil), 1000},
		{"date-empty", rangeLeaf(f, "date", "empty", nil, nil), 1}, {"date-present", rangeLeaf(f, "date", "notEmpty", nil, nil), 1000}, {"rating-empty", rangeLeaf(f, "rating", "empty", nil, nil), 1},
		{"tags-any", liquids.Filter{Op: "and", Conditions: []liquids.Filter{dateWindow(f), rangeLeaf(f, "tags0", "any", []string{f.tags[0]}, nil)}}, 500},
		{"tags-all", liquids.Filter{Op: "and", Conditions: []liquids.Filter{dateWindow(f), rangeLeaf(f, "tags0", "all", f.tags, nil)}}, 0},
		{"tags-none", liquids.Filter{Op: "and", Conditions: []liquids.Filter{dateWindow(f), rangeLeaf(f, "tags0", "none", []string{f.tags[0]}, nil)}}, 500},
		{"range-and-tags", liquids.Filter{Op: "and", Conditions: []liquids.Filter{rangeLeaf(f, "number", "between", "0.0", "0.9"), rangeLeaf(f, "tags0", "none", []string{f.tags[0]}, nil)}}, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, e := liquids.List(ctx, f.s.DB, f.cid, liquids.Query{Filters: &tc.filter, Page: 1, PageSize: 24})
			if e != nil || p.Total != tc.want {
				t.Fatal(p.Total, tc.want, e)
			}
		})
	}
	for _, direction := range []string{"asc", "desc"} {
		p, e := liquids.List(ctx, f.s.DB, f.cid, liquids.Query{Search: "Ä %_", Sort: "field:" + f.defs["number"].ID, Direction: direction, Page: 11, PageSize: 96})
		if e != nil || p.Total != 1001 || len(p.Items) != 41 || p.Items[40].ID != missing.ID {
			t.Fatal("NULL-last pagination changed", direction, p.Total, len(p.Items), e)
		}
	}
	ratingFive := rangeLeaf(f, "rating", "eq", 5, nil)
	orderedIDs := []string{}
	for i, id := range f.ids {
		if i%5 == 4 {
			orderedIDs = append(orderedIDs, id)
		}
	}
	sort.Strings(orderedIDs)
	for _, page := range []int{1, 2, 1} {
		p, e := liquids.List(ctx, f.s.DB, f.cid, liquids.Query{Filters: &ratingFive, Sort: "field:" + f.defs["rating"].ID, Direction: "desc", Page: page, PageSize: 96})
		if e != nil || p.Total != 200 || len(p.Items) != 96 {
			t.Fatal(p.Total, len(p.Items), e)
		}
		for i, l := range p.Items {
			if l.ID != orderedIDs[(page-1)*96+i] {
				t.Fatal("stable tie pagination changed")
			}
		}
	}
	foreignFields, e := fields.List(ctx, f.s.DB, f.otherCID)
	if e != nil {
		t.Fatal(e)
	}
	foreign := liquids.Filter{FieldID: foreignFields[0].ID, Operator: "notEmpty"}
	if _, e = liquids.List(ctx, f.s.DB, f.cid, liquids.Query{Filters: &foreign, Page: 1, PageSize: 24}); e == nil {
		t.Fatal("foreign field accepted")
	}
	invalid := rangeLeaf(f, "date", "contains", "2026", nil)
	if _, _, e = liquids.BuildFilter(invalid, f.byID); e == nil {
		t.Fatal("invalid date operator accepted")
	}
	own, e := liquids.List(ctx, f.s.DB, f.otherCID, liquids.Query{Search: "Ä %_", Page: 1, PageSize: 96})
	if e != nil || own.Total != 10 {
		t.Fatal("owner results changed", own.Total, e)
	}
}

func TestRangeFilteredDashboardAggregatesKeepTheirMeaning(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "widget-user", "Widget", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	defs := map[string]fields.Definition{}
	for _, typ := range []string{"number", "date", "rating", "tags"} {
		d := fields.Definition{Label: typ, Type: typ, Config: fields.Config{Scale: 1, Stars: 5}}
		d, e = fields.Save(ctx, s.DB, u.CollectionID, d, true)
		if e != nil {
			t.Fatal(e)
		}
		defs[typ] = d
	}
	tag := store.ID()
	if _, e = s.DB.Exec("INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,'Selected','selected','teal')", tag, u.CollectionID, defs["tags"].ID); e != nil {
		t.Fatal(e)
	}
	for i, row := range []struct {
		number, date string
		rating       int
		tagged       bool
	}{{"2.5", "2026-06-15", 4, true}, {"10.0", "2026-06-15", 0, false}, {"99.0", "2027-01-01", 5, true}} {
		name := fmt.Sprintf("Widget%d", i)
		favorite := i == 0
		values := map[string]*fields.Value{defs["number"].ID: {Type: "number", Value: rawValue(row.number)}, defs["date"].ID: {Type: "date", Value: rawValue(row.date)}}
		if row.rating > 0 {
			values[defs["rating"].ID] = &fields.Value{Type: "rating", Value: rawValue(row.rating)}
		}
		if row.tagged {
			values[defs["tags"].ID] = &fields.Value{Type: "tags", Value: rawValue([]string{tag})}
		}
		if _, e = liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Favorite: &favorite, Values: values}, nil); e != nil {
			t.Fatal(e)
		}
	}
	filter := &liquids.Filter{Op: "and", Conditions: []liquids.Filter{{FieldID: defs["date"].ID, Operator: "between", Value: rawValue("2026-01-01"), Value2: rawValue("2026-12-31")}, {FieldID: defs["number"].ID, Operator: "gte", Value: rawValue("0.0")}}}
	widgets := []dashboard.Widget{{ID: "count", Type: "count", Filters: filter}, {ID: "sum", Type: "sum", FieldID: defs["number"].ID, Filters: filter}, {ID: "average", Type: "average", FieldID: defs["rating"].ID, Filters: filter}, {ID: "favorites", Type: "favorites", Filters: filter}, {ID: "tags", Type: "tags", FieldID: defs["tags"].ID, Filters: filter}, {ID: "low", Type: "low", FieldID: defs["number"].ID, Threshold: "5.0", Inclusive: true, Filters: filter}, {ID: "recent", Type: "recent", Limit: 5, Filters: filter}, {ID: "unfiltered", Type: "count"}}
	results, e := dashboard.Query(ctx, s.DB, u.CollectionID, widgets)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range results {
		if r.Error != "" {
			t.Fatal(r.ID, r.Error)
		}
	}
	if *results[0].Count != 2 || results[1].Value != "12.5" || *results[1].Count != 2 || results[2].Value != "4.00" || *results[2].Count != 1 || *results[3].Count != 1 || len(results[4].Tags) != 1 || results[4].Tags[0].Count != 1 || *results[5].Count != 1 || len(results[6].Items) != 2 || *results[7].Count != 3 {
		t.Fatalf("filtered aggregates changed: %+v", results)
	}
}
