package store

import (
	"context"
	"github.com/Buffer379x/puffolio/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFreshStore(t *testing.T) {
	c := config.Config{DataDir: t.TempDir()}
	s, err := Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var v int
	if err = s.DB.QueryRow("PRAGMA user_version").Scan(&v); err != nil || v != 1 {
		t.Fatalf("schema=%d err=%v", v, err)
	}
	var fk int
	s.DB.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Fatal("foreign keys disabled")
	}
	if b, err := os.ReadFile(filepath.Join(c.DataDir, "setup-code")); err != nil || len(b) < 32 {
		t.Fatal("missing random setup code", err)
	}
}
func TestExclusiveInstanceLock(t *testing.T) {
	c := config.Config{DataDir: t.TempDir()}
	s, e := Open(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if other, e := Open(context.Background(), c); e == nil {
		other.Close()
		t.Fatal("second instance acquired lock")
	}
}
func TestRefuseNewerSchema(t *testing.T) {
	c := config.Config{DataDir: t.TempDir()}
	s, e := Open(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	s.DB.Exec("PRAGMA user_version=999")
	s.Close()
	if other, e := Open(context.Background(), c); e == nil {
		other.Close()
		t.Fatal("opened newer schema")
	}
}
func TestDataDirNotWritable(t *testing.T) {
	d := t.TempDir()
	os.Chmod(d, 0500)
	defer os.Chmod(d, 0700)
	if s, e := Open(context.Background(), config.Config{DataDir: d}); e == nil {
		s.Close()
		t.Fatal("accepted read-only directory")
	}
}
func TestDatabaseRejectsWrongTypedValueColumn(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at) VALUES('u','alice','Alice','x','user','now')`)
	s.DB.Exec(`INSERT INTO collections(id,user_id) VALUES('c','u')`)
	s.DB.Exec(`INSERT INTO field_definitions(id,collection_id,type,data) VALUES('f','c','text','{}')`)
	s.DB.Exec(`INSERT INTO liquids(id,collection_id,name,normalized_name,created_at,updated_at) VALUES('l','c','A','a','now','now')`)
	if _, e = s.DB.Exec(`INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_number) VALUES('c','l','f','text',2)`); e == nil {
		t.Fatal("database accepted number in text field")
	}
}
func TestMigrationChecksumIsRecorded(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var sum string
	if e = s.DB.QueryRow("SELECT checksum FROM schema_migrations WHERE version=1").Scan(&sum); e != nil || len(sum) != 64 {
		t.Fatal("missing migration checksum", e)
	}
}
