package fields

import (
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"slices"
	"testing"
)

func TestPaletteHasThirtyTwoDistinctColours(t *testing.T) {
	if len(PaletteIDs) != 32 {
		t.Fatalf("%d colours", len(PaletteIDs))
	}
	seen := map[string]bool{}
	for _, id := range PaletteIDs {
		if seen[id] || !ValidPalette(id) {
			t.Fatalf("%q duplicate or invalid", id)
		}
		seen[id] = true
	}
	for _, id := range []string{"", "#ffffff", "Lime", "lime "} {
		if ValidPalette(id) {
			t.Fatalf("%q accepted", id)
		}
	}
}
func TestPaletteMatchesAPIContract(t *testing.T) {
	raw, e := os.ReadFile("../../docs/api/openapi.yaml")
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		Components struct {
			Schemas struct {
				Palette struct {
					Enum []string `json:"enum"`
				} `json:"Palette"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if e = json.Unmarshal(raw, &spec); e != nil {
		t.Fatal(e)
	}
	if !slices.Equal(spec.Components.Schemas.Palette.Enum, PaletteIDs) {
		t.Fatalf("contract %v, code %v", spec.Components.Schemas.Palette.Enum, PaletteIDs)
	}
}
func TestDatabaseAcceptsExactlyThePalette(t *testing.T) {
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, statement := range []string{
		`INSERT INTO users(id,username,display_name,password_hash,role,created_at) VALUES('u','alice','Alice','x','admin','now')`,
		`INSERT INTO collections(id,user_id) VALUES('c','u')`,
		`INSERT INTO field_definitions(id,collection_id,type,data) VALUES('f','c','tags','{}')`,
	} {
		if _, e = s.DB.Exec(statement); e != nil {
			t.Fatal(e)
		}
	}
	insert := "INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,'c','f',?,?,?)"
	for _, id := range PaletteIDs {
		if _, e = s.DB.Exec(insert, id, id, id, id); e != nil {
			t.Fatalf("%s: %v", id, e)
		}
	}
	if _, e = s.DB.Exec(insert, "unknown", "unknown", "unknown", "unknown"); e == nil {
		t.Fatal("database accepted a colour outside the palette")
	}
}
