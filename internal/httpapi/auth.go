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
	if e != nil {
		respondError(w, fault.New(401, "login_failed", "Anmeldung fehlgeschlagen"))
		return
	}
	if e = s.setSession(w, r, u, 200); e != nil {
		respondError(w, e)
	}
}
func (s *server) consumeInvite(w http.ResponseWriter, r *http.Request) {
	var in struct{ Token, Password, DisplayName string }
	if e := decodeAuth(w, r, &in); e != nil {
		respondError(w, e)
		return
	}
	if !s.rateAllowed(r, "invite") {
		respondError(w, fault.New(429, "rate_limited", "Bitte später erneut versuchen"))
		return
	}
	if e := s.Store.Writer.LockContext(r.Context()); e != nil {
		return
	}
	defer s.Store.Writer.Unlock()
	u, e := s.auth.ConsumeInvitation(r.Context(), in.Token, in.Password, in.DisplayName)
	if e != nil {
		respondError(w, fault.Validation(e.Error()))
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
func (s *server) adminUsers(w http.ResponseWriter, r *http.Request) error {
	if e := requireAdmin(r); e != nil {
		return e
	}
	rows, e := s.Store.DB.QueryContext(r.Context(), "SELECT id,username,display_name,role,disabled,version FROM users ORDER BY username")
	if e != nil {
		return e
	}
	defer rows.Close()
	users := []auth.User{}
	for rows.Next() {
		var u auth.User
		if e = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Disabled, &u.Version); e != nil {
			return e
		}
		users = append(users, u)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	write(w, 200, map[string]any{"items": users})
	return nil
}
func (s *server) adminInvite(w http.ResponseWriter, r *http.Request) error {
	if e := requireAdmin(r); e != nil {
		return e
	}
	var in struct{ Username, DisplayName, Role string }
	if e := decode(w, r, &in); e != nil {
		return e
	}
	u, token, e := s.auth.Invite(r.Context(), in.Username, in.DisplayName, in.Role)
	if e != nil {
		return fault.Validation(e.Error())
	}
	if e = s.audit(r, "user.invited"); e != nil {
		return e
	}
	write(w, 201, map[string]any{"user": u, "invitationToken": token})
	return nil
}
func (s *server) adminUpdate(w http.ResponseWriter, r *http.Request) error {
	if e := requireAdmin(r); e != nil {
		return e
	}
	var in struct {
		Version     int     `json:"version"`
		Disabled    *bool   `json:"disabled,omitempty"`
		Role        *string `json:"role,omitempty"`
		DisplayName *string `json:"displayName,omitempty"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	tx, e := s.Store.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var u auth.User
	if e = tx.QueryRowContext(r.Context(), "SELECT id,username,display_name,role,disabled,version FROM users WHERE id=?", r.PathValue("id")).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Disabled, &u.Version); e == sql.ErrNoRows {
		return fault.NotFound()
	}
	if e != nil {
		return e
	}
	if in.Version != 0 && in.Version != u.Version {
		return fault.Conflict()
	}
	oldAdmin := u.Role == "admin" && !u.Disabled
	if in.Disabled != nil {
		u.Disabled = *in.Disabled
	}
	if in.Role != nil {
		if *in.Role != "admin" && *in.Role != "user" {
			return fault.Validation("Ungültige Rolle")
		}
		u.Role = *in.Role
	}
	if in.DisplayName != nil {
		u.DisplayName = strings.TrimSpace(*in.DisplayName)
		if u.DisplayName == "" || utf8.RuneCountInString(u.DisplayName) > 100 {
			return fault.Validation("Ungültiger Anzeigename")
		}
	}
	if oldAdmin && (u.Disabled || u.Role != "admin") {
		var n int
		if e = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM users WHERE role='admin' AND disabled=0").Scan(&n); e != nil {
			return e
		}
		if n <= 1 {
			return fault.New(409, "last_admin", "Letzter aktiver Administrator muss erhalten bleiben")
		}
	}
	u.Version++
	if _, e = tx.ExecContext(r.Context(), "UPDATE users SET display_name=?,role=?,disabled=?,version=? WHERE id=?", u.DisplayName, u.Role, u.Disabled, u.Version, u.ID); e != nil {
		return e
	}
	if u.Disabled {
		if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", u.ID); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.audit(r, "user.updated")
	write(w, 200, u)
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
