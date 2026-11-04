package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/dashboard"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/media"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestAllTypesMediaTagsAndWidgetRoundtrip(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "alice", "Alice", "admin", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	roles, e := fields.Starter(ctx, s.DB, u.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "YesNo", Type: "boolean"}, true)
	numberField := ""
	starter, e := fields.List(ctx, s.DB, u.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range starter {
		if d.Type == "number" && numberField == "" {
			numberField = d.ID
		}
	}
	defs, e := fields.List(ctx, s.DB, u.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	var pngbytes bytes.Buffer
	png.Encode(&pngbytes, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	im, e := media.New(s).Upload(ctx, u.CollectionID, &pngbytes)
	if e != nil {
		t.Fatal(e)
	}
	values := map[string]*fields.Value{}
	origIDs := map[string]bool{}
	var tagID string
	for _, d := range defs {
		origIDs[d.ID] = true
		var value any
		switch d.Type {
		case "text":
			value = "Äpfel %_"
		case "longtext":
			value = "line1\nline2"
		case "rating":
			value = 4
		case "number":
			value = "0.0"
			if d.Config.Scale == 0 {
				value = "24"
			}
		case "date":
			value = "2028-02-29"
		case "boolean":
			value = false
		case "color":
			value = "#ABCDEF"
		case "select":
			value = d.Config.Options[0].ID
		case "image":
			value = im.ID
		case "tags":
			tagID = store.ID()
			s.DB.Exec("INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,?)", tagID, u.CollectionID, d.ID, "Tag", "tag", "teal")
			value = []string{tagID}
		}
		raw, _ := json.Marshal(value)
		values[d.ID] = &fields.Value{Type: d.Type, Value: raw}
	}
	name := "All types"
	source, e := liquids.Save(ctx, s.DB, u.CollectionID, "", liquids.Input{Name: &name, Values: values}, nil)
	if e != nil {
		t.Fatal(e)
	}
	prefs := map[string]any{"theme": "dark", "timezone": "Pacific/Kiritimati", "view": "list", "images": true, "listFields": []string{roles["image"]}, "gridFields": []string{}, "roles": roles}
	raw, _ := json.Marshal(prefs)
	s.DB.Exec("INSERT INTO user_preferences(user_id,data) VALUES(?,?)", u.ID, string(raw))
	var selectField, selectOption string
	for _, d := range defs {
		if d.Type == "select" {
			selectField = d.ID
			selectOption = d.Config.Options[0].ID
		}
	}
	filterValue, _ := json.Marshal([]string{selectOption})
	widgets := []dashboard.Widget{{ID: store.ID(), Type: "sum", Size: "small", Title: "Amount", FieldID: numberField, Filters: &liquids.Filter{Op: "and", Conditions: []liquids.Filter{{FieldID: selectField, Operator: "any", Value: filterValue}, {FieldID: "favorite", Operator: "eq", Value: json.RawMessage("false")}}}}}
	raw, _ = json.Marshal(widgets)
	s.DB.Exec("INSERT INTO dashboards(user_id,data) VALUES(?,?)", u.ID, string(raw))
	view := map[string]any{"id": store.ID(), "name": "Number", "search": "", "sort": "field:" + numberField, "direction": "asc", "version": 1, "favorite": true}
	raw, _ = json.Marshal(view)
	s.DB.Exec("INSERT INTO saved_views(id,user_id,data) VALUES(?,?,?)", view["id"], u.ID, string(raw))
	var archive bytes.Buffer
	if e = Export(ctx, s, u.CollectionID, &archive, "test"); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "all.zip")
	os.WriteFile(p, archive.Bytes(), 0600)
	if _, e = Import(ctx, s, u.CollectionID, p, WebLimits()); e != nil {
		t.Fatal(e)
	}
	page, e := liquids.List(ctx, s.DB, u.CollectionID, liquids.Query{Page: 1, PageSize: 24})
	if e != nil || page.Total != 2 {
		t.Fatal(page, e)
	}
	var imported liquids.Liquid
	for _, l := range page.Items {
		if l.ID != source.ID {
			imported = l
		}
	}
	if len(imported.Values) != 11 {
		t.Fatal("missing values", len(imported.Values))
	}
	for fid, v := range imported.Values {
		if origIDs[fid] {
			t.Fatal("field ID not remapped")
		}
		switch v.Type {
		case "number":
			if string(v.Value) != `"0.0"` && string(v.Value) != `"24"` {
				t.Fatal("zero changed")
			}
		case "boolean":
			if string(v.Value) != "false" {
				t.Fatal("false lost")
			}
		case "date":
			if string(v.Value) != `"2028-02-29"` {
				t.Fatal("date changed")
			}
		case "image":
			var id string
			json.Unmarshal(v.Value, &id)
			if id == im.ID {
				t.Fatal("media not remapped")
			}
			a, _ := os.ReadFile(media.New(s).Path(im.ID, "master"))
			b, _ := os.ReadFile(media.New(s).Path(id, "master"))
			if sha256.Sum256(a) != sha256.Sum256(b) {
				t.Fatal("master bytes changed")
			}
		case "tags":
			var ids []string
			json.Unmarshal(v.Value, &ids)
			if len(ids) != 1 || ids[0] == tagID {
				t.Fatal("tag not remapped")
			}
		}
	}
	var importedViewRaw string
	if e = s.DB.QueryRow("SELECT data FROM saved_views WHERE user_id=? AND id!=?", u.ID, view["id"]).Scan(&importedViewRaw); e != nil {
		t.Fatal(e)
	}
	var importedView map[string]any
	json.Unmarshal([]byte(importedViewRaw), &importedView)
	if importedView["favorite"] != true {
		t.Fatal("saved favorite lost on import")
	}
	var dashboardRaw string
	s.DB.QueryRow("SELECT data FROM dashboards WHERE user_id=?", u.ID).Scan(&dashboardRaw)
	var got []dashboard.Widget
	json.Unmarshal([]byte(dashboardRaw), &got)
	results, e := dashboard.Query(ctx, s.DB, u.CollectionID, got)
	if e != nil || results[0].Count == nil || *results[0].Count != 1 {
		t.Fatalf("imported select filter did not match: %+v %v", results, e)
	}
	if len(got) != 1 || got[0].FieldID == numberField {
		t.Fatal("widget field reference not remapped")
	}
}
