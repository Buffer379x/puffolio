package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"net/http"
	"slices"
	"time"
)

// tagVersion checks that the tag belongs to this tag field and returns its version.
func tagVersion(ctx context.Context, q fields.Querier, cid, fid, tid string) (fields.Definition, int, error) {
	d, e := fields.Get(ctx, q, cid, fid)
	if e != nil {
		return d, 0, e
	}
	if d.Type != "tags" {
		return d, 0, fault.Validation("Kein Tagfeld")
	}
	var version int
	e = q.QueryRowContext(ctx, "SELECT version FROM tags WHERE collection_id=? AND field_id=? AND id=?", cid, fid, tid).Scan(&version)
	if e == sql.ErrNoRows {
		return d, 0, fault.NotFound()
	}
	return d, version, e
}

// defaultWithoutTag returns the field's default tags without this one, if it is among them.
func defaultWithoutTag(d fields.Definition, tid string) ([]string, bool) {
	var ids []string
	if len(d.Config.DefaultValue) == 0 || json.Unmarshal(d.Config.DefaultValue, &ids) != nil || !slices.Contains(ids, tid) {
		return nil, false
	}
	return slices.DeleteFunc(ids, func(id string) bool { return id == tid }), true
}

// requiredWithoutTag counts the liquids the required rule applies to whose only tag in this
// field is this one. Left without a value they could not be saved, not even as a favourite, and
// no collection export holding them could be imported again.
func requiredWithoutTag(ctx context.Context, q fields.Querier, cid string, d fields.Definition, tid string) (int, error) {
	if !d.Required || d.Archived {
		return 0, nil
	}
	rows, e := q.QueryContext(ctx, "SELECT l.created_at FROM liquid_tags lt JOIN liquids l ON l.collection_id=lt.collection_id AND l.id=lt.liquid_id WHERE lt.collection_id=? AND lt.field_id=? AND lt.tag_id=? AND NOT EXISTS(SELECT 1 FROM liquid_tags o WHERE o.collection_id=lt.collection_id AND o.liquid_id=lt.liquid_id AND o.field_id=lt.field_id AND o.tag_id<>lt.tag_id)", cid, d.ID, tid)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var created string
		if e = rows.Scan(&created); e != nil {
			return 0, e
		}
		required, e := fields.RequiredForRecord(d, created)
		if e != nil {
			return 0, e
		}
		if required {
			count++
		}
	}
	return count, rows.Err()
}

// widgetsWithoutTag returns the dashboard without the tag and how many widgets filtered by it.
func widgetsWithoutTag(ctx context.Context, q fields.Querier, uid, fid, tid string) ([]Widget, int, error) {
	var raw string
	var widgets []Widget
	if e := q.QueryRowContext(ctx, "SELECT data FROM dashboards WHERE user_id=?", uid).Scan(&raw); e == sql.ErrNoRows {
		return nil, 0, nil
	} else if e != nil {
		return nil, 0, e
	}
	if e := json.Unmarshal([]byte(raw), &widgets); e != nil {
		return nil, 0, e
	}
	count := 0
	for i := range widgets {
		next, changed := liquids.WithoutTag(widgets[i].Filters, fid, tid)
		if changed {
			widgets[i].Filters = next
			count++
		}
	}
	return widgets, count, nil
}

