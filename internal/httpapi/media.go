package httpapi

import (
	"errors"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/media"
	"net/http"
	"os"
)

func uploadFile(w http.ResponseWriter, r *http.Request, max int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, max+(1<<20))
	if e := r.ParseMultipartForm(1 << 20); e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			return nil, fault.New(413, "upload_too_large", "Datei zu groß")
		}
		return nil, fault.Validation("Dateiupload ungültig")
	}
	defer r.MultipartForm.RemoveAll()
	f, _, e := r.FormFile("file")
	if e != nil {
		return nil, fault.Validation("Datei erforderlich")
	}
	defer f.Close()
	return readLimited(f, max)
}
func (s *server) uploadMedia(w http.ResponseWriter, r *http.Request) error {
	b, e := uploadFile(w, r, media.MaxBytes)
	if e != nil {
		return e
	}
	m, e := media.New(s.Store).Upload(r.Context(), current(r).User.CollectionID, bytesReader(b))
	if e != nil {
		return e
	}
	write(w, 201, m)
	return nil
}
func (s *server) mediaInfo(w http.ResponseWriter, r *http.Request) error {
	m, e := media.New(s.Store).Info(r.Context(), current(r).User.CollectionID, r.PathValue("id"))
	if e != nil {
		return e
	}
	write(w, 200, m)
	return nil
}
func (s *server) getMedia(w http.ResponseWriter, r *http.Request) error {
	svc := media.New(s.Store)
	m, e := svc.Info(r.Context(), current(r).User.CollectionID, r.PathValue("id"))
	if e != nil {
		return e
	}
	variant := r.URL.Query().Get("variant")
	if variant != "" && variant != "master" && variant != "thumbnail" {
		return fault.Validation("Ungültige Bildvariante")
	}
	f, e := os.Open(svc.Path(m.ID, variant))
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Cookie")
	http.ServeContent(w, r, m.ID+".jpg", info.ModTime(), f)
	return nil
}
func (s *server) cropMedia(w http.ResponseWriter, r *http.Request) error {
	var in media.Crop
	if e := decode(w, r, &in); e != nil {
		return e
	}
	m, e := media.New(s.Store).Crop(r.Context(), current(r).User.CollectionID, r.PathValue("id"), in)
	if e != nil {
		return e
	}
	write(w, 201, m)
	return nil
}
func (s *server) deleteMedia(w http.ResponseWriter, r *http.Request) error {
	if e := media.New(s.Store).Delete(r.Context(), current(r).User.CollectionID, r.PathValue("id")); e != nil {
		return e
	}
	write(w, 200, map[string]bool{"success": true})
	return nil
}
