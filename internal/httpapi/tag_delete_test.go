package httpapi

import (
	"bytes"
	"context"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type tagFixture struct {
	c   *client
	s   *store.Store
	fid string
}

func newTagFixture(t *testing.T) (*tagFixture, *client) {
	t.Helper()
	c, s := setupClient(t)
	for _, f := range c.do("GET", "/api/v1/fields", nil, 200)["items"].([]any) {
		if m := f.(map[string]any); m["type"] == "tags" {
			return &tagFixture{c: c, s: s, fid: m["id"].(string)}, c
		}
	}
	t.Fatal("missing tags field")
	return nil, nil
}
func (f *tagFixture) tag(name string) string {
	f.c.t.Helper()
	return f.c.do("POST", "/api/v1/fields/"+f.fid+"/tags", map[string]any{"name": name, "paletteId": "teal"}, 201)["id"].(string)
}
func (f *tagFixture) liquid(name string, tags ...string) map[string]any {
	f.c.t.Helper()
	return f.c.do("POST", "/api/v1/liquids", map[string]any{"name": name, "values": map[string]any{f.fid: map[string]any{"type": "tags", "value": tags}}}, 201)
}
func (f *tagFixture) field() map[string]any {
	f.c.t.Helper()
	for _, item := range f.c.do("GET", "/api/v1/fields", nil, 200)["items"].([]any) {
		if m := item.(map[string]any); m["id"] == f.fid {
			return m
		}
	}
	f.c.t.Fatal("field gone")
	return nil
}
func (f *tagFixture) usage(tid string) map[string]any {
	f.c.t.Helper()
	return f.c.do("GET", "/api/v1/fields/"+f.fid+"/tags/"+tid+"/usage", nil, 200)
}
func (f *tagFixture) remove(tid string) {
	f.c.t.Helper()
	f.c.do("DELETE", "/api/v1/fields/"+f.fid+"/tags/"+tid, map[string]any{"version": 1, "confirm": true}, 200)
}
func (f *tagFixture) filter(operator string, ids ...string) map[string]any {
	return map[string]any{"fieldId": f.fid, "operator": operator, "value": ids}
}

func TestTagUsageCountsLiquidsAndTrash(t *testing.T) {
	f, c := newTagFixture(t)
	tid, unused := f.tag("Minze"), f.tag("Frei")
	f.liquid("Eins", tid)
	f.liquid("Zwei", tid)
	gone := f.liquid("Drei", tid)
	c.do("DELETE", "/api/v1/liquids/"+gone["id"].(string), map[string]any{"version": 1}, 200)
	got := f.usage(tid)
	if got["liquids"] != float64(3) || got["trashed"] != float64(1) {
		t.Fatalf("usage %v", got)
	}
	if got["isDefault"] != false || got["widgets"] != float64(0) || got["views"] != float64(0) {
		t.Fatalf("usage %v", got)
	}
	if got = f.usage(unused); got["liquids"] != float64(0) || got["trashed"] != float64(0) {
		t.Fatalf("unused tag %v", got)
	}
}
func TestDeleteTagRemovesItsAssignmentsOnly(t *testing.T) {
	f, c := newTagFixture(t)
	tid, keep := f.tag("Minze"), f.tag("Bleibt")
	both := f.liquid("Beide", tid, keep)
	other := f.liquid("Nur anderer", keep)
	gone := f.liquid("Papierkorb", tid)
	c.do("DELETE", "/api/v1/liquids/"+gone["id"].(string), map[string]any{"version": 1}, 200)
	f.remove(tid)

	tags := c.do("GET", "/api/v1/fields/"+f.fid+"/tags", nil, 200)["items"].([]any)
	if len(tags) != 1 || tags[0].(map[string]any)["id"] != keep {
		t.Fatalf("tags %v", tags)
	}
	after := c.do("GET", "/api/v1/liquids/"+both["id"].(string), nil, 200)
	values := after["values"].(map[string]any)[f.fid].(map[string]any)["value"].([]any)
	if len(values) != 1 || values[0] != keep {
		t.Fatalf("values %v", values)
	}
	if after["version"] != float64(2) {
		t.Fatalf("an open form must notice: version %v", after["version"])
	}
	if after["updatedAt"] != both["updatedAt"] {
		t.Fatal("nobody edited the liquid, so its change date stays")
	}
	if c.do("GET", "/api/v1/liquids/"+other["id"].(string), nil, 200)["version"] != float64(1) {
		t.Fatal("an unaffected liquid changed version")
	}
	// A stale form now meets the usual conflict instead of an unknown tag.
	c.do("PATCH", "/api/v1/liquids/"+both["id"].(string), map[string]any{"version": 1, "name": "Alt"}, 409)
	restored := c.do("POST", "/api/v1/liquids/"+gone["id"].(string)+"/restore", map[string]any{"version": 3}, 200)
	if _, ok := restored["values"].(map[string]any)[f.fid]; ok {
		t.Fatal("restored liquid got the deleted tag back")
	}
}
func TestDeleteTagRequiresConfirmAndVersion(t *testing.T) {
	f, c := newTagFixture(t)
	tid := f.tag("Minze")
	l := f.liquid("Eins", tid)
	path := "/api/v1/fields/" + f.fid + "/tags/" + tid
	c.do("DELETE", path, map[string]any{"version": 1}, 422)
	c.do("DELETE", path, map[string]any{"version": 9, "confirm": true}, 409)
	c.do("PATCH", path, map[string]any{"name": "Neu", "paletteId": "blue", "version": 1}, 200)
	c.do("DELETE", path, map[string]any{"version": 1, "confirm": true}, 409)
	after := c.do("GET", "/api/v1/liquids/"+l["id"].(string), nil, 200)
	if after["version"] != float64(1) || len(after["values"].(map[string]any)[f.fid].(map[string]any)["value"].([]any)) != 1 {
		t.Fatalf("a refused delete changed the liquid: %v", after)
	}
	c.do("DELETE", path, map[string]any{"version": 2, "confirm": true}, 200)
	c.do("DELETE", path, map[string]any{"version": 2, "confirm": true}, 404)
	c.do("GET", path+"/usage", nil, 404)
}
func TestDeleteTagRejectsOtherFieldAndArchivedField(t *testing.T) {
	f, c := newTagFixture(t)
	tid := f.tag("Minze")
	second := c.do("POST", "/api/v1/fields", map[string]any{"label": "Zweite", "type": "tags", "visibility": map[string]bool{"form": true}}, 201)["id"].(string)
	c.do("DELETE", "/api/v1/fields/"+second+"/tags/"+tid, map[string]any{"version": 1, "confirm": true}, 404)
	text := c.do("POST", "/api/v1/fields", map[string]any{"label": "Text", "type": "text", "visibility": map[string]bool{"form": true}}, 201)["id"].(string)
	c.do("GET", "/api/v1/fields/"+text+"/tags/"+tid+"/usage", nil, 422)
	version := f.field()["version"]
	c.do("POST", "/api/v1/fields/"+f.fid+"/archive", map[string]any{"version": version, "archived": true}, 200)
	c.do("DELETE", "/api/v1/fields/"+f.fid+"/tags/"+tid, map[string]any{"version": 1, "confirm": true}, 422)
}
func TestTagDeleteAndUsageStayWithinTheCollection(t *testing.T) {
	f, c := newTagFixture(t)
	tid := f.tag("Minze")
	path := "/api/v1/fields/" + f.fid + "/tags/" + tid
	second, _ := secondAccount(t, c, f.s, "zweite")
	second.do("GET", path+"/usage", nil, 404)
	second.do("DELETE", path, map[string]any{"version": 1, "confirm": true}, 404)
	// Visitors may read tags, but neither their use nor their removal.
	for _, method := range []string{"GET", "DELETE"} {
		target := path
		if method == "GET" {
			target += "/usage"
		}
		r := httptest.NewRequest(method, "http://localhost"+target, nil)
		r.Header.Set("Origin", "http://localhost")
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s as visitor: %d", method, w.Code)
		}
	}
	f.remove(tid)
}
func TestDeleteTagClearsDefaultValue(t *testing.T) {
	f, c := newTagFixture(t)
	first, second := f.tag("Erster"), f.tag("Zweiter")
	c.do("PATCH", "/api/v1/fields/"+f.fid, map[string]any{"version": f.field()["version"], "config": map[string]any{"filterable": true, "defaultValue": []string{first, second}}}, 200)
	if f.usage(first)["isDefault"] != true {
		t.Fatal("default not reported")
	}
	version := f.field()["version"].(float64)
	schema := c.do("GET", "/api/v1/fields", nil, 200)["version"]
	f.remove(first)

	d := f.field()
	if d["version"] != version+1 {
		t.Fatalf("field version %v", d["version"])
	}
	if c.do("GET", "/api/v1/fields", nil, 200)["version"] == schema {
		t.Fatal("schema version unchanged")
	}
	values := d["config"].(map[string]any)["defaultValue"].([]any)
	if len(values) != 1 || values[0] != second {
		t.Fatalf("default %v", values)
	}
	// Both failed with the stale id: creating a liquid and saving the field.
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Neu"}, 201)
	if got := l["values"].(map[string]any)[f.fid].(map[string]any)["value"].([]any); len(got) != 1 || got[0] != second {
		t.Fatalf("new liquid %v", got)
	}
	c.do("PATCH", "/api/v1/fields/"+f.fid, map[string]any{"version": d["version"], "label": "Aromen", "config": d["config"]}, 200)

	f.remove(second)
	d = f.field()
	if _, ok := d["config"].(map[string]any)["defaultValue"]; ok {
		t.Fatalf("empty default stayed: %v", d["config"])
	}
	l = c.do("POST", "/api/v1/liquids", map[string]any{"name": "Ohne"}, 201)
	if _, ok := l["values"].(map[string]any)[f.fid]; ok {
		t.Fatal("new liquid got a deleted default")
	}
}
func TestDeleteTagCleansWidgetFilters(t *testing.T) {
	f, c := newTagFixture(t)
	tid, keep := f.tag("Minze"), f.tag("Bleibt")
	f.liquid("Eins", keep)
	f.liquid("Zwei", tid)
	board := c.do("GET", "/api/v1/dashboard", nil, 200)
	widget := func(id string, filters any) map[string]any {
		return map[string]any{"id": id, "type": "count", "title": id, "size": "small", "filters": filters}
	}
	widgets := []any{
		widget("only", f.filter("any", tid)),
		widget("mixed", f.filter("any", tid, keep)),
		widget("nested", map[string]any{"op": "and", "conditions": []any{map[string]any{"op": "or", "conditions": []any{f.filter("any", tid), f.filter("any", keep)}}}}),
		widget("other", f.filter("any", keep)),
	}
	c.do("PUT", "/api/v1/dashboard", map[string]any{"version": board["version"], "widgets": widgets}, 200)
	version := c.do("GET", "/api/v1/dashboard", nil, 200)["version"].(float64)
	if got := f.usage(tid)["widgets"]; got != float64(3) {
		t.Fatalf("widgets %v", got)
	}
	f.remove(tid)

	after := c.do("GET", "/api/v1/dashboard", nil, 200)
	if after["version"] != version+1 {
		t.Fatalf("dashboard version %v", after["version"])
	}
	saved := after["widgets"].([]any)
	if len(saved) != 4 {
		t.Fatalf("widgets %v", saved)
	}
	if _, ok := saved[0].(map[string]any)["filters"]; ok {
		t.Fatalf("emptied filter stayed: %v", saved[0])
	}
	// Every widget counts again: all liquids, then the one holding the kept tag.
	results := c.do("POST", "/api/v1/dashboard/query", map[string]any{"widgets": saved}, 200)["items"].([]any)
	want := []float64{2, 1, 1, 1}
	for i, r := range results {
		if got := r.(map[string]any)["count"]; got != want[i] {
			t.Fatalf("widget %d: %v, want %v (%v)", i, got, want[i], r)
		}
	}
}
func TestDeleteTagCleansSavedViews(t *testing.T) {
	f, c := newTagFixture(t)
	tid, keep := f.tag("Minze"), f.tag("Bleibt")
	f.liquid("Eins", keep)
	view := func(name string, filters any) map[string]any {
		return c.do("POST", "/api/v1/saved-views", map[string]any{"name": name, "search": "", "sort": "name", "direction": "asc", "filters": filters}, 201)
	}
	only := view("Nur Minze", f.filter("any", tid))
	mixed := view("Gemischt", f.filter("all", tid, keep))
	other := view("Anderer", f.filter("any", keep))
	if got := f.usage(tid)["views"]; got != float64(2) {
		t.Fatalf("views %v", got)
	}
	f.remove(tid)

	byID := map[string]map[string]any{}
	for _, item := range c.do("GET", "/api/v1/saved-views", nil, 200)["items"].([]any) {
		m := item.(map[string]any)
		byID[m["id"].(string)] = m
	}
	if got := byID[only["id"].(string)]; got["version"] != float64(2) || got["filters"] != nil {
		t.Fatalf("only %v", got)
	}
	got := byID[mixed["id"].(string)]
	if got["version"] != float64(2) {
		t.Fatalf("mixed %v", got)
	}
	if ids := got["filters"].(map[string]any)["value"].([]any); len(ids) != 1 || ids[0] != keep {
		t.Fatalf("mixed %v", ids)
	}
	if byID[other["id"].(string)]["version"] != float64(1) {
		t.Fatal("an unaffected view changed version")
	}
	// A cleaned view can still be saved under its new version.
	got["name"] = "Umbenannt"
	c.do("PATCH", "/api/v1/saved-views/"+got["id"].(string), got, 200)
}
func (f *tagFixture) require() {
	f.c.t.Helper()
	f.c.do("PATCH", "/api/v1/fields/"+f.fid, map[string]any{"version": f.field()["version"], "required": true}, 200)
}
func (f *tagFixture) exportPreview() error {
	f.c.t.Helper()
	ctx := context.Background()
	cid := f.c.do("GET", "/api/v1/me", nil, 200)["collectionId"].(string)
	var archive bytes.Buffer
	if e := backup.Export(ctx, f.s, cid, &archive, "test"); e != nil {
		f.c.t.Fatal(e)
	}
	path := filepath.Join(f.c.t.TempDir(), "export.zip")
	if e := os.WriteFile(path, archive.Bytes(), 0600); e != nil {
		f.c.t.Fatal(e)
	}
	_, e := backup.Preview(path, backup.WebLimits())
	return e
}

