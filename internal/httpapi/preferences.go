package httpapi

import (
	"database/sql"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/dashboard"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/store"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type Preferences = auth.ApplicationPreferences
type Widget = dashboard.Widget
type Layout struct {
	Version int      `json:"version"`
	Widgets []Widget `json:"widgets"`
}

func (s *server) loadPreferences(r *http.Request) (Preferences, error) {
	var p Preferences
	var raw string
	e := s.Store.DB.QueryRowContext(r.Context(), "SELECT version,data FROM user_preferences WHERE user_id=?", current(r).User.ID).Scan(&p.Version, &raw)
	if e != nil {
		return p, e
	}
	version := p.Version
	e = json.Unmarshal([]byte(raw), &p)
	p.Version = version
	return p, e
}
func (s *server) getPreferences(w http.ResponseWriter, r *http.Request) error {
	p, e := s.loadPreferences(r)
	if e != nil {
		return e
	}
	write(w, 200, p)
	return nil
}
func (s *server) putPreferences(w http.ResponseWriter, r *http.Request) error {
	var p Preferences
	if e := decode(w, r, &p); e != nil {
		return e
	}
	if p.Theme != "light" && p.Theme != "dark" && p.Theme != "system" || p.View != "grid" && p.View != "list" {
		return fault.Validation("Ungültige Darstellung")
	}
	if _, e := time.LoadLocation(p.Timezone); e != nil {
		return fault.Validation("Ungültige Zeitzone")
	}
	defs, e := fields.List(r.Context(), s.Store.DB, current(r).User.CollectionID)
	if e != nil {
		return e
	}
	dm := map[string]fields.Definition{}
	for _, d := range defs {
		dm[d.ID] = d
	}
	for _, ids := range [][]string{p.ListFields, p.GridFields} {
		seen := map[string]bool{}
		if len(ids) > 100 {
			return fault.Validation("Zu viele Felder")
		}
		for _, id := range ids {
			if _, ok := dm[id]; !ok || seen[id] {
				return fault.Validation("Ungültige sichtbare Felder")
			}
			seen[id] = true
		}
	}
	roleTypes := map[string]string{"image": "image", "rating": "rating", "color": "color", "quantity": "number"}
	for role, id := range p.Roles {
		typ, ok := roleTypes[role]
		d, exists := dm[id]
		if !ok || id != "" && (!exists || d.Type != typ || d.Archived) {
			return fault.Validation("Ungültige Feldzuordnung")
		}
	}
	old := p.Version
	p.Version++
	raw, _ := json.Marshal(p)
	res, e := s.Store.DB.ExecContext(r.Context(), "UPDATE user_preferences SET version=?,data=? WHERE user_id=? AND version=?", p.Version, string(raw), current(r).User.ID, old)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fault.Conflict()
	}
	write(w, 200, p)
	return nil
}
func (s *server) getDashboard(w http.ResponseWriter, r *http.Request) error {
	var l Layout
	var raw string
	if e := s.Store.DB.QueryRowContext(r.Context(), "SELECT version,data FROM dashboards WHERE user_id=?", current(r).User.ID).Scan(&l.Version, &raw); e != nil {
		return e
	}
	if e := json.Unmarshal([]byte(raw), &l.Widgets); e != nil {
		return e
	}
	write(w, 200, l)
	return nil
}
func (s *server) putDashboard(w http.ResponseWriter, r *http.Request) error {
	var l Layout
	if e := decode(w, r, &l); e != nil {
		return e
	}
	if e := dashboard.Validate(l.Widgets); e != nil {
		return e
	}
	raw, _ := json.Marshal(l.Widgets)
	res, e := s.Store.DB.ExecContext(r.Context(), "UPDATE dashboards SET data=?,version=version+1 WHERE user_id=? AND version=?", string(raw), current(r).User.ID, l.Version)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fault.Conflict()
	}
	l.Version++
	write(w, 200, l)
	return nil
}
func (s *server) queryDashboard(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Widgets []Widget `json:"widgets"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	items, e := dashboard.Query(r.Context(), s.Store.DB, current(r).User.CollectionID, in.Widgets)
	if e != nil {
		return e
	}
	write(w, 200, map[string]any{"items": items})
	return nil
}

type SavedView struct {
	Favorite  bool            `json:"favorite"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Search    string          `json:"search"`
	Filters   *liquids.Filter `json:"filters,omitempty"`
	Sort      string          `json:"sort"`
	Direction string          `json:"direction"`
	Version   int             `json:"version"`
}

