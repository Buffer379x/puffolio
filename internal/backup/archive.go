// Package backup implements the portable collection format and offline instance
// restore. Callers hold Store.Writer for export, import and instance snapshots.
package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/media"
	"github.com/Buffer379x/puffolio/internal/store"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Limits struct {
	MaxExpandedBytes      int64
	MaxEntries, MaxImages int
}
type Counts struct {
	Liquids int `json:"liquids"`
	Fields  int `json:"fields"`
	Media   int `json:"media"`
}
type ImportPreview struct {
	Counts   Counts   `json:"counts"`
	Warnings []string `json:"warnings"`
}
type FileDigest struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Manifest struct {
	Kind          string                `json:"kind"`
	AppVersion    string                `json:"appVersion"`
	SchemaVersion int                   `json:"schemaVersion"`
	CreatedAt     string                `json:"createdAt"`
	Files         map[string]FileDigest `json:"files"`
	Counts        *Counts               `json:"counts,omitempty"`
}
type Collection struct {
	liquidCount           int
	latestLiquidCreatedAt time.Time
	SchemaVersion         int                 `json:"schemaVersion"`
	Fields                []fields.Definition `json:"fields"`
	Tags                  []fields.Tag        `json:"tags"`
	Liquids               []liquids.Liquid    `json:"liquids"`
	Media                 []media.Media       `json:"media"`
	Preferences           json.RawMessage     `json:"preferences"`
	Dashboard             json.RawMessage     `json:"dashboard"`
	Views                 []json.RawMessage   `json:"views"`
}

func WebLimits() Limits { return Limits{100 << 20, 10000, 5000} }
func Export(ctx context.Context, s *store.Store, cid string, w io.Writer, version string) error {
	return exportCollection(ctx, s, cid, w, version, nil)
}

