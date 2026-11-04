package httpapi

import (
	"bytes"
	"context"
	"github.com/Buffer379x/puffolio/internal/media"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrivateMediaIsNotReusedAcrossAccountAuthorization(t *testing.T) {
	c, s := setupClient(t)
	me := c.do("GET", "/api/v1/me", nil, 200)
	var pngBytes bytes.Buffer
	png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	m, e := media.New(s).Upload(context.Background(), me["collectionId"].(string), &pngBytes)
	if e != nil {
		t.Fatal(e)
	}
	get := func(cookie *http.Cookie, variant string, conditional bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://localhost/api/v1/media/"+m.ID+variant, nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if conditional {
			r.Header.Set("If-Modified-Since", "Thu, 01 Jan 2099 00:00:00 GMT")
		}
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w
	}
	for _, variant := range []string{"", "?variant=master", "?variant=thumbnail"} {
		w := get(c.cookie, variant, false)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		cc := w.Header().Get("Cache-Control")
		if !strings.Contains(cc, "private") || !strings.Contains(cc, "no-cache") || strings.Contains(cc, "immutable") {
			t.Fatalf("media authorizations may be reused from cache: %q", cc)
		}
		if !strings.Contains(strings.ToLower(strings.Join(w.Header().Values("Vary"), ",")), "cookie") {
			t.Fatal("media response does not vary by session cookie")
		}
	}
	other, _ := secondAccount(t, c, s, "other-user")
	for _, variant := range []string{"", "?variant=master", "?variant=thumbnail"} {
		// Visitors see the owner's published crop, never the uncropped master.
		wantAnonymous := 304
		if variant == "?variant=master" {
			wantAnonymous = 401
		}
		if w := get(nil, variant, true); w.Code != wantAnonymous {
			t.Fatalf("anonymous conditional media GET%s=%d", variant, w.Code)
		}
		if w := get(other.cookie, variant, true); w.Code != 404 {
			t.Fatalf("foreign conditional media GET=%d", w.Code)
		}
	}
}
