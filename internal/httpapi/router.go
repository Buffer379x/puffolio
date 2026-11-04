package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Dependencies struct {
	Store   *store.Store
	Config  config.Config
	Web     fs.FS
	Version string
}
type server struct {
	Dependencies
	auth         *auth.Service
	mux          *http.ServeMux
	rateMu       sync.Mutex
	attempts     map[string]attempt
	previewMu    sync.Mutex
	previews     map[string]preview
	publicAssets map[string]staticAsset
}
type attempt struct {
	Count int
	Until time.Time
}
type principal struct {
	User  auth.User
	Token string
	// Visitor marks an anonymous reader. User is then the collection's owner, so
	// read handlers need no special case; anything private must check this flag.
	Visitor bool
}
type contextKey int

const principalKey contextKey = 1

func current(r *http.Request) principal {
	p, _ := r.Context().Value(principalKey).(principal)
	return p
}
func New(d Dependencies) http.Handler {
	s := &server{Dependencies: d, auth: auth.New(d.Store), mux: http.NewServeMux(), attempts: map[string]attempt{}, previews: map[string]preview{}, publicAssets: loadStaticAssets(d.Web)}
	s.routes()
	return s.middleware(s.mux)
}
func (s *server) routes() {
	m := s.mux
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if e := s.Store.DB.PingContext(r.Context()); e != nil {
			respondError(w, fault.New(503, "not_ready", "Datenbank nicht bereit"))
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	m.HandleFunc("GET /api/v1/setup/status", s.setupStatus)
	m.HandleFunc("POST /api/v1/setup", s.setup)
	m.HandleFunc("POST /api/v1/auth/login", s.login)
	s.handle("GET /api/v1/me", s.me)
	s.handle("PATCH /api/v1/me", s.profile)
	s.handle("POST /api/v1/auth/logout", s.logout)
	s.handle("POST /api/v1/auth/password", s.password)
	s.public("GET /api/v1/fields", s.listFields)
	s.handle("POST /api/v1/fields", s.createField)
	s.handle("PATCH /api/v1/fields/{id}", s.updateField)
	s.handle("DELETE /api/v1/fields/{id}", s.deleteField)
	s.handle("POST /api/v1/fields/{id}/archive", s.archiveField)
	s.handle("PUT /api/v1/fields/order", s.orderFields)
	s.handle("GET /api/v1/fields/{id}/usage", s.fieldUsage)
	s.public("GET /api/v1/fields/{id}/tags", s.listTags)
	s.handle("POST /api/v1/fields/{id}/tags", s.createTag)
	s.handle("PATCH /api/v1/fields/{id}/tags/{tagId}", s.updateTag)
	s.public("GET /api/v1/liquids", s.listLiquids)
	s.handle("POST /api/v1/liquids", s.createLiquid)
	s.public("GET /api/v1/liquids/{id}", s.getLiquid)
	s.handle("PATCH /api/v1/liquids/{id}", s.updateLiquid)
	s.handle("DELETE /api/v1/liquids/{id}", s.trashLiquid)
	s.handle("POST /api/v1/liquids/{id}/restore", s.restoreLiquid)
	s.handle("POST /api/v1/liquids/bulk", s.bulkLiquids)
	s.public("GET /api/v1/preferences", s.getPreferences)
	s.handle("PUT /api/v1/preferences", s.putPreferences)
	s.public("GET /api/v1/dashboard", s.getDashboard)
	s.handle("PUT /api/v1/dashboard", s.putDashboard)
	s.public("POST /api/v1/dashboard/query", s.queryDashboard)
	s.handle("GET /api/v1/saved-views", s.listViews)
	s.handle("POST /api/v1/saved-views", s.createView)
	s.handle("PATCH /api/v1/saved-views/{id}", s.updateView)
	s.handle("DELETE /api/v1/saved-views/{id}", s.deleteView)
	s.handle("POST /api/v1/media", s.uploadMedia)
	s.public("GET /api/v1/media/{id}", s.getMedia)
	s.handle("GET /api/v1/media/{id}/info", s.mediaInfo)
	s.handle("POST /api/v1/media/{id}/crop", s.cropMedia)
	s.handle("DELETE /api/v1/media/{id}", s.deleteMedia)
	s.handle("GET /api/v1/admin/system", s.system)
	s.handle("POST /api/v1/admin/backups", s.backup)
	s.handle("POST /api/v1/exports", s.export)
	s.handle("POST /api/v1/imports/preview", s.importPreview)
	s.handle("POST /api/v1/imports", s.importCommit)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { respondError(w, fault.NotFound()) })
	m.HandleFunc("/", s.static)
}
func (s *server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := store.ID()
		w.Header().Set("X-Request-Id", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		defer func() {
			if v := recover(); v != nil {
				slog.Error("request panic", "requestId", requestID)
				respondError(w, fmt.Errorf("request panic"))
			}
		}()
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if !s.validOrigin(r) {
				respondError(w, fault.New(403, "origin_rejected", "Herkunft nicht erlaubt. PUFFOLIO_BASE_URL muss der aufgerufenen Adresse entsprechen oder leer sein."))
				return
			}
			// Public authentication parses/admit requests in its handler before
			// acquiring the same writer boundary. Never buffer import bodies here.
			publicAuth := r.Method == "POST" && (r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/setup")
			if !publicAuth && !runsOutsideWriter(r) {
				if e := s.Store.Writer.LockContext(r.Context()); e != nil {
					return
				}
				hold := &writerHold{lock: &s.Store.Writer}
				defer hold.release()
				r = r.WithContext(context.WithValue(r.Context(), writerHoldKey, hold))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// runsOutsideWriter lists POST routes that read, or that read a slow request
// body and do expensive work first. Holding the instance-wide writer for those
// would let one upload on a slow connection stall every other user's save.
// Media takes the writer itself for the final publish step.
func runsOutsideWriter(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	path := r.URL.Path
	return path == "/api/v1/media" || path == "/api/v1/dashboard/query" || path == "/api/v1/imports/preview" ||
		(strings.HasPrefix(path, "/api/v1/media/") && strings.HasSuffix(path, "/crop"))
}

type writerHoldContextKey struct{}

var writerHoldKey = writerHoldContextKey{}

// writerHold lets a handler give up the writer before a response that no longer
// depends on it, such as streaming a finished archive to a slow client.
type writerHold struct {
	once sync.Once
	lock *store.WriterLock
}

func (h *writerHold) release() { h.once.Do(h.lock.Unlock) }
func releaseWriter(r *http.Request) {
	if h, ok := r.Context().Value(writerHoldKey).(*writerHold); ok {
		h.release()
	}
}
func (s *server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return false
	}
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.User != nil {
		return false
	}
	if s.Config.PublicURL != "" {
		return origin == strings.TrimRight(s.Config.PublicURL, "/")
	}
	// Without an explicit base URL the browser's Origin must name the host it
	// sent the request to. The scheme is not compared: a TLS-terminating proxy
	// forwards plain HTTP, and the session cookie is host-scoped either way.
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host == r.Host
}

// handle registers a route for the signed-in administrator.
func (s *server) handle(pattern string, h func(http.ResponseWriter, *http.Request) error) {
	s.route(pattern, h, false)
}

// public registers a read route that also serves anonymous visitors. Puffolio has
// one owner who maintains the collection; everyone else may look at it.
func (s *server) public(pattern string, h func(http.ResponseWriter, *http.Request) error) {
	s.route(pattern, h, true)
}
func (s *server) route(pattern string, h func(http.ResponseWriter, *http.Request) error, public bool) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		p := principal{}
		var u auth.User
		cookie, e := r.Cookie("puffolio_session")
		if e == nil {
			u, e = s.auth.Session(r.Context(), cookie.Value)
		}
		if e == nil {
			p = principal{User: u, Token: cookie.Value}
			if r.Method != "GET" && r.Method != "HEAD" && !auth.Equal(r.Header.Get("X-CSRF-Token"), auth.CSRF(cookie.Value)) {
				respondError(w, fault.New(403, "csrf_rejected", "Sicherheitsprüfung fehlgeschlagen"))
				return
			}
		} else {
			if !public {
				respondError(w, fault.New(401, "unauthorized", "Anmeldung erforderlich"))
				return
			}
			owner, e := s.auth.Owner(r.Context())
			if e != nil {
				respondError(w, fault.New(401, "unauthorized", "Anmeldung erforderlich"))
				return
			}
			p = principal{User: owner, Visitor: true}
		}
		r = r.WithContext(context.WithValue(r.Context(), principalKey, p))
		if e = h(w, r); e != nil {
			if fault.Wrap(e).Status == 500 {
				slog.Error("request failed", "requestId", w.Header().Get("X-Request-Id"), "error", e.Error())
			}
			respondError(w, e)
		}
	})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	return decodeLimited(w, r, v, 2<<20)
}
func decodeAuth(w http.ResponseWriter, r *http.Request, v any) error {
	return decodeLimited(w, r, v, 8<<10)
}
func decodeLimited(w http.ResponseWriter, r *http.Request, v any, limit int64) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return fault.New(415, "content_type", "JSON erforderlich")
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			return fault.New(413, "body_too_large", "Anfrage zu groß")
		}
		return fault.New(400, "invalid_json", "Ungültiges JSON")
	}
	if d.Decode(new(any)) != io.EOF {
		return fault.New(400, "invalid_json", "Genau ein JSON-Objekt erforderlich")
	}
	return nil
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func respondError(w http.ResponseWriter, e error) {
	f := fault.Wrap(e)
	body := map[string]any{"code": f.Code, "message": f.Message}
	if len(f.Fields) > 0 {
		body["fields"] = f.Fields
	}
	write(w, f.Status, map[string]any{"error": body, "requestId": w.Header().Get("X-Request-Id")})
}
func (s *server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		respondError(w, fault.NotFound())
		return
	}
	if s.Web == nil {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if !fs.ValidPath(path) {
		http.NotFound(w, r)
		return
	}
	asset, cached := s.publicAssets[path]
	b := asset.identity
	var e error
	if !cached {
		b, e = fs.ReadFile(s.Web, path)
	}
	if e != nil {
		if strings.Contains(filepath.Base(path), ".") {
			http.NotFound(w, r)
			return
		}
		path = "index.html"
		b, e = fs.ReadFile(s.Web, path)
	}
	if e != nil {
		http.NotFound(w, r)
		return
	}
	if cached {
		varyAcceptEncoding(w.Header())
		gzip, acceptable := staticEncoding(r.Header.Values("Accept-Encoding"))
		if !acceptable {
			http.Error(w, "No acceptable content encoding", http.StatusNotAcceptable)
			return
		}
		w.Header().Set("Content-Type", asset.contentType)
		if gzip {
			b = asset.gzip
			w.Header().Set("Content-Encoding", "gzip")
		}
		// ServeContent does not set this for a Content-Encoding response.
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	}
	w.Header().Set("Cache-Control", "no-cache")
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if cached {
		w = staticResponseWriter{w}
	}
	http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(b))
}
func (s *server) rateAllowed(r *http.Request, username string) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	now := time.Now()
	for k, v := range s.attempts {
		if v.Until.Before(now) {
			delete(s.attempts, k)
		}
	}
	if len(s.attempts) > 10000 {
		return false
	}
	ip, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		ip = r.RemoteAddr
	}
	keys := []string{"ip:" + ip, "user:" + auth.Hash(fields.Normalize(username))}
	for _, key := range keys {
		v := s.attempts[key]
		if v.Count >= 10 && v.Until.After(now) {
			return false
		}
	}
	for _, key := range keys {
		v := s.attempts[key]
		if v.Until.Before(now) {
			v = attempt{Until: now.Add(15 * time.Minute)}
		}
		v.Count++
		s.attempts[key] = v
	}
	return true
}
func (s *server) setSession(w http.ResponseWriter, r *http.Request, u auth.User, status int) error {
	token, e := s.auth.StartSession(r.Context(), u.ID)
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "puffolio_session", Value: token, Path: "/", HttpOnly: true, Secure: s.Config.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 86400})
	write(w, status, map[string]any{"user": u, "collectionId": u.CollectionID, "csrfToken": auth.CSRF(token)})
	return nil
}
func readSetupCode(s *store.Store) string {
	b, _ := os.ReadFile(filepath.Join(s.Dir, "setup-code"))
	return strings.TrimSpace(string(b))
}
