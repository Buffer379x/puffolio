package auth

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"testing"
	"time"
)

func TestSessionsAndDisable(t *testing.T) {
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s)
	u, e := a.CreateUser(context.Background(), "alice", "Alice", "admin", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := a.StartSession(context.Background(), u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.Session(context.Background(), raw); e != nil {
		t.Fatal(e)
	}
	var stored string
	s.DB.QueryRow("SELECT token_hash FROM sessions").Scan(&stored)
	if stored == raw {
		t.Fatal("raw session persisted")
	}
	s.DB.Exec("UPDATE users SET disabled=1 WHERE id=?", u.ID)
	if _, e = a.Session(context.Background(), raw); e == nil {
		t.Fatal("disabled user session accepted")
	}
}
func TestPasswordHash(t *testing.T) {
	hash, e := HashPassword("A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	if !VerifyPassword(hash, "A-long-test-password") || VerifyPassword(hash, "wrong") {
		t.Fatal("password verification")
	}
}
func TestSetupInitializationFailureRollsBack(t *testing.T) {
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s)
	_, e = a.SetupInitialized(context.Background(), "alice", "Alice", "A-long-test-password", func(context.Context, *sql.Tx, User) error { return errors.New("synthetic initialization failure") })
	if e == nil {
		t.Fatal("initialization failure ignored")
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM users").Scan(&n)
	if n != 0 {
		t.Fatal("partial setup persisted")
	}
}
func TestSessionIdleAndAbsoluteExpiry(t *testing.T) {
	s, e := store.Open(context.Background(), config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s)
	u, e := a.CreateUser(context.Background(), "alice", "Alice", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	idle, _ := a.StartSession(context.Background(), u.ID)
	s.DB.Exec("UPDATE sessions SET last_used=? WHERE token_hash=?", time.Now().Add(-8*24*time.Hour).Unix(), Hash(idle))
	if _, e = a.Session(context.Background(), idle); e == nil {
		t.Fatal("idle expiry ignored")
	}
	absolute, _ := a.StartSession(context.Background(), u.ID)
	s.DB.Exec("UPDATE sessions SET expires_at=? WHERE token_hash=?", time.Now().Add(-time.Second).Unix(), Hash(absolute))
	if _, e = a.Session(context.Background(), absolute); e == nil {
		t.Fatal("absolute expiry ignored")
	}
}
