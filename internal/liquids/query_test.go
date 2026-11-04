package liquids

import (
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"strings"
	"testing"
)

func TestQueryLiteralUnicodeAndNumericSort(t *testing.T) {
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
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Number", Type: "number", Visibility: fields.Visibility{Form: true}}, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`"10"`, `"2"`} {
		name := "Öl 100%_"
		_, e = Save(ctx, s.DB, u.CollectionID, "", Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "number", Value: []byte(raw)}}}, nil)
		if e != nil {
			t.Fatal(e)
		}
	}
	name := "Other 100XX"
	Save(ctx, s.DB, u.CollectionID, "", Input{Name: &name}, nil)
	p, e := List(ctx, s.DB, u.CollectionID, Query{Search: "ÖL 100%_", Sort: "field:" + d.ID, Direction: "asc", Page: 1, PageSize: 24})
	if e != nil {
		t.Fatal(e)
	}
	if p.Total != 2 || string(p.Items[0].Values[d.ID].Value) != `"2"` {
		t.Fatalf("query=%+v", p)
	}
	p, e = List(ctx, s.DB, u.CollectionID, Query{Search: "%_", Page: 1, PageSize: 24})
	if e != nil || p.Total != 2 {
		t.Fatal(p, e)
	}
}
func TestFilterRejectsDepthAndWrongOperators(t *testing.T) {
	d := fields.Definition{ID: "a", Type: "color"}
	_, _, e := BuildFilter(Filter{FieldID: "a", Operator: "gte", Value: []byte(`"#FFFFFF"`)}, map[string]fields.Definition{"a": d})
	if e == nil {
		t.Fatal("color numeric comparison accepted")
	}
	f := Filter{FieldID: "a", Operator: "empty"}
	for i := 0; i < 4; i++ {
		f = Filter{Op: "and", Conditions: []Filter{f}}
	}
	if _, _, e = BuildFilter(f, map[string]fields.Definition{"a": d}); e == nil {
		t.Fatal("depth accepted")
	}
}

func TestNamesAndTextSortAlphabeticallyInGerman(t *testing.T) {
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
	d, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Hersteller", Type: "text"}, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"zitrone", "Äpfel", "Orange", "Öl", "apfelmus", "Banane"} {
		value := name
		raw, _ := json.Marshal(value)
		if _, e = Save(ctx, s.DB, u.CollectionID, "", Input{Name: &value, Values: map[string]*fields.Value{d.ID: {Type: "text", Value: raw}}}, nil); e != nil {
			t.Fatal(e)
		}
	}
	for _, sort := range []string{"name", "field:" + d.ID} {
		page, e := List(ctx, s.DB, u.CollectionID, Query{Sort: sort, Direction: "asc", Page: 1, PageSize: 24})
		if e != nil {
			t.Fatal(e)
		}
		names := []string{}
		for _, l := range page.Items {
			names = append(names, l.Name)
		}
		if got := strings.Join(names, ","); got != "Äpfel,apfelmus,Banane,Öl,Orange,zitrone" {
			t.Fatalf("sort %s: %s", sort, got)
		}
	}
}
