package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/config"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

const SchemaVersion = 1

type Store struct {
	DB     *sql.DB
	Dir    string
	Writer sync.Mutex
	lock   *os.File
}

func Open(ctx context.Context, c config.Config) (s *Store, err error) {
	if c.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	if err = os.MkdirAll(c.DataDir, 0700); err != nil {
		return nil, err
	}
	info, e := os.Stat(c.DataDir)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode().Perm()&0222 == 0 {
		return nil, fmt.Errorf("data directory is not writable")
	}
	if info.Mode().Perm()&0007 != 0 {
		if err = os.Chmod(c.DataDir, 0700); err != nil {
			return nil, fmt.Errorf("cannot secure data directory: %w", err)
		}
	}
	probe, e := os.CreateTemp(c.DataDir, ".write-probe-")
	if e != nil {
		return nil, fmt.Errorf("data directory: %w", e)
	}
	probe.Close()
	os.Remove(probe.Name())
	lock, e := os.OpenFile(filepath.Join(c.DataDir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		lock.Close()
		return nil, fmt.Errorf("another instance is using data directory: %w", e)
	}
	s = &Store{Dir: c.DataDir, lock: lock}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	p, _ := filepath.Abs(filepath.Join(c.DataDir, "puffolio.db"))
	dsn := (&url.URL{Scheme: "file", Path: p}).String() + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)"
	s.DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s.DB.SetMaxOpenConns(1)
	s.DB.SetMaxIdleConns(1)
	if err = s.DB.PingContext(ctx); err != nil {
		return nil, err
	}
	var version int
	if err = s.DB.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if version > SchemaVersion {
		return nil, fmt.Errorf("database schema %d is newer than supported %d", version, SchemaVersion)
	}
	if version == 0 {
		var count int
		if err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count); err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, fmt.Errorf("unrecognized existing database; refusing initialization")
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return nil, e
		}
		b, _ := migrations.ReadFile("migrations/001_initial.sql")
		if _, e = tx.ExecContext(ctx, string(b)); e != nil {
			tx.Rollback()
			return nil, e
		}
		sum := sha256.Sum256(b)
		if _, e = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(1,?,?)", hex.EncodeToString(sum[:]), time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
			tx.Rollback()
			return nil, e
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
	}
	expectedBytes, _ := migrations.ReadFile("migrations/001_initial.sql")
	expected := sha256.Sum256(expectedBytes)
	var checksum string
	if err = s.DB.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=1").Scan(&checksum); err != nil {
		return nil, fmt.Errorf("migration record invalid: %w", err)
	}
	if checksum != hex.EncodeToString(expected[:]) {
		return nil, fmt.Errorf("migration checksum mismatch")
	}
	os.Chmod(p, 0600)
	if err = os.MkdirAll(filepath.Join(c.DataDir, "media"), 0700); err != nil {
		return nil, err
	}
	var count int
	if err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		path := filepath.Join(c.DataDir, "setup-code")
		if _, e = os.Stat(path); os.IsNotExist(e) {
			if err = os.WriteFile(path, []byte(Token(32)+"\n"), 0600); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var e error
	if s.DB != nil {
		e = s.DB.Close()
	}
	if s.lock != nil {
		syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		s.lock.Close()
	}
	return e
}
func Token(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
