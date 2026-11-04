package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type client struct {
	h      http.Handler
	cookie *http.Cookie
	csrf   string
	t      *testing.T
}

func (c *client) do(method, path string, data any, status int) map[string]any {
	c.t.Helper()
	var b bytes.Buffer
	if data != nil {
		json.NewEncoder(&b).Encode(data)
	}
	r := httptest.NewRequest(method, "http://localhost"+path, &b)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost")
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	if c.csrf != "" {
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != status {
		c.t.Fatalf("%s %s: status %d expected %d body=%s", method, path, w.Code, status, w.Body.String())
	}
	for _, cookie := range w.Result().Cookies() {
		c.cookie = cookie
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if s, ok := out["csrfToken"].(string); ok {
		c.csrf = s
	}
	return out
}
func setupClient(t *testing.T) (*client, *store.Store) {
	t.Helper()
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	h := New(Dependencies{Store: s, Config: config.Config{DataDir: s.Dir}})
	c := &client{h: h, t: t}
	code, _ := os.ReadFile(filepath.Join(s.Dir, "setup-code"))
	c.do("POST", "/api/v1/setup", map[string]any{"code": strings.TrimSpace(string(code)), "username": "alice", "displayName": "Alice", "password": "A-long-test-password", "starterFields": true}, 201)
	return c, s
}

// secondAccount signs in an account beside the owner, directly through the
// service, to prove the collection boundary still holds below the HTTP login.
func secondAccount(t *testing.T, c *client, s *store.Store, name string) (*client, string) {
	t.Helper()
	ctx := context.Background()
	u, e := auth.New(s).CreateUser(ctx, name, name, "user", "Second-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = auth.InitializeUserTx(ctx, tx, u, nil); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	token, e := auth.New(s).StartSession(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	return &client{h: c.h, t: t, csrf: auth.CSRF(token), cookie: &http.Cookie{Name: "puffolio_session", Value: token}}, u.CollectionID
}
func TestSetupCanCompleteOnlyOnce(t *testing.T) {
	c, _ := setupClient(t)
	c.do("POST", "/api/v1/setup", map[string]any{"code": "x"}, 409)
}
func TestCSRFRejectsCrossOrigin(t *testing.T) {
	c, _ := setupClient(t)
	c.csrf = "wrong"
	c.do("POST", "/api/v1/liquids", map[string]any{"name": "X"}, 403)
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"A-long-test-password"}`))
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestIsolationConflictsAndPreservedValues(t *testing.T) {
	c, st := setupClient(t)
	fs := c.do("GET", "/api/v1/fields", nil, 200)["items"].([]any)
	var field string
	for _, f := range fs {
		m := f.(map[string]any)
		if m["type"] == "text" {
			field = m["id"].(string)
		}
	}
	l := c.do("POST", "/api/v1/liquids", map[string]any{"name": "Öl 100%_", "values": map[string]any{field: map[string]any{"type": "text", "value": "Hidden"}}}, 201)
	id := l["id"].(string)
	l = c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"name": "Changed", "version": 1}, 200)
	if len(l["values"].(map[string]any)) != 1 {
		t.Fatal("patch lost hidden values")
	}
	c.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"name": "stale", "version": 1}, 409)
	b, _ := secondAccount(t, c, st, "bob")
	b.do("GET", "/api/v1/liquids/"+id, nil, 404)
	b.do("PATCH", "/api/v1/liquids/"+id, map[string]any{"version": 2, "name": "attack"}, 404)
	b.do("GET", "/api/v1/admin/system", nil, 403)
	b.do("POST", "/api/v1/liquids", map[string]any{"name": "bad", "values": map[string]any{field: map[string]any{"type": "text", "value": "attack"}}}, 422)
}
