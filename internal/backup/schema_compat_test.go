package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// taggedCollection fills a store with one liquid carrying one tag.
func taggedCollection(t *testing.T, s *store.Store, cid string) {
	t.Helper()
	ctx := context.Background()
	d, e := fields.Save(ctx, s.DB, cid, fields.Definition{Label: "Aromen", Type: "tags"}, true)
	if e != nil {
		t.Fatal(e)
	}
	tag := store.ID()
	if _, e = s.DB.Exec("INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,?)", tag, cid, d.ID, "Minzig", "minzig", "teal"); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal([]string{tag})
	name := "Getaggt"
	if _, e = liquids.Save(ctx, s.DB, cid, "", liquids.Input{Name: &name, Values: map[string]*fields.Value{d.ID: {Type: "tags", Value: raw}}}, nil); e != nil {
		t.Fatal(e)
	}
}

// claimSchema copies a collection archive and changes the schema it states in its manifest
// and in its data, which is all that differs in an archive of an older release.
func claimSchema(t *testing.T, path string, manifestSchema, dataSchema int) string {
	t.Helper()
	in, e := zip.OpenReader(path)
	if e != nil {
		t.Fatal(e)
	}
	defer in.Close()
	files := map[string][]byte{}
	for _, f := range in.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name], e = io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	current := fmt.Sprintf(`"schemaVersion":%d`, store.SchemaVersion)
	if !bytes.Contains(files["collection.json"], []byte(current)) {
		t.Fatalf("collection data without %s", current)
	}
	files["collection.json"] = bytes.Replace(files["collection.json"], []byte(current), fmt.Appendf(nil, `"schemaVersion":%d`, dataSchema), 1)
	var m Manifest
	if e = json.Unmarshal(files["manifest.json"], &m); e != nil {
		t.Fatal(e)
	}
	m.SchemaVersion = manifestSchema
	sum := sha256.Sum256(files["collection.json"])
	m.Files["collection.json"] = FileDigest{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(files["collection.json"]))}
	if files["manifest.json"], e = json.Marshal(m); e != nil {
		t.Fatal(e)
	}
	return writeZip(t, files)
}
func writeZip(t *testing.T, files map[string][]byte) string {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, content := range files {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		w.Write(content)
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "archive.zip")
	if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	return p
}