// ExportSelected validates the entire bounded selection before writing any archive
// bytes. The full field/tag schema and view configuration remain portable.
func ExportSelected(ctx context.Context, s *store.Store, cid string, w io.Writer, version string, ids []string) error {
	if len(ids) == 0 || len(ids) > 500 {
		return fault.Validation("Auswahl muss 1 bis 500 Liquids enthalten")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return fault.Validation("Ungültige oder doppelte Liquid-ID")
		}
		seen[id] = true
		var owned string
		if e := s.DB.QueryRowContext(ctx, "SELECT id FROM liquids WHERE collection_id=? AND id=?", cid, id).Scan(&owned); e == sql.ErrNoRows {
			return fault.NotFound()
		} else if e != nil {
			return e
		}
	}
	return exportCollection(ctx, s, cid, w, version, ids)
}
func exportCollection(ctx context.Context, s *store.Store, cid string, w io.Writer, version string, selected []string) error {
	var c Collection
	c.SchemaVersion = store.SchemaVersion
	c.Tags = []fields.Tag{}
	c.Liquids = []liquids.Liquid{}
	c.Media = []media.Media{}
	c.Views = []json.RawMessage{}
	var e error
	c.Fields, e = fields.List(ctx, s.DB, cid)
	if e != nil {
		return e
	}
	for _, d := range c.Fields {
		if d.Type == "tags" {
			tags, e := fields.Tags(ctx, s.DB, cid, d.ID)
			if e != nil {
				return e
			}
			c.Tags = append(c.Tags, tags...)
		}
	}
	ids := append([]string{}, selected...)
	if selected == nil {
		rows, e := s.DB.QueryContext(ctx, "SELECT id FROM liquids WHERE collection_id=? ORDER BY id", cid)
		if e != nil {
			return e
		}
		ids = []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}

	} else {
		sort.Strings(ids)
	}
	neededMedia := map[string]bool{}
	if selected != nil {
		for _, d := range c.Fields {
			if d.Type == "image" {
				var id string
				if json.Unmarshal(d.Config.DefaultValue, &id) == nil && id != "" {
					neededMedia[id] = true
				}
			}
		}
		for _, id := range ids {
			l, e := liquids.Get(ctx, s.DB, cid, id)
			if e != nil {
				return e
			}
			for _, v := range l.Values {
				if v.Type == "image" {
					var mid string
					if json.Unmarshal(v.Value, &mid) == nil && mid != "" {
						neededMedia[mid] = true
					}
				}
			}
		}
	}
	var rows *sql.Rows
	rows, e = s.DB.QueryContext(ctx, "SELECT data FROM media WHERE collection_id=? ORDER BY id", cid)
	if e != nil {
		return e
	}
	for rows.Next() {
		var raw string
		var m media.Media
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return e
		}
		if e = json.Unmarshal([]byte(raw), &m); e != nil {
			rows.Close()
			return e
		}
		if selected == nil || neededMedia[m.ID] {
			c.Media = append(c.Media, m)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	var uid string
	if e = s.DB.QueryRowContext(ctx, "SELECT user_id FROM collections WHERE id=?", cid).Scan(&uid); e != nil {
		return e
	}
	var raw string
	if e = s.DB.QueryRowContext(ctx, "SELECT data FROM user_preferences WHERE user_id=?", uid).Scan(&raw); e != nil {
		return e
	}
	c.Preferences = json.RawMessage(raw)
	if e = s.DB.QueryRowContext(ctx, "SELECT data FROM dashboards WHERE user_id=?", uid).Scan(&raw); e != nil {
		return e
	}
	c.Dashboard = json.RawMessage(raw)
	rows, e = s.DB.QueryContext(ctx, "SELECT data FROM saved_views WHERE user_id=? ORDER BY id", uid)
	if e != nil {
		return e
	}
	for rows.Next() {
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return e
		}
		c.Views = append(c.Views, json.RawMessage(raw))
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	z := zip.NewWriter(w)
	manifest := newManifest("collection", version)
	counts := Counts{len(ids), len(c.Fields), len(c.Media)}
	manifest.Counts = &counts
	if e = addCollection(ctx, z, &manifest, c, s, cid, ids); e != nil {
		return e
	}
	for _, m := range c.Media {
		for _, name := range []string{"master.jpg", "image.jpg", "thumbnail.jpg"} {
			p := filepath.Join(s.Dir, "media", m.ID, name)
			if e = addDisk(ctx, z, &manifest, "media/"+m.ID+"/"+name, p); e != nil {
				return e
			}
		}
	}
	return closeArchive(z, manifest)
}
func newManifest(kind, version string) Manifest {
	return Manifest{Kind: kind, AppVersion: version, SchemaVersion: store.SchemaVersion, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Files: map[string]FileDigest{}}
}
func addDisk(ctx context.Context, z *zip.Writer, m *Manifest, name, p string) error {
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	return addFile(ctx, z, m, name, f)
}
func addFile(ctx context.Context, z *zip.Writer, m *Manifest, name string, r io.Reader) error {
	h := sha256.New()
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0600)
	header.SetModTime(time.Now().UTC())
	w, e := z.CreateHeader(header)
	if e != nil {
		return e
	}
	n, e := io.Copy(io.MultiWriter(w, h), &contextReader{ctx: ctx, r: r})
	if e != nil {
		return e
	}
	m.Files[name] = FileDigest{hex.EncodeToString(h.Sum(nil)), n}
	return nil
}
func closeArchive(z *zip.Writer, m Manifest) error {
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	h := &zip.FileHeader{Name: "manifest.json", Method: zip.Deflate}
	h.SetMode(0600)
	w, e := z.CreateHeader(h)
	if e != nil {
		return e
	}
	if _, e = w.Write(b); e != nil {
		return e
	}
	return z.Close()
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func Create(ctx context.Context, s *store.Store, w io.Writer, version string) error {
	var pageCount, pageSize int64
	if e := s.DB.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); e != nil {
		return e
	}
	if e := s.DB.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); e != nil {
		return e
	}
	if pageCount > 1<<40/pageSize {
		return fmt.Errorf("database exceeds backup budget")
	}
	if e := store.RequireSpace(s.Dir, pageCount*pageSize*3+(16<<20)); e != nil {
		return e
	}
	f, e := os.CreateTemp(s.Dir, ".snapshot-*.db")
	if e != nil {
		return e
	}
	tmp := f.Name()
	f.Close()
	os.Remove(tmp)
	defer os.Remove(tmp)
	if _, e = s.DB.ExecContext(ctx, "VACUUM INTO ?", tmp); e != nil {
		return fmt.Errorf("consistent snapshot: %w", e)
	}
	os.Chmod(tmp, 0600)
	z := zip.NewWriter(w)
	m := newManifest("instance", version)
	if e = addDisk(ctx, z, &m, "puffolio.db", tmp); e != nil {
		return e
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT id FROM media ORDER BY id")
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if !fields.UUID(id) {
			return fmt.Errorf("invalid media identifier")
		}
		for _, name := range []string{"master.jpg", "image.jpg", "thumbnail.jpg"} {
			if e = addDisk(ctx, z, &m, "media/"+id+"/"+name, filepath.Join(s.Dir, "media", id, name)); e != nil {
				return e
			}
		}
	}
	return closeArchive(z, m)
}

type checkedArchive struct {
	reader   *zip.ReadCloser
	manifest Manifest
	files    map[string]*zip.File
}

