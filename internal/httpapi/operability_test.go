package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/media"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A server reached by LAN IP or behind a proxy must work without configuration.
func TestOriginFollowsTheAddressInUseUnlessPinned(t *testing.T) {
	post := func(h http.Handler, host, origin string) int {
		r := httptest.NewRequest("POST", "http://"+host+"/api/v1/auth/login", strings.NewReader(`{"username":"x","password":"y"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	c, s := setupClient(t)
	for _, origin := range []string{"http://192.168.1.10:8787", "https://192.168.1.10:8787"} {
		if code := post(c.h, "192.168.1.10:8787", origin); code == 403 {
			t.Fatalf("same-host origin %s rejected", origin)
		}
	}
	for _, origin := range []string{"http://evil.example", "http://192.168.1.10:9999", "ftp://192.168.1.10:8787", "null"} {
		if code := post(c.h, "192.168.1.10:8787", origin); code != 403 {
			t.Fatalf("cross origin %s accepted with %d", origin, code)
		}
	}
	pinned := New(Dependencies{Store: s, Config: config.Config{DataDir: s.Dir, PublicURL: "https://puffolio.example/"}})
	if code := post(pinned, "192.168.1.10:8787", "http://192.168.1.10:8787"); code != 403 {
		t.Fatalf("explicit base URL not enforced: %d", code)
	}
	if code := post(pinned, "internal:8080", "https://puffolio.example"); code == 403 {
		t.Fatal("explicit base URL rejected")
	}
}

func TestVisibleFieldsAppearInViewsWithoutExtraConfiguration(t *testing.T) {
	c, _ := setupClient(t)
	prefs := func() (list, grid []any) {
		p := c.do("GET", "/api/v1/preferences", nil, 200)
		return p["listFields"].([]any), p["gridFields"].([]any)
	}
	has := func(ids []any, id string) bool {
		for _, v := range ids {
			if v == id {
				return true
			}
		}
		return false
	}
	field := c.do("POST", "/api/v1/fields", map[string]any{"label": "Neu", "type": "text", "visibility": map[string]bool{"form": true, "detail": true, "list": true, "grid": false}, "config": map[string]any{}}, 201)
	id := field["id"].(string)
	list, grid := prefs()
	if !has(list, id) || has(grid, id) {
		t.Fatalf("new field: list=%v grid=%v", list, grid)
	}
	c.do("PATCH", "/api/v1/fields/"+id, map[string]any{"version": field["version"], "visibility": map[string]bool{"form": true, "detail": true, "list": true, "grid": true}}, 200)
	if _, grid = prefs(); !has(grid, id) {
		t.Fatal("field made visible on cards is still hidden")
	}
}

func TestStarterAccountOpensOnAUsefulDashboard(t *testing.T) {
	c, _ := setupClient(t)
	layout := c.do("GET", "/api/v1/dashboard", nil, 200)
	kinds := []string{}
	for _, w := range layout["widgets"].([]any) {
		kinds = append(kinds, w.(map[string]any)["type"].(string))
	}
	if strings.Join(kinds, ",") != "count,favorites,average,expired,recent,donut,dates,expiring,timeline,shelfStatus" {
		t.Fatalf("default widgets: %v", kinds)
	}
	result := c.do("POST", "/api/v1/dashboard/query", map[string]any{"widgets": layout["widgets"]}, 200)
	for _, item := range result["items"].([]any) {
		w := item.(map[string]any)
		if _, failed := w["error"]; failed {
			t.Fatalf("default widget fails on a new account: %v", w)
		}
	}
}

func TestMediaRevalidatesInsteadOfRedownloading(t *testing.T) {
	c, s := setupClient(t)
	me := c.do("GET", "/api/v1/me", nil, 200)
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	m, e := media.New(s).Upload(context.Background(), me["collectionId"].(string), &b)
	if e != nil {
		t.Fatal(e)
	}
	get := func(since string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://localhost/api/v1/media/"+m.ID+"?variant=thumbnail", nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if since != "" {
			r.Header.Set("If-Modified-Since", since)
		}
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w
	}
	first := get("", c.cookie)
	if first.Code != 200 || first.Header().Get("Last-Modified") == "" {
		t.Fatalf("first load %d", first.Code)
	}
	if w := get(first.Header().Get("Last-Modified"), c.cookie); w.Code != 304 || w.Body.Len() != 0 {
		t.Fatalf("revalidation answered %d with %d bytes", w.Code, w.Body.Len())
	}
}

// slowBody delivers its first bytes and then waits, like an upload on poor WiFi.
type slowBody struct {
	head    io.Reader
	release <-chan struct{}
	rest    io.Reader
}

func (b *slowBody) Read(p []byte) (int, error) {
	if n, e := b.head.Read(p); e != io.EOF {
		return n, e
	}
	<-b.release
	return b.rest.Read(p)
}

func TestSlowUploadDoesNotBlockOtherWrites(t *testing.T) {
	c, _ := setupClient(t)
	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	part, _ := mw.CreateFormFile("file", "x.png")
	png.Encode(part, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	mw.Close()
	raw := form.Bytes()
	release := make(chan struct{})
	r := httptest.NewRequest("POST", "http://localhost/api/v1/media", &slowBody{head: bytes.NewReader(raw[:len(raw)/2]), release: release, rest: bytes.NewReader(raw[len(raw)/2:])})
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.AddCookie(c.cookie)
	upload := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { c.h.ServeHTTP(upload, r); close(done) }()
	time.Sleep(50 * time.Millisecond)
	saved := make(chan struct{})
	go func() {
		c.do("POST", "/api/v1/liquids", map[string]any{"name": "Während des Uploads"}, 201)
		close(saved)
	}()
	select {
	case <-saved:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("a slow upload blocked another write")
	}
	close(release)
	<-done
	if upload.Code != 201 {
		t.Fatalf("upload finished with %d: %s", upload.Code, upload.Body.String())
	}
	var m media.Media
	if e := json.Unmarshal(upload.Body.Bytes(), &m); e != nil || m.ID == "" {
		t.Fatal("upload response", e)
	}
}

func TestFinishedDownloadDoesNotHoldWriter(t *testing.T) {
	c, s := setupClient(t)
	blocked := make(chan bool, 1)
	r := httptest.NewRequest("POST", "http://localhost/api/v1/exports", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("X-CSRF-Token", c.csrf)
	r.AddCookie(c.cookie)
	w := &probeWriter{ResponseRecorder: httptest.NewRecorder(), onWrite: func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if e := s.Writer.LockContext(ctx); e != nil {
			blocked <- true
			return
		}
		s.Writer.Unlock()
		blocked <- false
	}}
	c.h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("export %d", w.Code)
	}
	if <-blocked {
		t.Fatal("writer stayed locked while streaming a finished archive")
	}
	entries, _ := os.ReadDir(filepath.Join(s.Dir))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".download-") {
			t.Fatal("download temp file left behind")
		}
	}
}

type probeWriter struct {
	*httptest.ResponseRecorder
	onWrite func()
	called  bool
}

func (p *probeWriter) Write(b []byte) (int, error) {
	if !p.called {
		p.called = true
		p.onWrite()
	}
	return p.ResponseRecorder.Write(b)
}

func TestVisitorsReadTheOwnersCollectionAndCannotChangeIt(t *testing.T) {
	c, s := setupClient(t)
	kept := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Sichtbar"}, 201)
	gone := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Weg"}, 201)
	c.do("DELETE", "/api/v1/liquids/"+gone["id"].(string), map[string]any{"version": gone["version"]}, 200)
	me := c.do("GET", "/api/v1/me", nil, 200)
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	m, e := media.New(s).Upload(context.Background(), me["collectionId"].(string), &b)
	if e != nil {
		t.Fatal(e)
	}
	visitor := &client{h: c.h, t: t}
	page := visitor.do("GET", "/api/v1/liquids?sort=name&direction=asc", nil, 200)
	if items := page["items"].([]any); len(items) != 1 || items[0].(map[string]any)["name"] != "Sichtbar" {
		t.Fatalf("visitor list: %v", items)
	}
	visitor.do("GET", "/api/v1/fields", nil, 200)
	visitor.do("GET", "/api/v1/preferences", nil, 200)
	layout := visitor.do("GET", "/api/v1/dashboard", nil, 200)
	visitor.do("POST", "/api/v1/dashboard/query", map[string]any{"widgets": layout["widgets"]}, 200)
	visitor.do("GET", "/api/v1/media/"+m.ID+"?variant=thumbnail", nil, 200)
	id := kept["id"].(string)
	writes := []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/v1/liquids", map[string]any{"name": "x"}},
		{"PATCH", "/api/v1/liquids/" + id, map[string]any{"version": kept["version"], "favorite": true}},
		{"DELETE", "/api/v1/liquids/" + id, map[string]any{"version": kept["version"]}},
		{"POST", "/api/v1/liquids/bulk", map[string]any{"ids": []string{id}, "action": "trash"}},
		{"POST", "/api/v1/fields", map[string]any{"label": "x", "type": "text"}},
		{"PUT", "/api/v1/preferences", map[string]any{}},
		{"PUT", "/api/v1/dashboard", map[string]any{}},
		{"POST", "/api/v1/media", nil},
		{"POST", "/api/v1/exports", map[string]any{}},
		{"POST", "/api/v1/imports", map[string]any{}},
		{"POST", "/api/v1/admin/backups", map[string]any{}},
	}
	for _, write := range writes {
		visitor.do(write.method, write.path, write.body, 401)
	}
	for _, private := range []string{"/api/v1/me", "/api/v1/liquids?trash=true", "/api/v1/media/" + m.ID + "?variant=master", "/api/v1/media/" + m.ID + "/info", "/api/v1/saved-views", "/api/v1/admin/system", "/api/v1/fields/x/usage"} {
		visitor.do("GET", private, nil, 401)
	}
	visitor.do("GET", "/api/v1/liquids/"+gone["id"].(string), nil, 404)
	// A signed-in owner still needs the CSRF token, also on the public POST.
	c.csrf = "wrong"
	c.do("POST", "/api/v1/dashboard/query", map[string]any{"widgets": []any{}}, 403)
}

func TestOnlyAdministratorsCanSignIn(t *testing.T) {
	c, s := setupClient(t)
	if _, e := auth.New(s).CreateUser(context.Background(), "legacy", "Legacy", "user", "Legacy-long-test-password"); e != nil {
		t.Fatal(e)
	}
	anon := &client{h: c.h, t: t}
	anon.do("POST", "/api/v1/auth/login", map[string]any{"username": "legacy", "password": "Legacy-long-test-password"}, 401)
	anon.do("POST", "/api/v1/auth/login", map[string]any{"username": "alice", "password": "A-long-test-password"}, 200)
}

func TestShelfLifeIsValidatedAndDefaultsToFiveYears(t *testing.T) {
	c, _ := setupClient(t)
	p := c.do("GET", "/api/v1/preferences", nil, 200)
	if p["shelfLifeMonths"] != float64(60) || p["roles"].(map[string]any)["purchase"] == nil {
		t.Fatalf("starter shelf life: %v", p)
	}
	roles := p["roles"].(map[string]any)
	if roles["shelfOverride"] == nil || roles["status"] == nil || p["consumedOption"] == nil {
		t.Fatalf("starter rules: %v", p)
	}
	consumed := p["consumedOption"]
	p["consumedOption"] = "someone-elses-option"
	c.do("PUT", "/api/v1/preferences", p, 422)
	p["consumedOption"] = consumed
	p["shelfLifeMonths"] = 1201
	c.do("PUT", "/api/v1/preferences", p, 422)
	p["shelfLifeMonths"] = 24
	if saved := c.do("PUT", "/api/v1/preferences", p, 200); saved["shelfLifeMonths"] != float64(24) {
		t.Fatalf("saved: %v", saved)
	}
}

func TestShelfLifeReachesEveryReaderAndHonoursOverridesAndConsumed(t *testing.T) {
	c, _ := setupClient(t)
	p := c.do("GET", "/api/v1/preferences", nil, 200)
	roles := p["roles"].(map[string]any)
	purchase, override, status := roles["purchase"].(string), roles["shelfOverride"].(string), roles["status"].(string)
	consumed := p["consumedOption"].(string)
	day := func(years int) string { return time.Now().AddDate(-years, 0, 0).Format("2006-01-02") }
	date := func(years int) map[string]any { return map[string]any{"type": "date", "value": day(years)} }
	add := func(name string, values map[string]any) map[string]any {
		return c.do("POST", "/api/v1/liquids", map[string]any{"name": name, "values": values}, 201)
	}
	standard := add("Standard", map[string]any{purchase: date(1)})
	if life := standard["shelfLife"].(map[string]any); life["percent"] != float64(80) || life["expired"] != false {
		t.Fatalf("saved response: %v", life)
	}
	own := add("Eigene", map[string]any{purchase: date(1), override: map[string]any{"type": "number", "value": "24"}})
	if life := own["shelfLife"].(map[string]any); life["percent"] != float64(50) {
		t.Fatalf("own storage time: %v", life)
	}
	used := add("Verbraucht", map[string]any{purchase: date(9), status: map[string]any{"type": "select", "value": consumed}})
	if used["shelfLife"] != nil {
		t.Fatalf("consumed liquid has a bar: %v", used["shelfLife"])
	}
	add("Abgelaufen", map[string]any{purchase: date(9)})
	visitor := &client{h: c.h, t: t}
	page := visitor.do("GET", "/api/v1/liquids?sort=shelf&direction=asc", nil, 200)
	order := []string{}
	for _, item := range page["items"].([]any) {
		order = append(order, item.(map[string]any)["name"].(string))
	}
	if got := strings.Join(order, ","); got != "Abgelaufen,Eigene,Standard,Verbraucht" {
		t.Fatalf("visitor order by remaining share: %s", got)
	}
	if first := page["items"].([]any)[1].(map[string]any); first["shelfLife"] == nil {
		t.Fatal("visitors must see the shelf life")
	}
	widgets := []map[string]any{{"id": "a", "type": "expired", "title": "A", "size": "small"}, {"id": "b", "type": "expiring", "title": "B", "size": "medium", "limit": 5}}
	result := visitor.do("POST", "/api/v1/dashboard/query", map[string]any{"widgets": widgets}, 200)["items"].([]any)
	if count := result[0].(map[string]any)["count"]; count != float64(1) {
		t.Fatalf("a consumed liquid must not count as expired: %v", count)
	}
	items := result[1].(map[string]any)["items"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["name"] != "Abgelaufen" || items[1].(map[string]any)["name"] != "Eigene" || items[0].(map[string]any)["shelfLife"] == nil {
		t.Fatalf("expiring list: %v", items)
	}
	c.do("POST", "/api/v1/saved-views", map[string]any{"name": "Nach Haltbarkeit", "sort": "shelf", "direction": "asc"}, 201)
}

func TestImportKeepsShelfSettingsPointingAtTheNewIDs(t *testing.T) {
	source, s := setupClient(t)
	before := source.do("GET", "/api/v1/preferences", nil, 200)
	me := source.do("GET", "/api/v1/me", nil, 200)
	var archive bytes.Buffer
	if e := backup.Export(context.Background(), s, me["collectionId"].(string), &archive, "test"); e != nil {
		t.Fatal(e)
	}
	target, _ := secondAccount(t, source, s, "shelf-target")
	preview := uploadReviewArchive(t, target, archive.Bytes(), 200)
	target.do("POST", "/api/v1/imports", map[string]any{"token": preview["token"], "confirm": true}, 200)
	after := target.do("GET", "/api/v1/preferences", nil, 200)
	roles, oldRoles := after["roles"].(map[string]any), before["roles"].(map[string]any)
	if after["shelfLifeMonths"] != float64(60) || roles["status"] == oldRoles["status"] || after["consumedOption"] == before["consumedOption"] {
		t.Fatalf("imported settings: %v", after)
	}
	found := false
	for _, f := range target.do("GET", "/api/v1/fields", nil, 200)["items"].([]any) {
		field := f.(map[string]any)
		if field["id"] == roles["status"] {
			for _, o := range field["config"].(map[string]any)["options"].([]any) {
				option := o.(map[string]any)
				found = found || option["id"] == after["consumedOption"] && option["label"] == "Aufgebraucht"
			}
		}
	}
	if !found {
		t.Fatalf("consumed option does not exist after import: %v", after)
	}
	target.do("PUT", "/api/v1/preferences", after, 200)
}
