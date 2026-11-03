package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	Role         string `json:"role"`
	Version      int    `json:"version"`
	Disabled     bool   `json:"disabled"`
	CollectionID string `json:"-"`
}
type Service struct{ Store *store.Store }

func New(s *store.Store) *Service { return &Service{Store: s} }

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,64}$`)

func Hash(token string) string { b := sha256.Sum256([]byte(token)); return hex.EncodeToString(b[:]) }
func CSRF(token string) string { return Hash("csrf:" + token) }
func Equal(a, b string) bool   { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func validUser(username, display, role string) error {
	if !usernamePattern.MatchString(username) || strings.TrimSpace(display) == "" || utf8.RuneCountInString(display) > 100 || role != "admin" && role != "user" {
		return fmt.Errorf("Ungültige Kontodaten")
	}
	return nil
}
func (a *Service) CreateUser(ctx context.Context, username, display, role, password string) (User, error) {
	if e := validUser(username, display, role); e != nil {
		return User{}, e
	}
	hash, e := HashPassword(password)
	if e != nil {
		return User{}, e
	}
	tx, e := a.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback()
	u, e := createUser(ctx, tx, username, display, role, hash)
	if e != nil {
		return User{}, e
	}
	return u, tx.Commit()
}
func createUser(ctx context.Context, tx *sql.Tx, username, display, role, hash string) (User, error) {
	u := User{ID: store.ID(), Username: fields.Normalize(username), DisplayName: strings.TrimSpace(display), Role: role, Version: 1, CollectionID: store.ID()}
	_, e := tx.ExecContext(ctx, "INSERT INTO users(id,username,display_name,role,password_hash,created_at) VALUES(?,?,?,?,?,?)", u.ID, u.Username, u.DisplayName, u.Role, hash, time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return User{}, fmt.Errorf("Benutzername nicht verfügbar")
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO collections(id,user_id) VALUES(?,?)", u.CollectionID, u.ID)
	return u, e
}
func (a *Service) Setup(ctx context.Context, username, display, password string) (User, error) {
	return a.SetupInitialized(ctx, username, display, password, nil)
}
func (a *Service) SetupInitialized(ctx context.Context, username, display, password string, init func(context.Context, *sql.Tx, User) error) (User, error) {
	if e := validUser(username, display, "admin"); e != nil {
		return User{}, e
	}
	hash, e := HashPassword(password)
	if e != nil {
		return User{}, e
	}
	tx, e := a.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&n); e != nil {
		return User{}, e
	}
	if n > 0 {
		return User{}, fmt.Errorf("setup already completed")
	}
	u, e := createUser(ctx, tx, username, display, "admin", hash)
	if e != nil {
		return User{}, e
	}
	if init != nil {
		if e = init(ctx, tx, u); e != nil {
			return User{}, e
		}
	}
	return u, tx.Commit()
}
func (a *Service) Login(ctx context.Context, username, password string) (User, error) {
	normalized, valid := loginUsername(username)
	if !valid {
		return User{}, fmt.Errorf("Anmeldung fehlgeschlagen")
	}
	var u User
	var hash string
	e := a.Store.DB.QueryRowContext(ctx, "SELECT u.id,u.username,u.display_name,u.role,u.version,u.disabled,u.password_hash,c.id FROM users u JOIN collections c ON c.user_id=u.id WHERE username=?", normalized).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Version, &u.Disabled, &hash, &u.CollectionID)
	matched, eligible := checkLoginPassword(hash, password, e == nil)
	if e != nil || !matched || !eligible || u.Disabled {
		return User{}, fmt.Errorf("Anmeldung fehlgeschlagen")
	}
	return u, nil
}
func (a *Service) StartSession(ctx context.Context, userID string) (string, error) {
	token := store.Token(32)
	now := time.Now().Unix()
	_, e := a.Store.DB.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_hash,created_at,last_used,expires_at) VALUES(?,?,?,?,?,?)", Hash(token), userID, Hash(CSRF(token)), now, now, now+30*86400)
	return token, e
}
func (a *Service) Session(ctx context.Context, token string) (User, error) {
	if len(token) != 64 {
		return User{}, fmt.Errorf("session missing")
	}
	now := time.Now().Unix()
	var u User
	e := a.Store.DB.QueryRowContext(ctx, "SELECT u.id,u.username,u.display_name,u.role,u.version,u.disabled,c.id FROM sessions s JOIN users u ON u.id=s.user_id JOIN collections c ON c.user_id=u.id WHERE s.token_hash=? AND s.expires_at>? AND s.last_used>? AND u.disabled=0", Hash(token), now, now-7*86400).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Version, &u.Disabled, &u.CollectionID)
	if e != nil {
		return u, fmt.Errorf("session expired")
	}
	_, e = a.Store.DB.ExecContext(ctx, "UPDATE sessions SET last_used=? WHERE token_hash=? AND last_used<?", now, Hash(token), now-60)
	return u, e
}
func (a *Service) Invite(ctx context.Context, username, display, role string) (User, string, error) {
	if e := validUser(username, display, role); e != nil {
		return User{}, "", e
	}
	tx, e := a.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return User{}, "", e
	}
	defer tx.Rollback()
	u, e := createUser(ctx, tx, username, display, role, "invitation-pending")
	if e != nil {
		return User{}, "", e
	}
	token := store.Token(32)
	_, e = tx.ExecContext(ctx, "INSERT INTO one_time_tokens(token_hash,user_id,purpose,expires_at) VALUES(?,?,'invite',?)", Hash(token), u.ID, time.Now().Add(24*time.Hour).Unix())
	if e != nil {
		return User{}, "", e
	}
	return u, token, tx.Commit()
}
func (a *Service) ConsumeInvitation(ctx context.Context, token, password, display string) (User, error) {
	return a.ConsumeInvitationInitialized(ctx, token, password, display, nil)
}
func (a *Service) ConsumeInvitationInitialized(ctx context.Context, token, password, display string, init func(context.Context, *sql.Tx, User) error) (User, error) {
	hash, e := HashPassword(password)
	if e != nil {
		return User{}, e
	}
	tx, e := a.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback()
	var u User
	e = tx.QueryRowContext(ctx, "SELECT u.id,u.username,u.display_name,u.role,u.version,u.disabled,c.id FROM one_time_tokens t JOIN users u ON u.id=t.user_id JOIN collections c ON c.user_id=u.id WHERE t.token_hash=? AND t.purpose='invite' AND t.expires_at>? AND u.disabled=0", Hash(token), time.Now().Unix()).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Version, &u.Disabled, &u.CollectionID)
	if e != nil {
		return User{}, fmt.Errorf("Einladung ungültig oder abgelaufen")
	}
	if display != "" {
		if utf8.RuneCountInString(display) > 100 {
			return User{}, fmt.Errorf("Anzeigename zu lang")
		}
		u.DisplayName = strings.TrimSpace(display)
	}
	if _, e = tx.ExecContext(ctx, "UPDATE users SET password_hash=?,display_name=?,version=version+1 WHERE id=?", hash, u.DisplayName, u.ID); e != nil {
		return User{}, e
	}
	u.Version++
	if _, e = tx.ExecContext(ctx, "DELETE FROM one_time_tokens WHERE token_hash=?", Hash(token)); e != nil {
		return User{}, e
	}
	if init != nil {
		if e = init(ctx, tx, u); e != nil {
			return User{}, e
		}
	}
	return u, tx.Commit()
}
func (a *Service) ResetPassword(ctx context.Context, username, password string) error {
	hash, e := HashPassword(password)
	if e != nil {
		return e
	}
	tx, e := a.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var id string
	if e = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE username=?", fields.Normalize(username)).Scan(&id); e != nil {
		return fmt.Errorf("Konto nicht gefunden")
	}
	if _, e = tx.ExecContext(ctx, "UPDATE users SET password_hash=?,version=version+1 WHERE id=?", hash, id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM one_time_tokens WHERE user_id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}

func loginUsername(username string) (string, bool) {
	// Bound raw input before normalization while allowing Unicode spellings that
	// fold to an existing ASCII username, and ordinary surrounding whitespace.
	if len(username) > 256 {
		return "", false
	}
	normalized := fields.Normalize(username)
	if !usernamePattern.MatchString(normalized) {
		return "", false
	}
	return normalized, true
}
