package httpapi

import (
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"net/http"
	"strconv"
	"time"
)

func (s *server) listLiquids(w http.ResponseWriter, r *http.Request) error {
	q, e := parseQuery(r)
	if e != nil {
		return e
	}
	if q.Trash && current(r).Visitor {
		return fault.New(401, "unauthorized", "Anmeldung erforderlich")
	}
	if _, q.Shelf, e = s.shelfRules(r); e != nil {
		return e
	}
	p, e := liquids.List(r.Context(), s.Store.DB, current(r).User.CollectionID, q)
	if e != nil {
		return e
	}
	write(w, 200, p)
	return nil
}
func parseQuery(r *http.Request) (liquids.Query, error) {
	v := r.URL.Query()
	q := liquids.Query{Search: v.Get("search"), Sort: v.Get("sort"), Direction: v.Get("direction"), Page: 1, PageSize: 24}
	for key, dst := range map[string]*int{"page": &q.Page, "pageSize": &q.PageSize} {
		if v.Get(key) != "" {
			n, e := strconv.Atoi(v.Get(key))
			if e != nil {
				return q, fault.Validation("Ungültige Pagination")
			}
			*dst = n
		}
	}
	for key, dst := range map[string]*bool{"favorite": &q.Favorite, "trash": &q.Trash} {
		if v.Get(key) != "" {
			b, e := strconv.ParseBool(v.Get(key))
			if e != nil {
				return q, fault.Validation("Ungültiger Filter")
			}
			*dst = b
		}
	}
	if raw := v.Get("filters"); raw != "" {
		if len(raw) > 32000 {
			return q, fault.Validation("Filter zu groß")
		}
		var f liquids.Filter
		if json.Unmarshal([]byte(raw), &f) != nil {
			return q, fault.Validation("Ungültiger Filter")
		}
		q.Filters = &f
	}
	return q, nil
}
func (s *server) getLiquid(w http.ResponseWriter, r *http.Request) error {
	l, e := liquids.Get(r.Context(), s.Store.DB, current(r).User.CollectionID, r.PathValue("id"))
	if e != nil {
		return e
	}
	if l.DeletedAt != nil && current(r).Visitor {
		return fault.NotFound()
	}
	if e = s.withShelfLife(r, &l); e != nil {
		return e
	}
	write(w, 200, l)
	return nil
}
func (s *server) withShelfLife(r *http.Request, l *liquids.Liquid) error {
	_, rules, e := s.shelfRules(r)
	if e == nil {
		l.ShelfLife = rules.Of(*l)
	}
	return e
}
func (s *server) createLiquid(w http.ResponseWriter, r *http.Request) error {
	return s.saveLiquid(w, r, true)
}
func (s *server) updateLiquid(w http.ResponseWriter, r *http.Request) error {
	return s.saveLiquid(w, r, false)
}
func (s *server) saveLiquid(w http.ResponseWriter, r *http.Request, create bool) error {
	var in liquids.Input
	if e := decode(w, r, &in); e != nil {
		return e
	}
	id := r.PathValue("id")
	if create {
		id = ""
	}
	p, e := s.loadPreferences(r)
	if e != nil {
		return e
	}
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		return e
	}
	l, e := liquids.Save(r.Context(), s.Store.DB, current(r).User.CollectionID, id, in, loc)
	if e != nil {
		return e
	}
	if e = s.withShelfLife(r, &l); e != nil {
		return e
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, l)
	return nil
}
func (s *server) trashLiquid(w http.ResponseWriter, r *http.Request) error {
	if raw := r.URL.Query().Get("permanent"); raw != "" {
		permanent, e := strconv.ParseBool(raw)
		if e != nil {
			return fault.Validation("Ungültiger Löschmodus")
		}
		if permanent {
			var in struct {
				Version int  `json:"version"`
				Confirm bool `json:"confirm"`
			}
			if e = decode(w, r, &in); e != nil {
				return e
			}
			if !in.Confirm {
				return fault.Validation("Endgültiges Löschen muss bestätigt werden")
			}
			if e = liquids.Permanent(r.Context(), s.Store.DB, current(r).User.CollectionID, r.PathValue("id"), in.Version); e != nil {
				return e
			}
			write(w, 200, map[string]bool{"success": true})
			return nil
		}
	}
	return s.changeTrash(w, r, false)
}
func (s *server) restoreLiquid(w http.ResponseWriter, r *http.Request) error {
	return s.changeTrash(w, r, true)
}
func (s *server) changeTrash(w http.ResponseWriter, r *http.Request, restore bool) error {
	var in struct {
		Version int `json:"version"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	l, e := liquids.Trash(r.Context(), s.Store.DB, current(r).User.CollectionID, r.PathValue("id"), in.Version, restore)
	if e != nil {
		return e
	}
	write(w, 200, l)
	return nil
}
func (s *server) bulkLiquids(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDs     []string `json:"ids"`
		Action  string   `json:"action"`
		FieldID string   `json:"fieldId,omitempty"`
		TagIDs  []string `json:"tagIds,omitempty"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if len(in.IDs) < 1 || len(in.IDs) > 100 || in.Action != "trash" && in.Action != "restore" && in.Action != "tag" {
		return fault.Validation("Ungültige Stapelaktion")
	}
	cid := current(r).User.CollectionID
	tx, e := s.Store.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	seen := map[string]bool{}
	for _, id := range in.IDs {
		if seen[id] {
			return fault.Validation("Doppelte Liquid-ID")
		}
		seen[id] = true
		var n int
		if e = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM liquids WHERE collection_id=? AND id=?", cid, id).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return fault.NotFound()
		}
	}
	if in.Action == "tag" {
		d, e := fields.Get(r.Context(), tx, cid, in.FieldID)
		if e != nil {
			return e
		}
		if d.Type != "tags" || d.Archived || len(in.TagIDs) < 1 || len(in.TagIDs) > 200 {
			return fault.Validation("Ungültige Tags")
		}
		seenTags := map[string]bool{}
		for _, tid := range in.TagIDs {
			if seenTags[tid] {
				return fault.Validation("Doppelter Tag")
			}
			seenTags[tid] = true
			var n int
			if e = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM tags WHERE collection_id=? AND field_id=? AND id=?", cid, d.ID, tid).Scan(&n); e != nil {
				return e
			}
			if n != 1 {
				return fault.Validation("Tag fehlt")
			}
		}
		// Validate the final distinct value for every target before any writes.
		for _, id := range in.IDs {
			union := make(map[string]bool, len(seenTags))
			for tid := range seenTags {
				union[tid] = true
			}
			rows, err := tx.QueryContext(r.Context(), "SELECT tag_id FROM liquid_tags WHERE collection_id=? AND liquid_id=? AND field_id=?", cid, id, d.ID)
			if err != nil {
				return err
			}
			for rows.Next() {
				var tid string
				if err = rows.Scan(&tid); err != nil {
					rows.Close()
					return err
				}
				union[tid] = true
				if len(union) > 200 {
					rows.Close()
					return fault.Validation("Ein Liquid darf je Tagfeld höchstens 200 Tags haben")
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		for _, id := range in.IDs {
			for _, tid := range in.TagIDs {
				if _, e = tx.ExecContext(r.Context(), "INSERT OR IGNORE INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES(?,?,?,?)", cid, id, d.ID, tid); e != nil {
					return e
				}
			}
			if _, e = tx.ExecContext(r.Context(), "UPDATE liquids SET version=version+1,updated_at=? WHERE collection_id=? AND id=?", time.Now().UTC().Format(time.RFC3339Nano), cid, id); e != nil {
				return e
			}
		}
	} else {
		var deleted any
		if in.Action == "trash" {
			deleted = time.Now().UTC().Format(time.RFC3339Nano)
		}
		for _, id := range in.IDs {
			if _, e = tx.ExecContext(r.Context(), "UPDATE liquids SET deleted_at=?,version=version+1,updated_at=? WHERE collection_id=? AND id=?", deleted, time.Now().UTC().Format(time.RFC3339Nano), cid, id); e != nil {
				return e
			}
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	write(w, 200, map[string]any{"success": true, "count": len(in.IDs)})
	return nil
}
