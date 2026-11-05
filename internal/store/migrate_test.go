package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/config"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func rawDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, e := sql.Open("sqlite", "file:"+filepath.Join(dir, "puffolio.db")+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}
func checksum(t *testing.T, file string) string {
	t.Helper()
	b, e := migrations.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func mustExec(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	if _, e := db.Exec(statement, args...); e != nil {
		t.Fatalf("%s: %v", statement, e)
	}
}

// schemaOneDatabase is a database as every release up to 0.3.0 left it: tags in use, one renamed.
func schemaOneDatabase(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	db := rawDB(t, dir)
	script, e := migrations.ReadFile("migrations/001_initial.sql")
	if e != nil {
		t.Fatal(e)
	}
	mustExec(t, db, string(script))
	mustExec(t, db, "INSERT INTO schema_migrations VALUES(1,?,'2026-01-01T00:00:00Z')", checksum(t, "migrations/001_initial.sql"))
	mustExec(t, db, `INSERT INTO users(id,username,display_name,password_hash,role,created_at) VALUES('u','alice','Alice','x','admin','now')`)
	mustExec(t, db, `INSERT INTO collections(id,user_id) VALUES('c','u')`)
	mustExec(t, db, `INSERT INTO field_definitions(id,collection_id,type,data) VALUES('tagfield','c','tags','{}'),('textfield','c','text','{}')`)
	mustExec(t, db, `INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id,version) VALUES('t1','c','tagfield','Erdbeere','erdbeere','lime',3),('t2','c','tagfield','Minze','minze','pink',1),('t3','c','tagfield','Ungenutzt','ungenutzt','slate',2)`)
	mustExec(t, db, `INSERT INTO liquids(id,collection_id,name,normalized_name,created_at,updated_at) VALUES('l1','c','A','a','now','now'),('l2','c','B','b','now','now')`)
	mustExec(t, db, `INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES('c','l1','tagfield','t1'),('c','l1','tagfield','t2'),('c','l2','tagfield','t1')`)
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	return config.Config{DataDir: dir}
}
func dump(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, e := db.Query(query)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	var out strings.Builder
	for rows.Next() {
		cells := make([]any, len(columns))
		for i := range cells {
			cells[i] = new(any)
		}
		if e = rows.Scan(cells...); e != nil {
			t.Fatal(e)
		}
		for _, cell := range cells {
			fmt.Fprintf(&out, "%v|", *cell.(*any))
		}
		out.WriteByte('\n')
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	return out.String()
}

const (
	dumpTags        = "SELECT id,collection_id,field_id,name,normalized_name,palette_id,version FROM tags ORDER BY id"
	dumpAssignments = "SELECT collection_id,liquid_id,field_id,tag_id FROM liquid_tags ORDER BY liquid_id,tag_id"
	dumpSchema      = "SELECT type,name,tbl_name,sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name"
)

func pragmaInt(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var v int
	if e := db.QueryRow("PRAGMA " + name).Scan(&v); e != nil {
		t.Fatal(e)
	}
	return v
}
func openMigrated(t *testing.T, c config.Config) *Store {
	t.Helper()
	s, e := Open(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestMigrateV1KeepsTagsAndAssignments(t *testing.T) {
	c := schemaOneDatabase(t)
	before := rawDB(t, c.DataDir)
	tags, assignments := dump(t, before, dumpTags), dump(t, before, dumpAssignments)
	if strings.Count(assignments, "\n") != 3 {
		t.Fatalf("fixture has no assignments: %q", assignments)
	}
	before.Close()
	s := openMigrated(t, c)
	if got := dump(t, s.DB, dumpTags); got != tags {
		t.Fatalf("tags changed\nbefore %s\nafter  %s", tags, got)
	}
	if got := dump(t, s.DB, dumpAssignments); got != assignments {
		t.Fatalf("assignments changed\nbefore %s\nafter  %s", assignments, got)
	}
	if v := pragmaInt(t, s.DB, "user_version"); v != 2 {
		t.Fatalf("schema=%d", v)
	}
	if v := pragmaInt(t, s.DB, "foreign_keys"); v != 1 {
		t.Fatal("foreign keys left off after the migration")
	}
	if got := dump(t, s.DB, "PRAGMA foreign_key_check"); got != "" {
		t.Fatalf("foreign key violations: %s", got)
	}
}
func TestMigratedSchemaEnforcesRules(t *testing.T) {
	s := openMigrated(t, schemaOneDatabase(t))
	mustExec(t, s.DB, "UPDATE tags SET palette_id='mint' WHERE id='t1'")
	if _, e := s.DB.Exec("UPDATE tags SET palette_id='#ffffff' WHERE id='t1'"); e == nil {
		t.Fatal("accepted a colour outside the palette")
	}
	if _, e := s.DB.Exec(`INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES('c','l2','tagfield','missing')`); e == nil {
		t.Fatal("accepted an assignment to an unknown tag")
	}
	if _, e := s.DB.Exec(`INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES('t9','c','textfield','Falsch','falsch','lime')`); e == nil {
		t.Fatal("trigger did not reject a tag on a text field")
	}
	if _, e := s.DB.Exec(`INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES('t9','c','tagfield','Doppelt','erdbeere','lime')`); e == nil {
		t.Fatal("accepted a duplicate tag name")
	}
	mustExec(t, s.DB, "DELETE FROM tags WHERE id='t2'")
	var left int
	if e := s.DB.QueryRow("SELECT count(*) FROM liquid_tags WHERE tag_id='t2'").Scan(&left); e != nil || left != 0 {
		t.Fatalf("assignments of a deleted tag remain: %d %v", left, e)
	}
	if e := s.DB.QueryRow("SELECT count(*) FROM liquid_tags WHERE tag_id='t1'").Scan(&left); e != nil || left != 2 {
		t.Fatalf("assignments of another tag lost: %d %v", left, e)
	}
}

// Everything except the colour list has to stay what it was, down to the constraints.
func TestMigrationOnlyWidensPalette(t *testing.T) {
	c := schemaOneDatabase(t)
	before := rawDB(t, c.DataDir)
	old := dump(t, before, dumpSchema)
	before.Close()
	s := openMigrated(t, c)
	list := regexp.MustCompile(`palette_id IN \([^)]*\)`)
	normalize := func(schema string) string {
		return list.ReplaceAllString(strings.ReplaceAll(schema, `"tags"`, "tags"), "palette_id IN (...)")
	}
	if got := dump(t, s.DB, dumpSchema); normalize(got) != normalize(old) {
		t.Fatalf("schema changed beyond the palette\nbefore %s\nafter  %s", normalize(old), normalize(got))
	}
}
func TestFailedMigrationKeepsV1(t *testing.T) {
	c := schemaOneDatabase(t)
	db := rawDB(t, c.DataDir)
	mustExec(t, db, "CREATE TABLE tags_new(id TEXT)")
	tags, assignments, schema := dump(t, db, dumpTags), dump(t, db, dumpAssignments), dump(t, db, dumpSchema)
	db.Close()
	if s, e := Open(context.Background(), c); e == nil {
		s.Close()
		t.Fatal("migration did not fail")
	}
	db = rawDB(t, c.DataDir)
	if v := pragmaInt(t, db, "user_version"); v != 1 {
		t.Fatalf("schema=%d after a failed migration", v)
	}
	if got := dump(t, db, dumpTags); got != tags {
		t.Fatalf("tags changed: %s", got)
	}
	if got := dump(t, db, dumpAssignments); got != assignments {
		t.Fatalf("assignments changed: %s", got)
	}
	if got := dump(t, db, dumpSchema); got != schema {
		t.Fatalf("schema changed: %s", got)
	}
	if got := dump(t, db, "SELECT version FROM schema_migrations WHERE version=2"); got != "" {
		t.Fatal("failed migration was recorded")
	}
	// The next start tries again once the cause is gone.
	mustExec(t, db, "DROP TABLE tags_new")
	db.Close()
	s := openMigrated(t, c)
	if v := pragmaInt(t, s.DB, "user_version"); v != 2 {
		t.Fatalf("schema=%d after the retry", v)
	}
	if got := dump(t, s.DB, dumpAssignments); got != assignments {
		t.Fatalf("assignments lost in the retry: %s", got)
	}
}
func TestDanglingReferenceStopsMigration(t *testing.T) {
	c := schemaOneDatabase(t)
	db := rawDB(t, c.DataDir)
	mustExec(t, db, "PRAGMA foreign_keys=OFF")
	mustExec(t, db, `INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES('c','l2','tagfield','missing')`)
	db.Close()
	if s, e := Open(context.Background(), c); e == nil {
		s.Close()
		t.Fatal("migrated a database with a dangling reference")
	}
	db = rawDB(t, c.DataDir)
	if v := pragmaInt(t, db, "user_version"); v != 1 {
		t.Fatalf("schema=%d", v)
	}
}
func TestFreshDatabaseAppliesAllMigrations(t *testing.T) {
	c := config.Config{DataDir: t.TempDir()}
	s := openMigrated(t, c)
	want := fmt.Sprintf("1|%s|\n2|%s|\n", checksum(t, "migrations/001_initial.sql"), checksum(t, "migrations/002_palette.sql"))
	if got := dump(t, s.DB, "SELECT version,checksum FROM schema_migrations ORDER BY version"); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if v := pragmaInt(t, s.DB, "user_version"); v != SchemaVersion {
		t.Fatalf("schema=%d", v)
	}
	if v := pragmaInt(t, s.DB, "foreign_keys"); v != 1 {
		t.Fatal("foreign keys left off")
	}
	applied := dump(t, s.DB, "SELECT version,applied_at FROM schema_migrations ORDER BY version")
	s.Close()
	// A second start finds nothing to do.
	s = openMigrated(t, c)
	if got := dump(t, s.DB, "SELECT version,applied_at FROM schema_migrations ORDER BY version"); got != applied {
		t.Fatalf("migrations ran again: %s", got)
	}
}
func TestChangedMigrationIsRejected(t *testing.T) {
	expectRefused := func(t *testing.T, c config.Config) {
		t.Helper()
		if s, e := Open(context.Background(), c); e == nil {
			s.Close()
			t.Fatal("opened a database with a wrong migration record")
		}
	}
	t.Run("latest", func(t *testing.T) {
		c := config.Config{DataDir: t.TempDir()}
		s := openMigrated(t, c)
		mustExec(t, s.DB, "UPDATE schema_migrations SET checksum='changed' WHERE version=2")
		s.Close()
		expectRefused(t, c)
	})
	t.Run("missing record", func(t *testing.T) {
		c := config.Config{DataDir: t.TempDir()}
		s := openMigrated(t, c)
		mustExec(t, s.DB, "DELETE FROM schema_migrations WHERE version=2")
		s.Close()
		expectRefused(t, c)
	})
	t.Run("older, before migrating", func(t *testing.T) {
		c := schemaOneDatabase(t)
		db := rawDB(t, c.DataDir)
		mustExec(t, db, "UPDATE schema_migrations SET checksum='changed' WHERE version=1")
		db.Close()
		expectRefused(t, c)
		db = rawDB(t, c.DataDir)
		if v := pragmaInt(t, db, "user_version"); v != 1 {
			t.Fatalf("migrated a database with a wrong record: schema=%d", v)
		}
	})
}
func TestBackupSchemaSupported(t *testing.T) {
	for _, v := range []int{1, SchemaVersion} {
		if !BackupSchemaSupported(v) {
			t.Errorf("schema %d refused", v)
		}
	}
	for _, v := range []int{-1, 0, SchemaVersion + 1} {
		if BackupSchemaSupported(v) {
			t.Errorf("schema %d accepted", v)
		}
	}
}
