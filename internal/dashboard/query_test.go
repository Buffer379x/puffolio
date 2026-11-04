package dashboard

import (
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"testing"
)

func TestDuplicatesAggregatesAndBadReferenceIsolation(t *testing.T) {
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
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Amount", Type: "number", Config: fields.Config{Scale: 1}}, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"2.5", "10.0"} {
		name := "X"
		b, _ := json.Marshal(n)
		if _, e = liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "number", Value: b}}}, nil); e != nil {
			t.Fatal(e)
		}
	}
	out, e := Query(ctx, s.DB, u.CollectionID, []Widget{{ID: "a", Type: "count"}, {ID: "b", Type: "count"}, {ID: "c", Type: "sum", FieldID: d.ID}, {ID: "d", Type: "sum", FieldID: "foreign"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(out) != 4 || out[0].Count == nil || *out[0].Count != 2 || out[1].Count == nil || *out[1].Count != 2 || out[2].Value != "12.5" || out[3].Error == "" {
		t.Fatalf("results=%+v", out)
	}
}

func TestSumSeparatesMissingValuesFromMeasuredZero(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "sum-user", "Sum", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Amount", Type: "number", Config: fields.Config{Scale: 1}}, true)
	if e != nil {
		t.Fatal(e)
	}
	name := "Missing"
	if _, e = liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name}, nil); e != nil {
		t.Fatal(e)
	}
	query := func(want string, count int) {
		t.Helper()
		out, e := Query(ctx, s.DB, u.CollectionID, []Widget{{ID: "sum", Type: "sum", FieldID: d.ID}})
		if e != nil || out[0].Error != "" || out[0].Value != want || out[0].Count == nil || *out[0].Count != count {
			t.Fatalf("sum: %+v %v", out, e)
		}
	}
	query("", 0)
	for i, n := range []string{"0.0", "1.5", "-1.5"} {
		raw, _ := json.Marshal(n)
		if _, e = liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "number", Value: raw}}}, nil); e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			query("0.0", 1)
		}
	}
	query("0.0", 3)
}

func TestRelativeDateWindowsFollowToday(t *testing.T) {
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
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Datum", Type: "date"}, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, day := range []string{"2026-01-01", "2026-06-15", "2026-12-31"} {
		name := day
		b, _ := json.Marshal(day)
		if _, e = liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "date", Value: b}}}, nil); e != nil {
			t.Fatal(e)
		}
	}
	names := func(window, today string) string {
		t.Helper()
		out, e := QueryAt(ctx, s.DB, u.CollectionID, []Widget{{ID: "a", Type: "dates", FieldID: d.ID, Window: window}}, Options{Today: today})
		if e != nil || out[0].Error != "" {
			t.Fatal(e, out[0].Error)
		}
		joined := ""
		for _, item := range out[0].Items {
			joined += item.Name + " "
		}
		return joined
	}
	if got := names("past", "2026-06-15"); got != "2026-06-15 2026-01-01 " {
		t.Fatalf("past, newest first: %q", got)
	}
	if got := names("upcoming", "2026-06-15"); got != "2026-06-15 2026-12-31 " {
		t.Fatalf("upcoming, nearest first: %q", got)
	}
	rules := func(months int) *liquids.ShelfRules {
		return &liquids.ShelfRules{PurchaseField: d.ID, DefaultMonths: months}
	}
	shelf := func(today string, months int) (int, string) {
		t.Helper()
		r := rules(months)
		r.Today = today
		out, e := QueryAt(ctx, s.DB, u.CollectionID, []Widget{{ID: "a", Type: "expired"}, {ID: "b", Type: "expiring"}}, Options{Today: today, Shelf: r})
		if e != nil || out[0].Error != "" || out[1].Error != "" || out[0].Count == nil {
			t.Fatal(e, out[0].Error, out[1].Error)
		}
		order := ""
		for _, item := range out[1].Items {
			order += item.Name + " "
		}
		return *out[0].Count, order
	}
	if count, order := shelf("2027-01-01", 12); count != 1 || order != "2026-01-01 2026-06-15 2026-12-31 " {
		t.Fatalf("exactly one storage time old is expired: %d %q", count, order)
	}
	if count, _ := shelf("2026-12-31", 12); count != 0 {
		t.Fatalf("one day before the end is not expired: %d", count)
	}
	if count, order := shelf("2036-01-01", 0); count != 0 || order != "" {
		t.Fatalf("no storage time: %d %q", count, order)
	}
	none, e := QueryAt(ctx, s.DB, u.CollectionID, []Widget{{ID: "a", Type: "expired"}, {ID: "b", Type: "expiring"}}, Options{Today: "2027-01-01"})
	if e != nil || none[0].Error != "" || *none[0].Count != 0 || len(none[1].Items) != 0 {
		t.Fatalf("not set up: %+v %v", none, e)
	}
	if e := Validate([]Widget{{ID: "a", Type: "dates", Size: "wide", Window: "soon"}}); e == nil {
		t.Fatal("unknown window accepted")
	}
}