func checkArchive(filename string, limits Limits) (a *checkedArchive, err error) {
	if limits.MaxExpandedBytes <= 0 || limits.MaxEntries <= 0 || limits.MaxImages <= 0 {
		return nil, fault.Validation("Positive Importgrenzen erforderlich")
	}
	zr, e := zip.OpenReader(filename)
	if e != nil {
		return nil, fault.Validation("Ungültiges ZIP-Archiv")
	}
	a = &checkedArchive{reader: zr, files: map[string]*zip.File{}}
	defer func() {
		if err != nil {
			zr.Close()
		}
	}()
	if len(zr.File) > limits.MaxEntries {
		return nil, fault.Validation("Archiv überschreitet Eintragsgrenze")
	}
	var expanded uint64
	images := map[string]bool{}
	for _, f := range zr.File {
		n := f.Name
		if n == "" || !fsPath(n) || f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 || f.Mode()&os.ModeType != 0 {
			return nil, fault.Validation("Unsicherer Archivpfad")
		}
		if _, ok := a.files[n]; ok {
			return nil, fault.Validation("Doppelter Archivpfad")
		}
		if f.UncompressedSize64 > uint64(limits.MaxExpandedBytes) || expanded > uint64(limits.MaxExpandedBytes)-f.UncompressedSize64 {
			return nil, fault.New(413, "import_limit", "Archiv überschreitet Grenze für entpackte Bytes")
		}
		expanded += f.UncompressedSize64
		a.files[n] = f
		if strings.HasPrefix(n, "media/") {
			parts := strings.Split(n, "/")
			if len(parts) != 3 || !fields.UUID(parts[1]) || (parts[2] != "master.jpg" && parts[2] != "image.jpg" && parts[2] != "thumbnail.jpg") {
				return nil, fault.Validation("Unbekannter Medienpfad")
			}
			images[parts[1]] = true
		} else if n != "manifest.json" && n != "collection.json" && n != "puffolio.db" {
			return nil, fault.Validation("Unbekannter Archiveintrag")
		}
	}
	if len(images) > limits.MaxImages {
		return nil, fault.Validation("Archiv überschreitet Bildgrenze")
	}
	mf, ok := a.files["manifest.json"]
	if !ok || mf.UncompressedSize64 > 8<<20 {
		return nil, fault.Validation("Manifest fehlt oder ist zu groß")
	}
	r, e := mf.Open()
	if e != nil {
		return nil, e
	}
	dec := json.NewDecoder(io.LimitReader(r, 8<<20))
	dec.DisallowUnknownFields()
	e = dec.Decode(&a.manifest)
	r.Close()
	if e != nil || !store.BackupSchemaSupported(a.manifest.SchemaVersion) || a.manifest.Kind != "collection" && a.manifest.Kind != "instance" {
		return nil, fault.Validation("Unbekannte Archiv- oder Schemaversion")
	}
	if len(a.manifest.Files) != len(a.files)-1 {
		return nil, fault.Validation("Manifest enthält falsche Dateiliste")
	}
	for n, f := range a.files {
		if n == "manifest.json" {
			continue
		}
		digest, ok := a.manifest.Files[n]
		if !ok || digest.Size < 0 || uint64(digest.Size) != f.UncompressedSize64 || len(digest.SHA256) != 64 {
			return nil, fault.Validation("Manifest stimmt nicht überein")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		h := sha256.New()
		count, e := io.Copy(h, io.LimitReader(r, digest.Size+1))
		r.Close()
		if e != nil || count != digest.Size || hex.EncodeToString(h.Sum(nil)) != digest.SHA256 {
			return nil, fault.Validation("Archivprüfsumme ungültig")
		}
	}
	return a, nil
}
func fsPath(n string) bool {
	return !strings.Contains(n, "\\") && !strings.ContainsRune(n, 0) && !strings.HasPrefix(n, "/") && path.Clean(n) == n && n != "." && n != ".." && !strings.HasPrefix(n, "../") && !strings.Contains(n, ":")
}
func (a *checkedArchive) collection() (Collection, error) {
	c, e := a.readCollection(nil)
	if e != nil {
		return c, e
	}
	if e = validateCollection(c, a); e != nil {
		return c, e
	}
	validator := liquidValidator(c)
	_, e = a.readCollection(validator)
	return c, e
}
func Preview(filename string, limits Limits) (ImportPreview, error) {
	a, e := checkArchive(filename, limits)
	if e != nil {
		return ImportPreview{}, e
	}
	defer a.reader.Close()
	c, e := a.collection()
	if e != nil {
		return ImportPreview{}, e
	}
	return ImportPreview{Counts: Counts{c.liquidCount, len(c.Fields), len(c.Media)}, Warnings: []string{}}, nil
}
func Restore(ctx context.Context, filename, dest string) (err error) {
	if info, e := os.Lstat(dest); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore destination must be an empty directory")
		}
		entries, e := os.ReadDir(dest)
		if e != nil {
			return e
		}
		if len(entries) != 0 {
			return fmt.Errorf("restore destination must be empty")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	a, e := checkArchive(filename, Limits{MaxExpandedBytes: 1 << 40, MaxEntries: 1_000_000, MaxImages: 500_000})
	if e != nil {
		return e
	}
	defer a.reader.Close()
	if a.manifest.Kind != "instance" {
		return fmt.Errorf("instance backup required")
	}
	if _, ok := a.files["puffolio.db"]; !ok {
		return fmt.Errorf("database missing")
	}
	if _, ok := a.files["collection.json"]; ok {
		return fmt.Errorf("unexpected collection payload")
	}
	if e = os.MkdirAll(dest, 0700); e != nil {
		return e
	}
	staging, e := os.MkdirTemp(dest, ".restore-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(staging)
	for n, f := range a.files {
		if n == "manifest.json" {
			continue
		}
		if e = extract(ctx, f, filepath.Join(staging, filepath.FromSlash(n))); e != nil {
			return e
		}
	}
	db, e := sql.Open("sqlite", filepath.Join(staging, "puffolio.db")+"?_pragma=foreign_keys(1)")
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if e = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); e != nil || integrity != "ok" {
		return fmt.Errorf("restored database integrity check failed")
	}
	var version int
	// An older database stays as it is here; store.Open migrates it at the next start.
	if e = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); e != nil || version != a.manifest.SchemaVersion {
		return fmt.Errorf("unsupported database schema")
	}
	rows, e := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return fmt.Errorf("restored database foreign keys invalid")
	}
	rows, e = db.QueryContext(ctx, "SELECT id FROM media")
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if len(a.files) != 2+3*len(ids) {
		return fmt.Errorf("unexpected instance archive files")
	}
	for _, id := range ids {
		if !fields.UUID(id) {
			return fmt.Errorf("invalid restored media id")
		}
		for _, name := range []string{"master.jpg", "image.jpg", "thumbnail.jpg"} {
			if _, ok := a.files["media/"+id+"/"+name]; !ok {
				return fmt.Errorf("restored media missing")
			}
		}
	}
	if _, e = db.ExecContext(ctx, "DELETE FROM sessions; DELETE FROM one_time_tokens;"); e != nil {
		return e
	}
	if e = db.Close(); e != nil {
		return e
	}
	entries, e := os.ReadDir(staging)
	if e != nil {
		return e
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	installed := []string{}
	defer func() {
		if err != nil {
			for _, p := range installed {
				os.RemoveAll(p)
			}
		}
	}()
	for _, entry := range entries {
		if e = os.Rename(filepath.Join(staging, entry.Name()), filepath.Join(dest, entry.Name())); e != nil {
			return e
		}
		installed = append(installed, filepath.Join(dest, entry.Name()))
	}
	return os.Chmod(dest, 0700)
}
func extract(ctx context.Context, f *zip.File, dest string) error {
	if e := os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
		return e
	}
	out, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer out.Close()
	r, e := f.Open()
	if e != nil {
		return e
	}
	defer r.Close()
	n, e := io.Copy(out, &contextReader{ctx: ctx, r: io.LimitReader(r, int64(f.UncompressedSize64)+1)})
	if e != nil {
		return e
	}
	if uint64(n) != f.UncompressedSize64 {
		return fmt.Errorf("archive size mismatch")
	}
	return out.Sync()
}

// PreviewForCollection validates the archive for an authenticated destination.
func PreviewForCollection(ctx context.Context, s *store.Store, cid, filename string, limits Limits) (ImportPreview, error) {
	a, e := checkArchive(filename, limits)
	if e != nil {
		return ImportPreview{}, e
	}
	defer a.reader.Close()
	c, e := a.collection()
	if e != nil {
		return ImportPreview{}, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return ImportPreview{}, e
	}
	defer tx.Rollback()
	if _, _, e = checkDestination(ctx, tx, cid, c); e != nil {
		return ImportPreview{}, e
	}
	if e = tx.Commit(); e != nil {
		return ImportPreview{}, e
	}
	return ImportPreview{Counts: Counts{c.liquidCount, len(c.Fields), len(c.Media)}, Warnings: []string{}}, nil
}
