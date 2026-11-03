package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/backup"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func uploadReviewArchive(t *testing.T, c *client, archive []byte, status int) map[string]any {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, e := form.CreateFormFile("file", "collection.zip")
	if e != nil {
		t.Fatal(e)
	}
	part.Write(archive)
	form.Close()
	r := httptest.NewRequest("POST", "http://localhost/api/v1/imports/preview", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.AddCookie(c.cookie)
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("preview status%d expected%d body=%s", w.Code, status, w.Body.String())
	}
	var out map[string]any
	if e = json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestImportPreviewChecksDestinationAndCommitRechecksChangedRules(t *testing.T) {
	source, s := setupClient(t)
	source.do("POST", "/api/v1/liquids", map[string]any{"name": "Incoming"}, 201)
	me := source.do("GET", "/api/v1/me", nil, 200)
	var archive bytes.Buffer
	if e := backup.Export(context.Background(), s, me["collectionId"].(string), &archive, "test"); e != nil {
		t.Fatal(e)
	}
	inv := source.do("POST", "/api/v1/admin/users", map[string]any{"username": "target-user", "displayName": "Target", "role": "user"}, 201)
	target := &client{h: source.h, t: t}
	tm := target.do("POST", "/api/v1/auth/invite", map[string]any{"token": inv["invitationToken"], "password": "Target-long-test-password"}, 200)
	cid := tm["collectionId"].(string)
	preview := uploadReviewArchive(t, target, archive.Bytes(), 200)
	target.do("POST", "/api/v1/fields", map[string]any{"label": "All records required", "type": "boolean", "required": true, "visibility": map[string]bool{"form": true}, "backfillValue": false}, 201)
	out := target.do("POST", "/api/v1/imports", map[string]any{"token": preview["token"], "confirm": true}, 422)
	if out["error"].(map[string]any)["code"] != "validation_failed" {
		t.Fatal("unstable error wrapper")
	}
	uploadReviewArchive(t, target, archive.Bytes(), 422)
	var fields, liquids, version int
	s.DB.QueryRow("SELECT count(*) FROM field_definitions WHERE collection_id=?", cid).Scan(&fields)
	s.DB.QueryRow("SELECT count(*) FROM liquids WHERE collection_id=?", cid).Scan(&liquids)
	s.DB.QueryRow("SELECT version FROM user_preferences WHERE user_id=?", tm["user"].(map[string]any)["id"]).Scan(&version)
	if fields != 1 || liquids != 0 || version != 1 {
		t.Fatal("rejected import mutated destination", fields, liquids, version)
	}
}
