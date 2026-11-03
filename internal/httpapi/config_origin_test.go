package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadedCanonicalHTTPSOriginSetsSecureSetupAndLoginCookies(t *testing.T) {
	t.Setenv("PUFFOLIO_DATA_DIR", t.TempDir())
	t.Setenv("PUFFOLIO_BASE_URL", "https://puffolio.example")
	t.Setenv("PUFFOLIO_PUBLIC_URL", "http://legacy.invalid")
	t.Setenv("PUFFOLIO_COOKIE_SECURE", "true")
	t.Setenv("PUFFOLIO_SECURE_COOKIE", "false")
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	s, e := store.Open(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := New(Dependencies{Store: s, Config: cfg})
	code, e := os.ReadFile(filepath.Join(s.Dir, "setup-code"))
	if e != nil {
		t.Fatal(e)
	}
	post := func(path, origin string, input any, status int) *httptest.ResponseRecorder {
		var body bytes.Buffer
		json.NewEncoder(&body).Encode(input)
		r := httptest.NewRequest("POST", "http://internal:8080/api/v1"+path, &body)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s via external origin: got%d expected%d", path, w.Code, status)
		}
		return w
	}
	setup := post("/setup", "https://puffolio.example", map[string]any{"code": strings.TrimSpace(string(code)), "username": "secure-user", "displayName": "Secure", "password": "Secure-long-test-password", "starterFields": false}, 201)
	checkCookie := func(w *httptest.ResponseRecorder) {
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatal("HTTPS authentication response lacks secure cookie flags")
		}
	}
	checkCookie(setup)
	credentials := map[string]any{"username": "secure-user", "password": "Secure-long-test-password"}
	post("/auth/login", "http://legacy.invalid", credentials, 403)
	checkCookie(post("/auth/login", "https://puffolio.example", credentials, 200))
}
