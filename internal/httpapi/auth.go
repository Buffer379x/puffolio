package httpapi

import (
	"context"
	"database/sql"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *server) setupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	if e := s.Store.DB.QueryRowContext(r.Context(), "SELECT count(*) FROM users").Scan(&count); e != nil {
		respondError(w, e)
		return
	}
	write(w, 200, map[string]bool{"setupRequired": count == 0})
}
func (s *server) setup(w http.ResponseWriter, r *http.Request) {
	var n int
	s.Store.DB.QueryRowContext(r.Context(), "SELECT count(*) FROM users").Scan(&n)
	if n > 0 {
		respondError(w, fault.New(409, "setup_completed", "Einrichtung bereits abgeschlossen"))
		return
	}
	var in struct {
		Code, Username, DisplayName, Password string
		StarterFields                         bool `json:"starterFields"`
	}
	if e := decodeAuth(w, r, &in); e != nil {
		respondError(w, e)
		return
	}
	if !s.rateAllowed(r, "setup") {
		respondError(w, fault.New(429, "rate_limited", "Bitte später erneut versuchen"))
		return
	}
	code := readSetupCode(s.Store)
	if code == "" || !auth.Equal(code, in.Code) {
		respondError(w, fault.New(403, "invalid_code", "Einrichtungscode ungültig"))
		return
	}
	if e := s.Store.Writer.LockContext(r.Context()); e != nil {
		return
	}
	defer s.Store.Writer.Unlock()
	u, e := s.auth.SetupInitialized(r.Context(), in.Username, in.DisplayName, in.Password, func(ctx context.Context, tx *sql.Tx, u auth.User) error {
		roles := map[string]string{}
		var err error
		if in.StarterFields {
			roles, err = fields.StarterTx(ctx, tx, u.CollectionID)
			if err != nil {
				return err
			}
		}
		return auth.InitializeUserTx(ctx, tx, u, roles)
	})
	if e != nil {
		respondError(w, fault.Validation(e.Error()))
		return
	}
	os.Remove(filepath.Join(s.Store.Dir, "setup-code"))
	if e = s.setSession(w, r, u, 201); e != nil {
		respondError(w, e)
	}
}
func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if e := decodeAuth(w, r, &in); e != nil {
		respondError(w, e)
		return
	}
	if !s.rateAllowed(r, in.Username) {
		respondError(w, fault.New(429, "rate_limited", "Bitte später erneut versuchen"))
		return
	}
	if e := s.Store.Writer.LockContext(r.Context()); e != nil {
		return
	}
	defer s.Store.Writer.Unlock()
	u, e := s.auth.Login(r.Context(), in.Username, in.Password)
	// Only administrators sign in; accounts left over from 0.1.0 keep their data.
	if e != nil || u.Role != "admin" {
		respondError(w, fault.New(401, "login_failed", "Anmeldung fehlgeschlagen"))
		return
	}
	if e = s.setSession(w, r, u, 200); e != nil {
		respondError(w, e)
	}
}
func (s *server) me(w http.ResponseWriter, r *http.Request) error {
	p := current(r)
	write(w, 200, map[string]any{"user": p.User, "collectionId": p.User.CollectionID, "csrfToken": auth.CSRF(p.Token)})
	return nil
}
func (s *server) profile(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		DisplayName string `json:"displayName"`
		Username    string `json:"username,omitempty"`
		Version     int    `json:"version"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	u := current(r).User
	if in.Version != u.Version {
		return fault.Conflict()
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.DisplayName == "" || utf8.RuneCountInString(in.DisplayName) > 100 {
		return fault.Validation("Anzeigename erforderlich (maximal 100 Zeichen)")
	}
	if in.Username != "" && in.Username != u.Username {
		return fault.Validation("Benutzername kann nicht geändert werden")
	}
	_, e := s.Store.DB.ExecContext(r.Context(), "UPDATE users SET display_name=?,version=version+1 WHERE id=? AND version=?", in.DisplayName, u.ID, u.Version)
	if e != nil {
		return e
	}
	u.DisplayName = in.DisplayName
	u.Version++
	p := current(r)
	write(w, 200, map[string]any{"user": u, "collectionId": u.CollectionID, "csrfToken": auth.CSRF(p.Token)})
	return nil
}
func (s *server) logout(w http.ResponseWriter, r *http.Request) error {
	_, e := s.Store.DB.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", auth.Hash(current(r).Token))
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "puffolio_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Config.SecureCookie, SameSite: http.SameSiteLaxMode})
	write(w, 200, map[string]bool{"success": true})
	return nil
}
func (s *server) password(w http.ResponseWriter, r *http.Request) error {
	var in struct{ CurrentPassword, NewPassword string }
	if e := decode(w, r, &in); e != nil {
		return e
	}
	u := current(r).User
	if _, e := s.auth.Login(r.Context(), u.Username, in.CurrentPassword); e != nil {
		return fault.New(403, "password_invalid", "Passwort ungültig")
	}
	hash, e := auth.HashPassword(in.NewPassword)
	if e != nil {
		return fault.Validation(e.Error())
	}
	tx, e := s.Store.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(r.Context(), "UPDATE users SET password_hash=?,version=version+1 WHERE id=?", hash, u.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=? AND token_hash!=?", u.ID, auth.Hash(current(r).Token)); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	write(w, 200, map[string]bool{"success": true})
	return nil
}
func requireAdmin(r *http.Request) error {
	if current(r).User.Role != "admin" {
		return fault.New(403, "forbidden", "Administration erforderlich")
	}
	return nil
}
func (s *server) audit(r *http.Request, action string) error {
	_, e := s.Store.DB.ExecContext(r.Context(), "INSERT INTO audit_events(id,user_id,action,created_at) VALUES(?,?,?,?)", store.ID(), current(r).User.ID, action, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}
func (s *server) system(w http.ResponseWriter, r *http.Request) error {
	if e := requireAdmin(r); e != nil {
		return e
	}
	var integrity string
	if e := s.Store.DB.QueryRowContext(r.Context(), "PRAGMA quick_check").Scan(&integrity); e != nil {
		return e
	}
	write(w, 200, map[string]any{"version": s.Version, "schemaVersion": store.SchemaVersion, "database": integrity})
	return nil
}
