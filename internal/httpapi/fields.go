package httpapi

import (
	"database/sql"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (s *server) listFields(w http.ResponseWriter, r *http.Request) error {
	cid := current(r).User.CollectionID
	items, e := fields.List(r.Context(), s.Store.DB, cid)
	if e != nil {
		return e
	}
	var version int
	if e = s.Store.DB.QueryRowContext(r.Context(), "SELECT schema_version FROM collections WHERE id=?", cid).Scan(&version); e != nil {
		return e
	}
	write(w, 200, map[string]any{"items": items, "version": version})
	return nil
}
func (s *server) createField(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		fields.Definition
		BackfillValue json.RawMessage `json:"backfillValue,omitempty"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	d, e := fields.CreateWithBackfill(r.Context(), s.Store.DB, current(r).User.CollectionID, in.Definition, in.BackfillValue)
	if e != nil {
		return e
	}
	if e = s.showInViews(r, d.ID, d.Visibility.List, d.Visibility.Grid); e != nil {
		return e
	}
	write(w, 201, d)
	return nil
}
func (s *server) updateField(w http.ResponseWriter, r *http.Request) error {
	cid := current(r).User.CollectionID
	old, e := fields.Get(r.Context(), s.Store.DB, cid, r.PathValue("id"))
	if e != nil {
		return e
	}
	var patch map[string]json.RawMessage
	if e = decode(w, r, &patch); e != nil {
		return e
	}
	if _, ok := patch["version"]; !ok {
		return fault.Validation("Version erforderlich")
	}
	raw, _ := json.Marshal(old)
	var merged map[string]json.RawMessage
	json.Unmarshal(raw, &merged)
	for k, v := range patch {
		if _, ok := merged[k]; !ok && k != "requiredFrom" {
			return fault.Validation("Unbekannte Feldeigenschaft")
		}
		merged[k] = v
	}
	b, _ := json.Marshal(merged)
	var in fields.Definition
	if json.Unmarshal(b, &in) != nil || in.ID != old.ID {
		return fault.Validation("Ungültiges Feld")
	}
	d, e := fields.Save(r.Context(), s.Store.DB, cid, in, false)
	if e != nil {
		return e
	}
	if e = s.showInViews(r, d.ID, d.Visibility.List && !old.Visibility.List, d.Visibility.Grid && !old.Visibility.Grid); e != nil {
		return e
	}
	write(w, 200, d)
	return nil
}

// showInViews adds a field the owner just made visible to their column choices.
// "Ansicht" can still hide it again; it only never appears by omission.
func (s *server) showInViews(r *http.Request, id string, list, grid bool) error {
	if !list && !grid {
		return nil
	}
	p, e := s.loadPreferences(r)
	if e != nil {
		return e
	}
	add := func(ids []string) []string {
		for _, existing := range ids {
			if existing == id {
				return ids
			}
		}
		return append(ids, id)
	}
	if list {
		p.ListFields = add(p.ListFields)
	}
	if grid {
		p.GridFields = add(p.GridFields)
	}
	version := p.Version
	p.Version++
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	_, e = s.Store.DB.ExecContext(r.Context(), "UPDATE user_preferences SET version=?,data=? WHERE user_id=? AND version=?", p.Version, string(raw), current(r).User.ID, version)
	return e
}
func (s *server) archiveField(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Version  int  `json:"version"`
		Archived bool `json:"archived"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	cid := current(r).User.CollectionID
	d, e := fields.Get(r.Context(), s.Store.DB, cid, r.PathValue("id"))
	if e != nil {
		return e
	}
	if d.Version != in.Version {
		return fault.Conflict()
	}
	d.Archived = in.Archived
	d, e = fields.Save(r.Context(), s.Store.DB, cid, d, false)
	if e != nil {
		return e
	}
	write(w, 200, d)
	return nil
}
func (s *server) deleteField(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Version int  `json:"version"`
		Confirm bool `json:"confirm"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !in.Confirm {
		return fault.Validation("Löschen muss bestätigt werden")
	}
	cid := current(r).User.CollectionID
	d, e := fields.Get(r.Context(), s.Store.DB, cid, r.PathValue("id"))
	if e != nil {
		return e
	}
	if d.Version != in.Version {
		return fault.Conflict()
	}
	tx, e := s.Store.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM field_definitions WHERE collection_id=? AND id=?", cid, d.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE collections SET schema_version=schema_version+1 WHERE id=?", cid); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	write(w, 200, map[string]bool{"success": true})
	return nil
}
func (s *server) orderFields(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDs     []string `json:"ids"`
		Version int      `json:"version"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	cid := current(r).User.CollectionID
	tx, e := s.Store.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var version int
	if e = tx.QueryRowContext(r.Context(), "SELECT schema_version FROM collections WHERE id=?", cid).Scan(&version); e != nil {
		return e
	}
	if version != in.Version {
		return fault.Conflict()
	}
	defs, e := fields.List(r.Context(), tx, cid)
	if e != nil {
		return e
	}
	if len(defs) != len(in.IDs) {
		return fault.Validation("Alle Felder genau einmal erforderlich")
	}
	dm := map[string]fields.Definition{}
	for _, d := range defs {
		dm[d.ID] = d
	}
	for i, id := range in.IDs {
		d, ok := dm[id]
		if !ok {
			return fault.Validation("Ungültige Feldreihenfolge")
		}
		delete(dm, id)
		d.Position = i
		d.Version++
		b, _ := json.Marshal(d)
		if _, e = tx.ExecContext(r.Context(), "UPDATE field_definitions SET data=? WHERE collection_id=? AND id=?", string(b), cid, id); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE collections SET schema_version=schema_version+1 WHERE id=?", cid); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	return s.listFields(w, r)
}
func (s *server) listTags(w http.ResponseWriter, r *http.Request) error {
	tags, e := fields.Tags(r.Context(), s.Store.DB, current(r).User.CollectionID, r.PathValue("id"))
	if e != nil {
		return e
	}
	write(w, 200, map[string]any{"items": tags})
	return nil
}
func (s *server) saveTag(w http.ResponseWriter, r *http.Request, create bool) error {
	cid, fid := current(r).User.CollectionID, r.PathValue("id")
	d, e := fields.Get(r.Context(), s.Store.DB, cid, fid)
	if e != nil {
		return e
	}
	if d.Type != "tags" || d.Archived {
		return fault.Validation("Kein aktives Tagfeld")
	}
	var in struct {
		Name      string `json:"name"`
		PaletteID string `json:"paletteId"`
		Version   int    `json:"version,omitempty"`
	}
	if e = decode(w, r, &in); e != nil {
		return e
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 || !fields.ValidPalette(in.PaletteID) {
		return fault.Validation("Tagname und gültige Palette erforderlich")
	}
	tag := fields.Tag{ID: store.ID(), FieldID: fid, Name: in.Name, PaletteID: in.PaletteID, Version: 1}
	if create {
		_, e = s.Store.DB.ExecContext(r.Context(), "INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id) VALUES(?,?,?,?,?,?)", tag.ID, cid, fid, tag.Name, fields.Normalize(tag.Name), tag.PaletteID)
	} else {
		tag.ID = r.PathValue("tagId")
		var version int
		if e = s.Store.DB.QueryRowContext(r.Context(), "SELECT version FROM tags WHERE collection_id=? AND field_id=? AND id=?", cid, fid, tag.ID).Scan(&version); e == sql.ErrNoRows {
			return fault.NotFound()
		}
		if e != nil {
			return e
		}
		if version != in.Version {
			return fault.Conflict()
		}
		tag.Version = version + 1
		_, e = s.Store.DB.ExecContext(r.Context(), "UPDATE tags SET name=?,normalized_name=?,palette_id=?,version=? WHERE collection_id=? AND field_id=? AND id=? AND version=?", tag.Name, fields.Normalize(tag.Name), tag.PaletteID, tag.Version, cid, fid, tag.ID, version)
	}
	if e != nil {
		if strings.Contains(e.Error(), "UNIQUE") {
			return fault.New(409, "duplicate_tag", "Tagname bereits vorhanden")
		}
		return e
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, tag)
	return nil
}
func (s *server) createTag(w http.ResponseWriter, r *http.Request) error {
	return s.saveTag(w, r, true)
}
func (s *server) updateTag(w http.ResponseWriter, r *http.Request) error {
	return s.saveTag(w, r, false)
}
func (s *server) fieldUsage(w http.ResponseWriter, r *http.Request) error {
	cid, fid := current(r).User.CollectionID, r.PathValue("id")
	if _, e := fields.Get(r.Context(), s.Store.DB, cid, fid); e != nil {
		return e
	}
	var count int
	if e := s.Store.DB.QueryRowContext(r.Context(), "SELECT count(*) FROM (SELECT liquid_id FROM field_values WHERE collection_id=? AND field_id=? UNION SELECT liquid_id FROM liquid_tags WHERE collection_id=? AND field_id=?)", cid, fid, cid, fid).Scan(&count); e != nil {
		return e
	}
	write(w, 200, map[string]int{"values": count})
	return nil
}