// A liquid that must hold a tag would otherwise block its own save and favourite, and make every
// later collection export impossible to import.
func TestDeletingTheLastRequiredTagKeepsLiquidsAndExportsUsable(t *testing.T) {
	f, c := newTagFixture(t)
	tid, next := f.tag("Minze"), f.tag("Neu")
	f.require()
	l := f.liquid("Eins", tid)
	gone := f.liquid("Papierkorb", tid)
	c.do("DELETE", "/api/v1/liquids/"+gone["id"].(string), map[string]any{"version": 1}, 200)
	if got := f.usage(tid)["orphaned"]; got != float64(2) {
		t.Fatalf("orphaned %v", got)
	}
	before := f.field()
	f.remove(tid)

	after := f.field()
	if after["required"] != true {
		t.Fatal("field no longer required")
	}
	if after["requiredFrom"] == before["requiredFrom"] || after["version"] != before["version"].(float64)+1 {
		t.Fatalf("rule not moved: %v", after)
	}
	if e := f.exportPreview(); e != nil {
		t.Fatalf("export cannot be imported: %v", e)
	}
	id := l["id"].(string)
	updated := c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 2, "favorite": true}, 200)
	if updated["favorite"] != true {
		t.Fatal("favourite not saved")
	}
	c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 3, "name": "Geändert"}, 200)
	// The rule stays for what is created from now on.
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "Ohne Tag"}, 422)
	f.liquid("Mit Tag", next)
	if e := f.exportPreview(); e != nil {
		t.Fatalf("export cannot be imported: %v", e)
	}
}
func TestDeletingARequiredTagWithOthersLeftKeepsTheRule(t *testing.T) {
	f, _ := newTagFixture(t)
	tid, keep := f.tag("Minze"), f.tag("Bleibt")
	f.require()
	f.liquid("Beide", tid, keep)
	if got := f.usage(tid)["orphaned"]; got != float64(0) {
		t.Fatalf("orphaned %v", got)
	}
	before := f.field()
	f.remove(tid)
	after := f.field()
	if after["requiredFrom"] != before["requiredFrom"] || after["version"] != before["version"] {
		t.Fatalf("rule moved without need: %v", after)
	}
}
func TestDeletingATagOfAnOptionalOrOlderLiquidKeepsTheRule(t *testing.T) {
	f, _ := newTagFixture(t)
	tid, other := f.tag("Minze"), f.tag("Alt")
	f.liquid("Vor der Pflicht", tid)
	f.liquid("Auch davor", other)
	f.require()
	if got := f.usage(tid)["orphaned"]; got != float64(0) {
		t.Fatalf("exempt liquid counted: %v", got)
	}
	before := f.field()
	f.remove(tid)
	if after := f.field(); after["requiredFrom"] != before["requiredFrom"] {
		t.Fatalf("rule moved without need: %v", after)
	}
	optional, _ := newTagFixture(t)
	free := optional.tag("Frei")
	optional.liquid("Eins", free)
	if got := optional.usage(free)["orphaned"]; got != float64(0) {
		t.Fatalf("optional field counted: %v", got)
	}
	optional.remove(free)
	if e := optional.exportPreview(); e != nil {
		t.Fatal(e)
	}
}
