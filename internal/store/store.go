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
	"syscall"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

const SchemaVersion = 2

// migrationFiles run in order, one per schema version. A published script never
// changes: its checksum is stored when it runs and compared at every start.
var migrationFiles = [SchemaVersion]string{
	"migrations/001_initial.sql",
	"migrations/002_palette.sql",
}

// BackupSchemaSupported tells whether a backup of that schema can be brought in: older
// ones can, because every migration keeps what they contain. Newer ones cannot.
func BackupSchemaSupported(v int) bool { return v >= 1 && v <= SchemaVersion }

type Store struct {
	DB     *sql.DB
	Dir    string
	Writer WriterLock
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
	// "return nil, err" clears s before deferred calls run, so the cleanup keeps its own reference.
	opened := s
	defer func() {
		if err != nil {
			opened.Close()
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
	}
	if err = verifyMigrations(ctx, s.DB, version); err != nil {
		return nil, err
	}
	for next := version + 1; next <= SchemaVersion; next++ {
		if err = applyMigration(ctx, s.DB, next); err != nil {
			return nil, fmt.Errorf("migration to schema %d failed: %w", next, err)
		}
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
func migrationScript(version int) ([]byte, string, error) {
	script, e := migrations.ReadFile(migrationFiles[version-1])
	if e != nil {
		return nil, "", e
	}
	sum := sha256.Sum256(script)
	return script, hex.EncodeToString(sum[:]), nil
}

// verifyMigrations refuses a database whose history differs from the scripts in this build.
func verifyMigrations(ctx context.Context, db *sql.DB, applied int) error {
	for version := 1; version <= applied; version++ {
		_, expected, e := migrationScript(version)
		if e != nil {
			return e
		}
		var checksum string
		if e = db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=?", version).Scan(&checksum); e != nil {
			return fmt.Errorf("migration record invalid: %w", e)
		}
		if checksum != expected {
			return fmt.Errorf("migration checksum mismatch")
		}
	}
	return nil
}

// applyMigration runs one script atomically: it either completes or leaves the database as it was.
//
// Foreign keys stay off meanwhile. A script may rebuild a table, and with foreign keys on,
// DROP TABLE deletes every row that references it through ON DELETE CASCADE, here all tag
// assignments. SQLite ignores the setting inside a transaction and keeps it per connection,
// so this one connection does everything, and the check before the commit replaces the
// enforcement that was off.
func applyMigration(ctx context.Context, db *sql.DB, version int) (err error) {
	script, sum, err := migrationScript(version)
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err = setForeignKeys(ctx, conn, 0); err != nil {
		return err
	}
	defer func() {
		if e := setForeignKeys(ctx, conn, 1); e != nil && err == nil {
			err = e
		}
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, string(script)); err != nil {
		return err
	}
	var reached, violations int
	if err = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&reached); err != nil {
		return err
	}
	if reached != version {
		return fmt.Errorf("script left schema %d", reached)
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM pragma_foreign_key_check").Scan(&violations); err != nil {
		return err
	}
	if violations != 0 {
		return fmt.Errorf("%d broken foreign key references", violations)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(?,?,?)", version, sum, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}
func setForeignKeys(ctx context.Context, conn *sql.Conn, want int) error {
	if _, e := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA foreign_keys=%d", want)); e != nil {
		return e
	}
	var got int
	if e := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&got); e != nil {
		return e
	}
	if got != want {
		return fmt.Errorf("foreign keys stayed %d", got)
	}
	return nil
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