// instanceArchive builds an instance backup whose database ran the first dbSchema migrations.
func instanceArchive(t *testing.T, manifestSchema, dbSchema int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "puffolio.db")
	db, e := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	scripts := []string{"001_initial.sql", "002_palette.sql"}
	for i, name := range scripts[:dbSchema] {
		script, e := os.ReadFile(filepath.Join("..", "store", "migrations", name))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(script)); e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(script)
		if _, e = db.Exec("INSERT INTO schema_migrations VALUES(?,?,'2026-01-01T00:00:00Z')", i+1, hex.EncodeToString(sum[:])); e != nil {
			t.Fatal(e)
		}
	}
	for _, statement := range []string{
		`INSERT INTO users(id,username,display_name,password_hash,role,created_at) VALUES('u','alice','Alice','x','admin','now')`,
		`INSERT INTO collections(id,user_id) VALUES('c','u')`,
		`INSERT INTO field_definitions(id,collection_id,type,data) VALUES('f','c','tags','{}')`,
		`INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id,version) VALUES('t1','c','f','Eins','eins','lime',4),('t2','c','f','Zwei','zwei','pink',1)`,
		`INSERT INTO liquids(id,collection_id,name,normalized_name,created_at,updated_at) VALUES('l1','c','A','a','now','now'),('l2','c','B','b','now','now')`,
		`INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES('c','l1','f','t1'),('c','l1','f','t2'),('c','l2','f','t1')`,
	} {
		if _, e = db.Exec(statement); e != nil {
			t.Fatal(e)
		}
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	content, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(content)
	m := Manifest{Kind: "instance", SchemaVersion: manifestSchema, Files: map[string]FileDigest{"puffolio.db": {SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))}}}
	raw, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return writeZip(t, map[string][]byte{"manifest.json": raw, "puffolio.db": content})
}
func TestOlderCollectionArchiveImports(t *testing.T) {
	ctx := context.Background()
	source := reviewStore(t)
	owner := reviewUser(t, source, "owner")
	taggedCollection(t, source, owner.CollectionID)
	archive := claimSchema(t, reviewExport(t, source, owner.CollectionID), 1, 1)
	if _, e := Preview(archive, WebLimits()); e != nil {
		t.Fatal("preview refused an older archive: ", e)
	}
	target := reviewStore(t)
	u := reviewUser(t, target, "target")
	if _, e := Import(ctx, target, u.CollectionID, archive, WebLimits()); e != nil {
		t.Fatal("import refused an older archive: ", e)
	}
	var palette string
	var assigned int
	if e := target.DB.QueryRow("SELECT t.palette_id,count(*) FROM tags t JOIN liquid_tags lt ON lt.tag_id=t.id WHERE t.collection_id=?", u.CollectionID).Scan(&palette, &assigned); e != nil {
		t.Fatal(e)
	}
	if palette != "teal" || assigned != 1 {
		t.Fatalf("palette=%q assigned=%d", palette, assigned)
	}
}
func TestOlderInstanceArchiveRestoresAndMigrates(t *testing.T) {
	ctx := context.Background()
	dest := filepath.Join(t.TempDir(), "restored")
	if e := Restore(ctx, instanceArchive(t, 1, 1), dest); e != nil {
		t.Fatal("restore refused an older archive: ", e)
	}
	s, e := store.Open(ctx, config.Config{DataDir: dest})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var version int
	if e = s.DB.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != store.SchemaVersion {
		t.Fatalf("schema=%d err=%v", version, e)
	}
	rows, e := s.DB.Query("SELECT lt.liquid_id||':'||t.id||':'||t.palette_id||':'||t.version FROM liquid_tags lt JOIN tags t ON t.id=lt.tag_id ORDER BY 1")
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var row string
		if e = rows.Scan(&row); e != nil {
			t.Fatal(e)
		}
		got = append(got, row)
	}
	want := "l1:t1:lime:4 l1:t2:pink:1 l2:t1:lime:4"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v want %s", got, want)
	}
}
func TestNewBackupsCarryCurrentSchema(t *testing.T) {
	ctx := context.Background()
	s := reviewStore(t)
	u := reviewUser(t, s, "owner")
	read := func(path, name string) []byte {
		t.Helper()
		z, e := zip.OpenReader(path)
		if e != nil {
			t.Fatal(e)
		}
		defer z.Close()
		f, e := z.Open(name)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		b, e := io.ReadAll(f)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	collection := reviewExport(t, s, u.CollectionID)
	var m Manifest
	if e := json.Unmarshal(read(collection, "manifest.json"), &m); e != nil || m.SchemaVersion != 2 {
		t.Fatalf("collection manifest schema=%d err=%v", m.SchemaVersion, e)
	}
	if !bytes.Contains(read(collection, "collection.json"), []byte(`"schemaVersion":2`)) {
		t.Fatal("collection data does not state schema 2")
	}
	var b bytes.Buffer
	if e := Create(ctx, s, &b, "test"); e != nil {
		t.Fatal(e)
	}
	instance := filepath.Join(t.TempDir(), "instance.zip")
	if e := os.WriteFile(instance, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(read(instance, "manifest.json"), &m); e != nil || m.SchemaVersion != 2 {
		t.Fatalf("instance manifest schema=%d err=%v", m.SchemaVersion, e)
	}
}
func TestUnsupportedBackupSchemasAreRefused(t *testing.T) {
	s := reviewStore(t)
	u := reviewUser(t, s, "owner")
	export := reviewExport(t, s, u.CollectionID)
	for _, schema := range [][2]int{
		{0, 0},
		{store.SchemaVersion + 1, store.SchemaVersion + 1},
		{1, store.SchemaVersion}, // Manifest and data disagree.
		{store.SchemaVersion, 1},
	} {
		if _, e := Preview(claimSchema(t, export, schema[0], schema[1]), WebLimits()); e == nil {
			t.Errorf("collection archive %v accepted", schema)
		}
	}
	for _, schema := range [][2]int{
		{0, 1},
		{store.SchemaVersion + 1, 1},
		{1, store.SchemaVersion}, // Manifest and database disagree.
		{store.SchemaVersion, 1},
	} {
		dest := filepath.Join(t.TempDir(), "restored")
		if e := Restore(context.Background(), instanceArchive(t, schema[0], schema[1]), dest); e == nil {
			t.Errorf("instance archive %v accepted", schema)
		}
	}
}
