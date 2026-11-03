package httpapi

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

type staticAsset struct {
	identity, gzip []byte
	contentType    string
}

type staticResponseWriter struct{ http.ResponseWriter }

func (w staticResponseWriter) WriteHeader(status int) {
	// ServeContent's early failed-precondition branch writes an empty 412
	// without clearing the selected asset's length/encoding, unlike its 304/416.
	if status == http.StatusPreconditionFailed {
		w.Header().Del("Content-Encoding")
		w.Header().Set("Content-Length", "0")
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w staticResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Only existing bundled public scripts/styles are cached. The finite map and
// owned byte slices are fully initialized before publishing the handler, then
// read-only: request paths cannot grow it or trigger repeated compression.
func loadStaticAssets(web fs.FS) map[string]staticAsset {
	assets := make(map[string]staticAsset)
	if web == nil {
		return assets
	}
	fs.WalkDir(web, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(path.Ext(name))
		if ext != ".js" && ext != ".mjs" && ext != ".css" {
			return nil
		}
		b, err := fs.ReadFile(web, name)
		if err != nil {
			return nil // Preserve original read/error behavior instead of caching a failure.
		}
		var compressed bytes.Buffer
		z, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
		if err != nil {
			return nil
		}
		_, writeErr := z.Write(b)
		closeErr := z.Close()
		if writeErr != nil || closeErr != nil {
			return nil
		}
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = http.DetectContentType(b)
		}
		assets[name] = staticAsset{identity: b, gzip: compressed.Bytes(), contentType: contentType}
		return nil
	})
	return assets
}

func varyAcceptEncoding(header http.Header) {
	for _, line := range header.Values("Vary") {
		for _, token := range strings.Split(line, ",") {
			token = strings.TrimSpace(token)
			if token == "*" || strings.EqualFold(token, "Accept-Encoding") {
				return
			}
		}
	}
	header.Add("Vary", "Accept-Encoding")
}

// Prefer accepted gzip unless identity has an explicitly higher weight. Identity
// remains the default fallback; an explicit coding refusal overrides wildcard.
func staticEncoding(lines []string) (useGzip, acceptable bool) {
	value := strings.Join(lines, ",")
	if strings.TrimSpace(value) == "" {
		return false, true
	}
	gzipQ, identityQ, wildcardQ := -1, -1, -1
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, ";")
		var target *int
		switch strings.ToLower(strings.TrimSpace(parts[0])) {
		case "gzip":
			target = &gzipQ
		case "identity":
			target = &identityQ
		case "*":
			target = &wildcardQ
		default:
			continue
		}
		q := 1000
		if len(parts) > 1 {
			key, raw, found := strings.Cut(parts[1], "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "q") || len(parts) != 2 {
				q = 0
			} else {
				q = staticQuality(strings.TrimSpace(raw))
			}
		}
		// Ambiguous duplicate entries cannot override a zero refusal.
		if *target < 0 || q < *target {
			*target = q
		}
	}
	if gzipQ < 0 {
		gzipQ = wildcardQ
	}
	identity := identityQ
	if identity < 0 {
		identity = 1000
		if wildcardQ == 0 {
			identity = 0
		}
	}
	if gzipQ > 0 && (identityQ < 0 || gzipQ >= identity) {
		return true, true
	}
	return false, identity > 0
}

// RFC qvalue, represented in thousandths; invalid input is conservative refusal.
func staticQuality(raw string) int {
	whole, fraction, _ := strings.Cut(raw, ".")
	if (whole != "0" && whole != "1") || len(fraction) > 3 {
		return 0
	}
	q, multiplier := 0, 100
	for _, digit := range fraction {
		if digit < '0' || digit > '9' || (whole == "1" && digit != '0') {
			return 0
		}
		q += int(digit-'0') * multiplier
		multiplier /= 10
	}
	if whole == "1" {
		return 1000
	}
	return q
}
