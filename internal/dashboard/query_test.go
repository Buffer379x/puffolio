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
