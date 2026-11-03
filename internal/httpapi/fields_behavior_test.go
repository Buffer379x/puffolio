package httpapi

import (
	"testing"
	"time"
)

func TestNarrowingArchiveDefaultsAndLegacyRequired(t *testing.T) {
	c, s := setupClient(t)
	num := c.do("POST", "/api/v1/fields", map[string]any{"label": "Exact", "type": "number", "visibility": map[string]bool{"form": true}, "config": map[string]any{"scale": 1, "min": "0", "max": "100", "defaultValue": "2.5"}}, 201)
	fid := num["id"].(string)
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Legacy"}, 201)
	id := l["id"].(string)
	if l["values"].(map[string]any)[fid].(map[string]any)["value"] != "2.5" {
		t.Fatal("missing default")
	}
	c.do("PATCH", "/api/v1/fields/"+fid, map[string]any{"version": 1, "config": map[string]any{"scale": 2}}, 422)
	c.do("PATCH", "/api/v1/fields/"+fid, map[string]any{"version": 1, "config": map[string]any{"scale": 1, "max": "2"}}, 422)
	c.do("POST", "/api/v1/fields/"+fid+"/archive", map[string]any{"version": 1, "archived": true}, 200)
	updated := c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 1, "name": "Kept"}, 200)
	if updated["values"].(map[string]any)[fid].(map[string]any)["value"] != "2.5" {
		t.Fatal("archive lost value")
	}
	c.do("POST", "/api/v1/fields", map[string]any{"label": "New required", "type": "text", "required": true, "visibility": map[string]bool{"form": true}}, 201)
	c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 2, "name": "Legacy allowed"}, 200)
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "New requires value"}, 422)
	var rows int
	s.DB.QueryRow("SELECT count(*) FROM field_values WHERE field_id=?", fid).Scan(&rows)
	if rows != 1 {
		t.Fatal("values changed", rows)
	}
}
func TestTodayDefaultOnlyAtCreation(t *testing.T) {
	c, _ := setupClient(t)
	d := c.do("POST", "/api/v1/fields", map[string]any{"label": "Date", "type": "date", "visibility": map[string]bool{"form": true}, "config": map[string]any{"defaultToday": true}}, 201)
	fid := d["id"].(string)
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Today"}, 201)
	loc, _ := time.LoadLocation("Europe/Berlin")
	if l["values"].(map[string]any)[fid].(map[string]any)["value"] != time.Now().In(loc).Format("2006-01-02") {
		t.Fatal("date default timezone")
	}
	id := l["id"].(string)
	c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 1, "values": map[string]any{fid: nil}}, 200)
	l = c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 2, "name": "No refill"}, 200)
	if _, ok := l["values"].(map[string]any)[fid]; ok {
		t.Fatal("default reapplied on patch")
	}
}
func TestTagColorUpdateKeepsAssignments(t *testing.T) {
	c, _ := setupClient(t)
	fs := c.do("GET", "/api/v1/fields", nil, 200)["items"].([]any)
	var fid string
	for _, f := range fs {
		m := f.(map[string]any)
		if m["type"] == "tags" {
			fid = m["id"].(string)
		}
	}
	tag := c.do("POST", "/api/v1/fields/"+fid+"/tags", map[string]any{"name": "Äpfel", "paletteId": "teal"}, 201)
	tid := tag["id"].(string)
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Tagged", "values": map[string]any{fid: map[string]any{"type": "tags", "value": []string{tid}}}}, 201)
	c.do("POST", "/api/v1/fields/"+fid+"/tags", map[string]any{"name": "A\u0308PFEL", "paletteId": "blue"}, 409)
	c.do("PATCH", "/api/v1/fields/"+fid+"/tags/"+tid, map[string]any{"name": "Fresh", "paletteId": "violet", "version": 1}, 200)
	l = c.do("GET", "/api/v1/liquids/"+l["id"].(string), nil, 200)
	if l["values"].(map[string]any)[fid].(map[string]any)["value"].([]any)[0] != tid {
		t.Fatal("tag update broke relation")
	}
}
func TestUsedOptionCanOnlyBeArchived(t *testing.T) {
	c, _ := setupClient(t)
	d := c.do("POST", "/api/v1/fields", map[string]any{"label": "Status2", "type": "select", "visibility": map[string]bool{"form": true}, "config": map[string]any{"options": []any{map[string]any{"id": "temp", "label": "One"}}}}, 201)
	fid := d["id"].(string)
	option := d["config"].(map[string]any)["options"].([]any)[0].(map[string]any)
	oid := option["id"].(string)
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Selected", "values": map[string]any{fid: map[string]any{"type": "select", "value": oid}}}, 201)
	c.do("PATCH", "/api/v1/fields/"+fid, map[string]any{"version": 1, "config": map[string]any{"options": []any{}}}, 422)
	option["archived"] = true
	c.do("PATCH", "/api/v1/fields/"+fid, map[string]any{"version": 1, "config": map[string]any{"options": []any{option}}}, 200)
	c.do("PATCH", "/api/v1/liquids/"+l["id"].(string), map[string]any{"version": 1, "values": map[string]any{fid: map[string]any{"type": "select", "value": oid}}}, 200)
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "New assignment", "values": map[string]any{fid: map[string]any{"type": "select", "value": oid}}}, 422)
}
func TestDashboardRejectsDeepFilterLayout(t *testing.T) {
	c, _ := setupClient(t)
	layout := c.do("GET", "/api/v1/dashboard", nil, 200)
	filter := map[string]any{"fieldId": "missing", "operator": "empty"}
	for i := 0; i < 4; i++ {
		filter = map[string]any{"op": "and", "conditions": []any{filter}}
	}
	c.do("PUT", "/api/v1/dashboard", map[string]any{"version": layout["version"], "widgets": []any{map[string]any{"id": "01900000-0000-7000-8000-000000000001", "type": "count", "title": "Deep", "size": "small", "filters": filter}}}, 422)
}
func TestRatingAndDateNarrowingRejectExistingValues(t *testing.T) {
	c, _ := setupClient(t)
	rating := c.do("POST", "/api/v1/fields", map[string]any{"label": "Ten stars", "type": "rating", "config": map[string]any{"stars": 10}}, 201)
	date := c.do("POST", "/api/v1/fields", map[string]any{"label": "Purchase", "type": "date", "config": map[string]any{"min": "2026-01-01"}}, 201)
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "Existing", "values": map[string]any{rating["id"].(string): map[string]any{"type": "rating", "value": 9}, date["id"].(string): map[string]any{"type": "date", "value": "2026-02-01"}}}, 201)
	c.do("PATCH", "/api/v1/fields/"+rating["id"].(string), map[string]any{"version": 1, "config": map[string]any{"stars": 5}}, 422)
	c.do("PATCH", "/api/v1/fields/"+date["id"].(string), map[string]any{"version": 1, "config": map[string]any{"min": "2027-01-01"}}, 422)
}
func TestFieldDefaultsRejectUnknownMediaAndTags(t *testing.T) {
	c, _ := setupClient(t)
	id := "01900000-0000-7000-8000-000000000001"
	c.do("POST", "/api/v1/fields", map[string]any{"label": "Bad image default", "type": "image", "config": map[string]any{"defaultValue": id}}, 422)
	c.do("POST", "/api/v1/fields", map[string]any{"label": "Bad tags default", "type": "tags", "config": map[string]any{"defaultValue": []string{id}}}, 422)
}
