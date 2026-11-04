package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestStatisticWidgets(t *testing.T) {
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
	save := func(d fields.Definition) fields.Definition {
		t.Helper()
		saved, e := fields.Save(ctx, s.DB, u.CollectionID, d, true)
		if e != nil {
			t.Fatal(e)
		}
		return saved
	}
	date, rating := save(fields.Definition{Label: "Kauf", Type: "date"}), save(fields.Definition{Label: "Note", Type: "rating", Config: fields.Config{Stars: 5}})
	maker := save(fields.Definition{Label: "Hersteller", Type: "text"})
	state := save(fields.Definition{Label: "Status", Type: "select", Config: fields.Config{Options: []fields.Option{{Label: "Vorrat"}, {Label: "Leer"}}}})
	tags := save(fields.Definition{Label: "Tags", Type: "tags"})
	tagIDs := []string{}
	for _, name := range []string{"Frucht", "Minze"} {
		id := store.ID()
		if _, e = s.DB.Exec("INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,?)", id, u.CollectionID, tags.ID, name, name, "lime"); e != nil {
			t.Fatal(e)
		}
		tagIDs = append(tagIDs, id)
	}
	add := func(name, day string, stars int, who string, option string, tagged []string) {
		t.Helper()
		values := map[string]*fields.Value{}
		put := func(d fields.Definition, v any) {
			b, _ := json.Marshal(v)
			values[d.ID] = &fields.Value{Type: d.Type, Value: b}
		}
		if day != "" {
			put(date, day)
		}
		if stars > 0 {
			put(rating, stars)
		}
		if who != "" {
			put(maker, who)
		}
		if option != "" {
			put(state, option)
		}
		if tagged != nil {
			put(tags, tagged)
		}
		if _, e := liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: values}, nil); e != nil {
			t.Fatal(name, e)
		}
	}
	stock, empty := state.Config.Options[0].ID, state.Config.Options[1].ID
	add("A", "2026-06-10", 5, "Nordwind", stock, []string{tagIDs[0], tagIDs[1]})
	add("B", "2026-06-20", 3, "NORDWIND", stock, []string{tagIDs[0]})
	add("C", "2025-07-01", 5, "Old Mill", empty, nil)
	add("D", "2025-06-30", 0, "", "", nil)
	rules := &liquids.ShelfRules{PurchaseField: date.ID, DefaultMonths: 12, Today: "2026-06-30"}
	out, e := QueryAt(ctx, s.DB, u.CollectionID, []Widget{
		{ID: "tags", Type: "donut", FieldID: tags.ID, Limit: 5},
		{ID: "maker", Type: "donut", FieldID: maker.ID, Limit: 5},
		{ID: "state", Type: "donut", FieldID: state.ID, Limit: 5},
		{ID: "fold", Type: "donut", FieldID: rating.ID, Limit: 3},
		{ID: "ratings", Type: "ratings", FieldID: rating.ID},
		{ID: "timeline", Type: "timeline", FieldID: date.ID},
		{ID: "shelf", Type: "shelfStatus"},
		{ID: "top", Type: "top", FieldID: rating.ID, Limit: 3},
		{ID: "wrong", Type: "ratings", FieldID: date.ID},
	}, Options{Today: "2026-06-30", Shelf: rules})
	if e != nil {
		t.Fatal(e)
	}
	segments := func(i int) string {
		t.Helper()
		if out[i].Error != "" {
			t.Fatal(out[i].ID, out[i].Error)
		}
		text := ""
		for _, seg := range out[i].Segments {
			text += fmt.Sprintf("%s=%d ", seg.Label, seg.Count)
		}
		return text
	}
	for i, want := range map[int]string{
		0: "Frucht=2 Minze=1 Ohne Angabe=2 ",
		2: "Vorrat=2 Leer=1 Ohne Angabe=1 ",
		3: "5 Sterne=2 3 Sterne=1 Ohne Angabe=1 ",
		4: "1=0 2=0 3=1 4=0 5=2 ",
		6: "Ab 50 %=2 Unter 50 %=0 Unter 20 %=1 Abgelaufen=1 ",
	} {
		if got := segments(i); got != want {
			t.Fatalf("%s: %q want %q", out[i].ID, got, want)
		}
	}
	if got := segments(1); got != "Nordwind=2 Old Mill=1 Ohne Angabe=1 " {
		t.Fatalf("case variants must merge: %q", got)
	}
	months := out[5].Segments
	if len(months) != 12 || months[0].ID != "2025-07" || months[0].Count != 1 || months[11].ID != "2026-06" || months[11].Count != 2 || *out[5].Count != 3 {
		t.Fatalf("timeline: %+v", months)
	}
	if len(out[7].Items) != 3 || out[7].Items[2].Name != "B" {
		t.Fatalf("top: %+v", out[7].Items)
	}
	if out[8].Error == "" {
		t.Fatal("wrong field type accepted")
	}
	none, e := QueryAt(ctx, s.DB, u.CollectionID, []Widget{{ID: "shelf", Type: "shelfStatus"}}, Options{Today: "2026-06-30"})
	if e != nil || none[0].Error != "" || len(none[0].Segments) != 0 {
		t.Fatalf("not set up: %+v %v", none, e)
	}
}
