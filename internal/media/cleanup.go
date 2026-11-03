package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cleanup takes the same writer lock as requests and snapshot creation. Images
// referenced by trash remain live until their liquid's 30-day retention ends.
func (s *Service) Cleanup(ctx context.Context, now time.Time) error {
	s.Store.Writer.Lock()
	defer s.Store.Writer.Unlock()
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	cut := now.UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	if _, e = tx.ExecContext(ctx, "DELETE FROM liquids WHERE deleted_at IS NOT NULL AND deleted_at<?", cut); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<? OR last_used<?", now.Unix(), now.Add(-7*24*time.Hour).Unix()); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM one_time_tokens WHERE expires_at<?", now.Unix()); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, "SELECT m.id FROM media m WHERE m.created_at<? AND NOT "+mediaReferencedSQL, now.Add(-24*time.Hour).UTC().Format(time.RFC3339Nano))
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
		if _, e = tx.ExecContext(ctx, "DELETE FROM media WHERE id=?", id); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	for _, id := range ids {
		if validID(id) {
			if e = os.RemoveAll(filepath.Join(s.Store.Dir, "media", id)); e != nil {
				return e
			}
		}
	}
	live := map[string]bool{}
	rows, e = s.Store.DB.QueryContext(ctx, "SELECT id FROM media")
	if e != nil {
		return e
	}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		live[id] = true
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(filepath.Join(s.Store.Dir, "media"))
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if e = ctx.Err(); e != nil {
			return e
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !live[entry.Name()] && info.ModTime().Before(now.Add(-24*time.Hour)) && (validID(entry.Name()) || strings.HasPrefix(entry.Name(), ".staging-") || strings.HasPrefix(entry.Name(), ".import-")) {
			if e = os.RemoveAll(filepath.Join(s.Store.Dir, "media", entry.Name())); e != nil {
				return e
			}
		}
	}
	for _, dir := range []string{s.Store.Dir, filepath.Join(s.Store.Dir, "imports")} {
		entries, e = os.ReadDir(dir)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".preview-") && !strings.HasPrefix(entry.Name(), ".download-") && !strings.HasPrefix(entry.Name(), ".snapshot-") {
				continue
			}
			info, e := entry.Info()
			if e != nil {
				return e
			}
			if info.ModTime().Before(now.Add(-24 * time.Hour)) {
				if e = os.Remove(filepath.Join(dir, entry.Name())); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
