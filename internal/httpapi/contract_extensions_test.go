package httpapi

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestSavedFavoriteViewAndSystemFilter(t *testing.T) {
	c, _ := setupClient(t)
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "Fav", "favorite": true}, 201)
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "Other"}, 201)
	v := c.do("POST", "/api/v1/saved-views", map[string]any{"name": "Favorites", "favorite": true, "search": "", "sort": "name", "direction": "asc"}, 201)
	if v["favorite"] != true {
		t.Fatal("favorite missing")
	}
	items := c.do("GET", "/api/v1/saved-views", nil, 200)["items"].([]any)
	if items[0].(map[string]any)["favorite"] != true {
		t.Fatal("favorite not persisted")
	}
	filter, _ := json.Marshal(map[string]any{"fieldId": "favorite", "operator": "eq", "value": false})
	p := c.do("GET", "/api/v1/liquids?filters="+url.QueryEscape(string(filter)), nil, 200)
	if p["total"] != float64(1) || p["items"].([]any)[0].(map[string]any)["name"] != "Other" {
		t.Fatal("system favorite filter")
	}
}
func TestFieldUsageIncludesTags(t *testing.T) {
	c, _ := setupClient(t)
	fs := c.do("GET", "/api/v1/fields", nil, 200)["items"].([]any)
	var fid string
	for _, f := range fs {
		m := f.(map[string]any)
		if m["type"] == "tags" {
			fid = m["id"].(string)
		}
	}
	a := c.do("POST", "/api/v1/fields/"+fid+"/tags", map[string]any{"name": "A", "paletteId": "teal"}, 201)
	b := c.do("POST", "/api/v1/fields/"+fid+"/tags", map[string]any{"name": "B", "paletteId": "blue"}, 201)
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "Two tags one liquid", "values": map[string]any{fid: map[string]any{"type": "tags", "value": []any{a["id"], b["id"]}}}}, 201)
	usage := c.do("GET", "/api/v1/fields/"+fid+"/usage", nil, 200)
	if usage["values"] != float64(1) {
		t.Fatal("must count liquids once", usage)
	}
	c.do("GET", "/api/v1/fields/01900000-0000-7000-8000-000000000001/usage", nil, 404)
}
func TestPermanentTrashDeletionRequiresConfirmationAndVersion(t *testing.T) {
	c, s := setupClient(t)
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Disposable"}, 201)
	id := l["id"].(string)
	c.do("DELETE", "/api/v1/liquids/"+id+"?permanent=true", map[string]any{"version": 1, "confirm": true}, 409)
	c.do("DELETE", "/api/v1/liquids/"+id, map[string]any{"version": 1}, 200)
	c.do("DELETE", "/api/v1/liquids/"+id+"?permanent=true", map[string]any{"version": 2, "confirm": false}, 422)
	c.do("DELETE", "/api/v1/liquids/"+id+"?permanent=true", map[string]any{"version": 1, "confirm": true}, 409)
	c.do("DELETE", "/api/v1/liquids/"+id+"?permanent=true", map[string]any{"version": 2, "confirm": true}, 200)
	c.do("GET", "/api/v1/liquids/"+id, nil, 404)
	var count int
	s.DB.QueryRow("SELECT count(*) FROM liquids WHERE id=?", id).Scan(&count)
	if count != 0 {
		t.Fatal("permanent delete did not delete")
	}
}
func TestRequiredFieldBackfillZeroFalseAndInvalidRollback(t *testing.T) {
	c, s := setupClient(t)
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Legacy"}, 201)
	id := l["id"].(string)
	n := c.do("POST", "/api/v1/fields", map[string]any{"label": "Required amount", "type": "number", "required": true, "visibility": map[string]bool{"form": true}, "config": map[string]any{"scale": 1, "min": "0", "max": "10"}, "backfillValue": "0.0"}, 201)
	fid := n["id"].(string)
	l = c.do("GET", "/api/v1/liquids/"+id, nil, 200)
	if l["version"] != float64(2) || l["values"].(map[string]any)[fid].(map[string]any)["value"] != "0.0" {
		t.Fatal("zero backfill or version missing")
	}
	b := c.do("POST", "/api/v1/fields", map[string]any{"label": "Required bool", "type": "boolean", "required": true, "visibility": map[string]bool{"form": true}, "backfillValue": false}, 201)
	l = c.do("GET", "/api/v1/liquids/"+id, nil, 200)
	if l["version"] != float64(3) || l["values"].(map[string]any)[b["id"].(string)].(map[string]any)["value"] != false {
		t.Fatal("false backfill lost")
	}
	var before int
	s.DB.QueryRow("SELECT count(*) FROM field_definitions").Scan(&before)
	c.do("POST", "/api/v1/fields", map[string]any{"label": "Invalid backfill", "type": "number", "required": true, "visibility": map[string]bool{"form": true}, "config": map[string]any{"min": "1"}, "backfillValue": "0"}, 422)
	var after int
	s.DB.QueryRow("SELECT count(*) FROM field_definitions").Scan(&after)
	if before != after {
		t.Fatal("invalid backfill partially created field")
	}
	l = c.do("GET", "/api/v1/liquids/"+id, nil, 200)
	if l["version"] != float64(3) {
		t.Fatal("invalid backfill bumped version")
	}
	c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 3, "values": map[string]any{fid: nil}}, 422)
}
