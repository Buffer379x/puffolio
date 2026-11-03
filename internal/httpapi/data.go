package httpapi

import (
	"bytes"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/store"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type preview struct {
	Path, CollectionID string
	Expires            time.Time
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
func readLimited(r io.Reader, max int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > max {
		return nil, fault.New(413, "upload_too_large", "Datei zu groß")
	}
	return b, nil
}
func (s *server) download(w http.ResponseWriter, r *http.Request, name string, create func(io.Writer) error) error {
	f, e := os.CreateTemp(s.Store.Dir, ".download-*.zip")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if e = create(f); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if _, e = f.Seek(0, 0); e != nil {
		return e
	}
	info, e := f.Stat()
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, info.ModTime(), f)
	return nil
}
func (s *server) export(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDs json.RawMessage `json:"ids,omitempty"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	var ids []string
	if len(in.IDs) > 0 {
		if e := json.Unmarshal(in.IDs, &ids); e != nil || ids == nil {
			return fault.Validation("Liquid-Auswahl muss eine Liste sein")
		}
	}
	return s.download(w, r, "puffolio-collection.zip", func(out io.Writer) error {
		if len(in.IDs) > 0 {
			return backup.ExportSelected(r.Context(), s.Store, current(r).User.CollectionID, out, s.Version, ids)
		}
		return backup.Export(r.Context(), s.Store, current(r).User.CollectionID, out, s.Version)
	})
}
func (s *server) backup(w http.ResponseWriter, r *http.Request) error {
	if e := requireAdmin(r); e != nil {
		return e
	}
	var in struct {
		Password string `json:"password"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	u := current(r).User
	if !s.rateAllowed(r, u.Username) {
		return fault.New(429, "rate_limited", "Bitte später erneut versuchen")
	}
	if _, e := s.auth.Login(r.Context(), u.Username, in.Password); e != nil {
		return fault.New(403, "password_invalid", "Passwort ungültig")
	}
	if e := s.audit(r, "backup.created"); e != nil {
		return e
	}
	return s.download(w, r, "puffolio-instance.zip", func(out io.Writer) error { return backup.Create(r.Context(), s.Store, out, s.Version) })
}
func (s *server) importPreview(w http.ResponseWriter, r *http.Request) error {
	const max = 100 << 20
	r.Body = http.MaxBytesReader(w, r.Body, max+(1<<20))
	mr, e := r.MultipartReader()
	if e != nil {
		return fault.Validation("ZIP-Datei erforderlich")
	}
	dir := filepath.Join(s.Store.Dir, "imports")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".preview-*.zip")
	if e != nil {
		return e
	}
	keep := false
	defer func() {
		f.Close()
		if !keep {
			os.Remove(f.Name())
		}
	}()
	found := false
	for {
		part, e := mr.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			return fault.Validation("Upload unvollständig")
		}
		if part.FormName() != "file" || found {
			part.Close()
			return fault.Validation("Genau eine ZIP-Datei erforderlich")
		}
		found = true
		n, e := io.Copy(f, io.LimitReader(part, max+1))
		part.Close()
		if e != nil {
			return fault.Validation("Upload fehlgeschlagen")
		}
		if n > max {
			return fault.New(413, "import_limit", "Webimport maximal 100 MiB; größere Exporte über das CLI mit expliziten Grenzen importieren")
		}
	}
	if !found {
		return fault.Validation("ZIP-Datei erforderlich")
	}
	if e = f.Sync(); e != nil {
		return e
	}
	p, e := backup.PreviewForCollection(r.Context(), s.Store, current(r).User.CollectionID, f.Name(), backup.WebLimits())
	if e != nil {
		return e
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	now := time.Now()
	active := 0
	for key, v := range s.previews {
		if v.Expires.Before(now) {
			os.Remove(v.Path)
			delete(s.previews, key)
		} else if v.CollectionID == current(r).User.CollectionID {
			active++
		}
	}
	if active >= 5 || len(s.previews) >= 100 {
		return fault.New(429, "preview_limit", "Zu viele offene Importvorschauen")
	}
	token := store.Token(32)
	s.previews[auth.Hash(token)] = preview{Path: f.Name(), CollectionID: current(r).User.CollectionID, Expires: now.Add(time.Hour)}
	keep = true
	write(w, 200, map[string]any{"token": token, "counts": p.Counts, "warnings": p.Warnings})
	return nil
}
func (s *server) importCommit(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token   string `json:"token"`
		Confirm bool   `json:"confirm"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !in.Confirm {
		return fault.Validation("Import muss bestätigt werden")
	}
	key := auth.Hash(in.Token)
	s.previewMu.Lock()
	p, ok := s.previews[key]
	if ok && p.CollectionID == current(r).User.CollectionID {
		delete(s.previews, key)
	}
	s.previewMu.Unlock()
	if !ok || p.CollectionID != current(r).User.CollectionID || p.Expires.Before(time.Now()) {
		return fault.Validation("Importvorschau ungültig oder abgelaufen")
	}
	defer os.Remove(p.Path)
	counts, e := backup.Import(r.Context(), s.Store, p.CollectionID, p.Path, backup.WebLimits())
	if e != nil {
		return e
	}
	write(w, 200, counts)
	return nil
}
