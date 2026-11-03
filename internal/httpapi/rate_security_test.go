package httpapi

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRateLimiterRetainsBoundedKeysForOversizedNames(t *testing.T) {
	s := &server{attempts: map[string]attempt{}}
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	name := "private-input-marker-" + strings.Repeat("x", 128<<10)
	if !s.rateAllowed(r, name) {
		t.Fatal("first attempt rejected")
	}
	for key := range s.attempts {
		if len(key) > 256 || strings.Contains(key, "private-input-marker") {
			t.Fatalf("retained unbounded/raw rate key; bytes=%d", len(key))
		}
	}
}
func TestRateLimiterNormalizesEquivalentAccountNames(t *testing.T) {
	s := &server{attempts: map[string]attempt{}}
	names := []string{"  A\u0308LICE  ", "älice"}
	for i := 0; i < 10; i++ {
		r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", nil)
		r.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", i+1)
		if !s.rateAllowed(r, names[i%2]) {
			t.Fatal("attempt rejected before account limit")
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", nil)
	r.RemoteAddr = "192.0.2.99:1234"
	if s.rateAllowed(r, "ÄLiCe") {
		t.Fatal("equivalent Unicode/space/case spelling split account limit")
	}
}
func TestInvalidLoginUsernamesReturnGenericFailure(t *testing.T) {
	c, _ := setupClient(t)
	for _, name := range []string{strings.Repeat("x", 65), "a", "bad user"} {
		out := c.do("POST", "/api/v1/auth/login", map[string]any{"username": name, "password": "wrong-password-value"}, 401)
		err := out["error"].(map[string]any)
		if err["code"] != "login_failed" || err["message"] != "Anmeldung fehlgeschlagen" || err["fields"] != nil {
			t.Fatal("username validation leaked a distinct response")
		}
	}
}