// viewsWithoutTag returns only the saved views that filtered by the tag, already without it.
func viewsWithoutTag(ctx context.Context, q fields.Querier, uid, fid, tid string) ([]SavedView, error) {
	rows, e := q.QueryContext(ctx, "SELECT data FROM saved_views WHERE user_id=?", uid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []SavedView{}
	for rows.Next() {
		var raw string
		var v SavedView
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(raw), &v); e != nil {
			return nil, e
		}
		if next, changed := liquids.WithoutTag(v.Filters, fid, tid); changed {
			v.Filters = next
			out = append(out, v)
		}
	}
	return out, rows.Err()
}
func (s *server) tagUsage(w http.ResponseWriter, r *http.Request) error {
	ctx, db := r.Context(), s.Store.DB
	cid, uid, fid, tid := current(r).User.CollectionID, current(r).User.ID, r.PathValue("id"), r.PathValue("tagId")
	d, _, e := tagVersion(ctx, db, cid, fid, tid)
	if e != nil {
		return e
	}
	var usage struct {
		Liquids   int  `json:"liquids"`
		Trashed   int  `json:"trashed"`
		IsDefault bool `json:"isDefault"`
		Widgets   int  `json:"widgets"`
		Views     int  `json:"views"`
		Orphaned  int  `json:"orphaned"`
	}
	e = db.QueryRowContext(ctx, "SELECT count(*),count(l.deleted_at) FROM liquid_tags lt JOIN liquids l ON l.collection_id=lt.collection_id AND l.id=lt.liquid_id WHERE lt.collection_id=? AND lt.field_id=? AND lt.tag_id=?", cid, fid, tid).Scan(&usage.Liquids, &usage.Trashed)
	if e != nil {
		return e
	}
	_, usage.IsDefault = defaultWithoutTag(d, tid)
	if usage.Orphaned, e = requiredWithoutTag(ctx, db, cid, d, tid); e != nil {
		return e
	}
	if _, usage.Widgets, e = widgetsWithoutTag(ctx, db, uid, fid, tid); e != nil {
		return e
	}
	views, e := viewsWithoutTag(ctx, db, uid, fid, tid)
	if e != nil {
		return e
	}
	usage.Views = len(views)
	write(w, 200, usage)
	return nil
}

// deleteTag removes a tag everywhere at once. The database drops its assignments; the field's
// default, the widgets and the saved views keep tag ids in JSON and are cleaned here. Left in a
// default the id would stop every new liquid, left in a filter it would silently show nothing.
// A required field applies from now on if that leaves a liquid without its value.
func (s *server) deleteTag(w http.ResponseWriter, r *http.Request) error {
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
	ctx := r.Context()
	cid, uid, fid, tid := current(r).User.CollectionID, current(r).User.ID, r.PathValue("id"), r.PathValue("tagId")
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	d, version, e := tagVersion(ctx, tx, cid, fid, tid)
	if e != nil {
		return e
	}
	if d.Archived {
		return fault.Validation("Kein aktives Tagfeld")
	}
	if version != in.Version {
		return fault.Conflict()
	}
	orphaned, e := requiredWithoutTag(ctx, tx, cid, d, tid)
	if e != nil {
		return e
	}
	// An open form holding this tag now meets the usual conflict. Nobody edited the liquid,
	// so updated_at stays and "last changed" keeps its order.
	if _, e = tx.ExecContext(ctx, "UPDATE liquids SET version=version+1 WHERE collection_id=? AND id IN (SELECT liquid_id FROM liquid_tags WHERE collection_id=? AND field_id=? AND tag_id=?)", cid, cid, fid, tid); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM tags WHERE collection_id=? AND field_id=? AND id=?", cid, fid, tid); e != nil {
		return e
	}
	rest, isDefault := defaultWithoutTag(d, tid)
	if isDefault {
		d.Config.DefaultValue = nil
		if len(rest) > 0 {
			d.Config.DefaultValue, _ = json.Marshal(rest)
		}
	}
	// The cutoff the app already has for "only new liquids": those that lost their tag stay usable.
	if orphaned > 0 {
		d.RequiredFrom = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if isDefault || orphaned > 0 {
		d.Version++
		raw, _ := json.Marshal(d)
		if _, e = tx.ExecContext(ctx, "UPDATE field_definitions SET data=? WHERE collection_id=? AND id=?", string(raw), cid, fid); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE collections SET schema_version=schema_version+1 WHERE id=?", cid); e != nil {
			return e
		}
	}
	widgets, changed, e := widgetsWithoutTag(ctx, tx, uid, fid, tid)
	if e != nil {
		return e
	}
	if changed > 0 {
		raw, _ := json.Marshal(widgets)
		if _, e = tx.ExecContext(ctx, "UPDATE dashboards SET data=?,version=version+1 WHERE user_id=?", string(raw), uid); e != nil {
			return e
		}
	}
	views, e := viewsWithoutTag(ctx, tx, uid, fid, tid)
	if e != nil {
		return e
	}
	for _, v := range views {
		v.Version++
		raw, _ := json.Marshal(v)
		if _, e = tx.ExecContext(ctx, "UPDATE saved_views SET version=?,data=? WHERE id=? AND user_id=?", v.Version, string(raw), v.ID, uid); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	write(w, 200, map[string]bool{"success": true})
	return nil
}