func (s *server) listViews(w http.ResponseWriter, r *http.Request) error {
	rows, e := s.Store.DB.QueryContext(r.Context(), "SELECT data FROM saved_views WHERE user_id=? ORDER BY id", current(r).User.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []SavedView{}
	for rows.Next() {
		var raw string
		var v SavedView
		if e = rows.Scan(&raw); e != nil {
			return e
		}
		if e = json.Unmarshal([]byte(raw), &v); e != nil {
			return e
		}
		items = append(items, v)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	write(w, 200, map[string]any{"items": items})
	return nil
}
func (s *server) createView(w http.ResponseWriter, r *http.Request) error {
	return s.saveView(w, r, true)
}
func (s *server) updateView(w http.ResponseWriter, r *http.Request) error {
	return s.saveView(w, r, false)
}
func (s *server) saveView(w http.ResponseWriter, r *http.Request, create bool) error {
	var v SavedView
	if e := decode(w, r, &v); e != nil {
		return e
	}
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" || utf8.RuneCountInString(v.Name) > 100 {
		return fault.Validation("Name erforderlich")
	}
	q := liquids.Query{Favorite: v.Favorite, Search: v.Search, Sort: v.Sort, Direction: v.Direction, Filters: v.Filters, Page: 1, PageSize: 1}
	if _, e := liquids.List(r.Context(), s.Store.DB, current(r).User.CollectionID, q); e != nil {
		return e
	}
	uid := current(r).User.ID
	if create {
		var n int
		s.Store.DB.QueryRowContext(r.Context(), "SELECT count(*) FROM saved_views WHERE user_id=?", uid).Scan(&n)
		if n >= 100 {
			return fault.Validation("Maximal 100 Ansichten")
		}
		v.ID = store.ID()
		v.Version = 1
		raw, _ := json.Marshal(v)
		if _, e := s.Store.DB.ExecContext(r.Context(), "INSERT INTO saved_views(id,user_id,version,data) VALUES(?,?,?,?)", v.ID, uid, v.Version, string(raw)); e != nil {
			return e
		}
		write(w, 201, v)
		return nil
	}
	v.ID = r.PathValue("id")
	var old int
	if e := s.Store.DB.QueryRowContext(r.Context(), "SELECT version FROM saved_views WHERE id=? AND user_id=?", v.ID, uid).Scan(&old); e == sql.ErrNoRows {
		return fault.NotFound()
	} else if e != nil {
		return e
	}
	if old != v.Version {
		return fault.Conflict()
	}
	v.Version++
	raw, _ := json.Marshal(v)
	if _, e := s.Store.DB.ExecContext(r.Context(), "UPDATE saved_views SET version=?,data=? WHERE id=? AND user_id=?", v.Version, string(raw), v.ID, uid); e != nil {
		return e
	}
	write(w, 200, v)
	return nil
}
func (s *server) deleteView(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Version int `json:"version"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	var version int
	if e := s.Store.DB.QueryRowContext(r.Context(), "SELECT version FROM saved_views WHERE id=? AND user_id=?", r.PathValue("id"), current(r).User.ID).Scan(&version); e == sql.ErrNoRows {
		return fault.NotFound()
	} else if e != nil {
		return e
	}
	if in.Version != version {
		return fault.Conflict()
	}
	if _, e := s.Store.DB.ExecContext(r.Context(), "DELETE FROM saved_views WHERE id=? AND user_id=?", r.PathValue("id"), current(r).User.ID); e != nil {
		return e
	}
	write(w, 200, map[string]bool{"success": true})
	return nil
}
