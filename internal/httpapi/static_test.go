package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/Buffer379x/puffolio/internal/media"
)

func staticFixture() fstest.MapFS {
	return fstest.MapFS{
		"index.html":          {Data: []byte("<!doctype html><title>Static test</title>")},
		"theme.js":            {Data: []byte(strings.Repeat("document.documentElement.dataset.theme='dark';\n", 20))},
		"assets/app-hash.js":  {Data: []byte(strings.Repeat("export const message='public bundle';\n", 200))},
		"assets/app-hash.css": {Data: []byte(strings.Repeat(".test{color:rebeccapurple}\n", 100))},
		"brand/icon.svg":      {Data: []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")},
	}
}

func staticRequest(h http.Handler, method, path, encoding string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost"+path, nil)
	if encoding != "" {
		r.Header.Set("Accept-Encoding", encoding)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func gunzipStatic(t *testing.T, b []byte) []byte {
	t.Helper()
	z, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	plain, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

func assertStaticVary(t *testing.T, header http.Header, wantCookie bool) {
	t.Helper()
	var encodings, cookies int
	for _, line := range header.Values("Vary") {
		for _, token := range strings.Split(line, ",") {
			switch strings.ToLower(strings.TrimSpace(token)) {
			case "accept-encoding":
				encodings++
			case "cookie":
				cookies++
			}
		}
	}
	if encodings != 1 || (wantCookie && cookies != 1) {
		t.Fatalf("Vary lost or duplicated tokens: %v", header.Values("Vary"))
	}
}

func TestStaticGzipPreservesBytesMIMEAndCachePolicy(t *testing.T) {
	files := staticFixture()
	h := New(Dependencies{Web: files})
	for _, path := range []string{"/assets/app-hash.js", "/assets/app-hash.css", "/theme.js"} {
		t.Run(path, func(t *testing.T) {
			w := staticRequest(h, "GET", path, "gzip, deflate, br")
			if w.Code != 200 || w.Header().Get("Content-Encoding") != "gzip" {
				t.Fatalf("status=%d encoding=%q; public asset was not delivered as gzip", w.Code, w.Header().Get("Content-Encoding"))
			}
			if got := gunzipStatic(t, w.Body.Bytes()); !bytes.Equal(got, files[path[1:]].Data) {
				t.Fatal("gzip changed source bytes")
			}
			if w.Header().Get("Content-Length") != strconv.Itoa(w.Body.Len()) {
				t.Fatal("compressed length mismatch")
			}
			identity := staticRequest(h, "GET", path, "identity")
			if w.Header().Get("Content-Type") != identity.Header().Get("Content-Type") {
				t.Fatal("gzip changed original MIME")
			}
			mime := w.Header().Get("Content-Type")
			if strings.HasSuffix(path, ".css") {
				if !strings.HasPrefix(mime, "text/css") {
					t.Fatal(mime)
				}
			} else if !strings.HasPrefix(mime, "text/javascript") && !strings.HasPrefix(mime, "application/javascript") {
				t.Fatal(mime)
			}
			wantCache := "public, max-age=31536000, immutable"
			if path == "/theme.js" {
				wantCache = "no-cache"
			}
			if w.Header().Get("Cache-Control") != wantCache || identity.Header().Get("Cache-Control") != wantCache {
				t.Fatal("cache policy changed")
			}
			assertStaticVary(t, w.Header(), false)
			assertStaticVary(t, identity.Header(), false)
		})
	}
}

func TestStaticEncodingNegotiation(t *testing.T) {
	files := staticFixture()
	h := New(Dependencies{Web: files})
	for _, tc := range []struct {
		encoding string
		gzip     bool
		status   int
	}{
		{"gzip", true, 200}, {"GZip;Q=0.5", true, 200}, {"*;q=0.5", true, 200},
		{"gzip;q=0", false, 200}, {"GZIP;q=0.000", false, 200}, {"gzip;q=0,*;q=1", false, 200},
		{"gzip;q=0.5,identity;q=0.8", false, 200}, {"gzip;q=0.8,identity;q=0.5", true, 200},
		{"gzip;q=0.5,identity;q=0.5", true, 200}, {"*;q=0,gzip;q=1", true, 200},
		{"identity", false, 200}, {"br", false, 200}, {"", false, 200},
		{"gzip;q=oops", false, 200}, {"gzip;q=NaN", false, 200}, {"gzip;q=1.1", false, 200},
		{"gzip;q=0.0001", false, 200}, {"gzip;q=0,gzip;q=1", false, 200},
		{"*;q=0", false, 406}, {"gzip;q=0,identity;q=0", false, 406},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			w := staticRequest(h, "GET", "/assets/app-hash.js", tc.encoding)
			if w.Code != tc.status {
				t.Fatalf("status=%d want%d", w.Code, tc.status)
			}
			assertStaticVary(t, w.Header(), false)
			if tc.gzip {
				if w.Header().Get("Content-Encoding") != "gzip" {
					t.Fatal("accepted gzip was not selected")
				}
				if !bytes.Equal(gunzipStatic(t, w.Body.Bytes()), files["assets/app-hash.js"].Data) {
					t.Fatal("gzip bytes differ")
				}
			} else {
				if w.Header().Get("Content-Encoding") != "" {
					t.Fatal("gzip sent despite refusal")
				}
				if tc.status == 200 && !bytes.Equal(w.Body.Bytes(), files["assets/app-hash.js"].Data) {
					t.Fatal("identity bytes differ")
				}
			}
			if tc.status == 200 && w.Header().Get("Content-Length") != strconv.Itoa(w.Body.Len()) {
				t.Fatal("length mismatch")
			}
		})
	}
}

func TestStaticHeadVaryFallbackAndRanges(t *testing.T) {
	files := staticFixture()
	h := New(Dependencies{Web: files})
	for _, enc := range []string{"gzip", "gzip;q=0"} {
		get := staticRequest(h, "GET", "/assets/app-hash.js", enc)
		head := staticRequest(h, "HEAD", "/assets/app-hash.js", enc)
		if head.Code != 200 || head.Body.Len() != 0 {
			t.Fatal("HEAD body or status")
		}
		for _, key := range []string{"Content-Type", "Content-Length", "Content-Encoding", "Cache-Control", "Vary"} {
			if head.Header().Get(key) != get.Header().Get(key) {
				t.Fatalf("HEAD %s mismatch", key)
			}
		}
	}
	r := httptest.NewRequest("GET", "http://localhost/theme.js", nil)
	r.Header.Add("Accept-Encoding", "br")
	r.Header.Add("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	w.Header().Add("Vary", "Cookie")
	h.ServeHTTP(w, r)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("multiple header lines ignored")
	}
	assertStaticVary(t, w.Header(), true)
	r = httptest.NewRequest("GET", "http://localhost/theme.js", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	w.Header().Add("Vary", "Cookie, aCcEpT-EnCoDiNg")
	h.ServeHTTP(w, r)
	assertStaticVary(t, w.Header(), true)
	w = httptest.NewRecorder()
	w.Header().Set("Vary", "*")
	h.ServeHTTP(w, r)
	if got := w.Header().Values("Vary"); len(got) != 1 || got[0] != "*" {
		t.Fatal("Vary:* must remain intact", got)
	}
	for _, path := range []string{"/", "/settings/fields"} {
		w := staticRequest(h, "GET", path, "gzip")
		if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), files["index.html"].Data) || w.Header().Get("Cache-Control") != "no-cache" || w.Header().Get("Content-Encoding") != "" {
			t.Fatal("SPA/index behavior changed", path)
		}
	}
	w = staticRequest(h, "GET", "/assets/missing.js", "gzip")
	if w.Code != 404 || w.Header().Get("Content-Encoding") != "" {
		t.Fatal("missing asset not clean404")
	}
	encoded := staticRequest(h, "GET", "/assets/app-hash.js", "gzip").Body.Bytes()
	r = httptest.NewRequest("GET", "http://localhost/assets/app-hash.js", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	r.Header.Set("Range", "bytes=0-9")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 206 || w.Header().Get("Content-Encoding") != "gzip" || !bytes.Equal(w.Body.Bytes(), encoded[:10]) || w.Header().Get("Content-Length") != "10" || w.Header().Get("Content-Range") != fmt.Sprintf("bytes 0-9/%d", len(encoded)) {
		t.Fatal("encoded-byte range mismatch")
	}
	r.Header.Set("Range", "bytes=999999-")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 416 || w.Header().Get("Content-Encoding") != "" {
		t.Fatal("range error retains gzip representation")
	}
}

type countingStaticFS struct {
	fs.FS
	mu    sync.Mutex
	reads map[string]int
}

func (c *countingStaticFS) ReadFile(name string) ([]byte, error) {
	c.mu.Lock()
	c.reads[name]++
	c.mu.Unlock()
	return fs.ReadFile(c.FS, name)
}
func (c *countingStaticFS) count(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads[name]
}

func TestStaticConcurrentCachedRepresentations(t *testing.T) {
	files := staticFixture()
	counted := &countingStaticFS{FS: files, reads: map[string]int{}}
	h := New(Dependencies{Web: counted})
	if got := counted.count("assets/app-hash.js"); got != 1 {
		t.Fatalf("eligible asset must be read once at initialization, got%d", got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			enc := "gzip"
			if i%2 == 0 {
				enc = "identity"
			}
			w := staticRequest(h, "GET", "/assets/app-hash.js?request="+strconv.Itoa(i), enc)
			if w.Code != 200 {
				t.Errorf("status=%d", w.Code)
				return
			}
			b := w.Body.Bytes()
			if enc == "gzip" {
				if w.Header().Get("Content-Encoding") != "gzip" {
					t.Error("missing gzip")
					return
				}
				b = gunzipStatic(t, b)
			}
			if !bytes.Equal(b, files["assets/app-hash.js"].Data) {
				t.Error("concurrent representation corrupted")
			}
			if miss := staticRequest(h, "GET", fmt.Sprintf("/assets/missing-%d.js", i), "gzip"); miss.Code != 404 {
				t.Error("missing asset cached as success")
			}
		}(i)
	}
	wg.Wait()
	if got := counted.count("assets/app-hash.js"); got != 1 {
		t.Fatalf("cached asset reread%d times", got)
	}
}

func TestStaticGzipDoesNotCompressPrivateAPIAuthOrMedia(t *testing.T) {
	c, s := setupClient(t)
	// Exercise public and private routes on the same gzip-enabled handler.
	c.h = New(Dependencies{Store: s, Web: staticFixture()})
	me := c.do("GET", "/api/v1/me", nil, 200)
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	m, err := media.New(s).Upload(context.Background(), me["collectionId"].(string), &pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/me", "/api/v1/media/" + m.ID} {
		r := httptest.NewRequest("GET", "http://localhost"+path, nil)
		r.Header.Set("Accept-Encoding", "gzip")
		r.AddCookie(c.cookie)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Content-Encoding") != "" || !strings.Contains(w.Header().Get("Cache-Control"), "no-") {
			t.Fatal("private response policy changed")
		}
		if strings.Contains(strings.ToLower(strings.Join(w.Header().Values("Vary"), ",")), "accept-encoding") {
			t.Fatal("public compression negotiation reached private route")
		}
		if strings.Contains(path, "/media/") {
			if !bytes.HasPrefix(w.Body.Bytes(), []byte{0xff, 0xd8}) {
				t.Fatal("media no longer original JPEG")
			}
		} else if !json.Valid(w.Body.Bytes()) {
			t.Fatal("API no longer plain JSON")
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"A-long-test-password"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Content-Encoding") != "" || w.Header().Get("Cache-Control") != "no-store" || !json.Valid(w.Body.Bytes()) {
		t.Fatal("auth response compression changed")
	}
}

func TestStaticConditionalResponseFraming(t *testing.T) {
	files := staticFixture()
	h := New(Dependencies{Web: files})
	for _, method := range []string{"GET", "HEAD"} {
		for _, encoding := range []string{"identity", "gzip"} {
			ordinary := staticRequest(h, "GET", "/assets/app-hash.js", encoding)
			for _, condition := range []struct {
				name, header, value string
				status              int
			}{
				{"failed-match", "If-Match", `"missing"`, 412},
				{"match-existing", "If-Match", "*", 200},
				{"not-modified", "If-None-Match", "*", 304},
				{"modified", "If-None-Match", `"missing"`, 200},
			} {
				t.Run(method+"/"+encoding+"/"+condition.name, func(t *testing.T) {
					r := httptest.NewRequest(method, "http://localhost/assets/app-hash.js", nil)
					r.Header.Set("Accept-Encoding", encoding)
					r.Header.Set(condition.header, condition.value)
					w := httptest.NewRecorder()
					h.ServeHTTP(w, r)
					response := w.Result()
					defer response.Body.Close()
					body, err := io.ReadAll(response.Body)
					if err != nil {
						t.Fatal("conditional response body did not read to completion", err)
					}
					if response.StatusCode != condition.status {
						t.Fatalf("status=%d want%d", response.StatusCode, condition.status)
					}
					assertStaticVary(t, response.Header, false)
					if condition.status == 412 || condition.status == 304 {
						if len(body) != 0 || response.Header.Get("Content-Encoding") != "" {
							t.Fatalf("empty conditional response has body=%d encoding=%q", len(body), response.Header.Get("Content-Encoding"))
						}
						if condition.status == 412 && (response.ContentLength != 0 || response.Header.Get("Content-Length") != "0") {
							t.Fatalf("empty412 advertises length=%d header=%q", response.ContentLength, response.Header.Get("Content-Length"))
						}
						if condition.status == 304 && response.Header.Get("Content-Length") != "" {
							t.Fatal("304 retained selected length")
						}
						return
					}
					for _, key := range []string{"Content-Length", "Content-Encoding", "Content-Type", "Cache-Control"} {
						if response.Header.Get(key) != ordinary.Header().Get(key) {
							t.Fatalf("successful conditional %s changed", key)
						}
					}
					if method == "HEAD" {
						if len(body) != 0 {
							t.Fatal("conditional HEAD has body")
						}
						return
					}
					if encoding == "gzip" {
						body = gunzipStatic(t, body)
					}
					if !bytes.Equal(body, files["assets/app-hash.js"].Data) {
						t.Fatal("successful conditional changed asset bytes")
					}
				})
			}
		}
	}
}
