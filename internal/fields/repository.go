package fields

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/store"
	"sort"
	"time"
)

type Querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func List(ctx context.Context, q Querier, cid string) ([]Definition, error) {
	rows, e := q.QueryContext(ctx, "SELECT data FROM field_definitions WHERE collection_id=?", cid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Definition{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var d Definition
		if e = json.Unmarshal([]byte(raw), &d); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position == out[j].Position {
			return out[i].ID < out[j].ID
		}
		return out[i].Position < out[j].Position
	})
	return out, rows.Err()
}
func Get(ctx context.Context, q Querier, cid, id string) (Definition, error) {
	var raw string
	e := q.QueryRowContext(ctx, "SELECT data FROM field_definitions WHERE collection_id=? AND id=?", cid, id).Scan(&raw)
	if e == sql.ErrNoRows {
		return Definition{}, fault.NotFound()
	}
	if e != nil {
		return Definition{}, e
	}
	var d Definition
	e = json.Unmarshal([]byte(raw), &d)
	return d, e
}
func Save(ctx context.Context, db *sql.DB, cid string, d Definition, create bool) (Definition, error) {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return d, e
	}
	defer tx.Rollback()
	d, e = SaveTx(ctx, tx, cid, d, create)
	if e != nil {
		return d, e
	}
	return d, tx.Commit()
}
func SaveTx(ctx context.Context, tx *sql.Tx, cid string, d Definition, create bool) (Definition, error) {
	var e error
	if d.Type == "rating" && d.Config.Stars == 0 {
		d.Config.Stars = 5
	}
	for i := range d.Config.Options {
		if d.Config.Options[i].ID == "" || create && !UUID(d.Config.Options[i].ID) {
			d.Config.Options[i].ID = store.ID()
		}
	}
	if e := ValidateDefinition(d); e != nil {
		return d, fault.Validation(e.Error())
	}
	if create {
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM field_definitions WHERE collection_id=?", cid).Scan(&n); e != nil {
			return d, e
		}
		if n >= 100 {
			return d, fault.Validation("Maximal 100 Felder")
		}
		d.ID = store.ID()
		d.Version = 1
		d.Position = n
		if d.Required {
			d.RequiredFrom = time.Now().UTC().Format(time.RFC3339Nano)
		}
	} else {
		old, e := Get(ctx, tx, cid, d.ID)
		if e != nil {
			return d, e
		}
		if old.Version != d.Version {
			return d, fault.Conflict()
		}
		d.RequiredFrom = old.RequiredFrom
		if d.Required && !old.Required {
			d.RequiredFrom = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if !d.Required {
			d.RequiredFrom = ""
		}
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM field_values WHERE collection_id=? AND field_id=?)+(SELECT count(*) FROM liquid_tags WHERE collection_id=? AND field_id=?)", cid, d.ID, cid, d.ID).Scan(&n); e != nil {
			return d, e
		}
		if n > 0 && (old.Type != d.Type || old.Type == "number" && old.Config.Scale != d.Config.Scale) {
			return d, fault.Validation("Typ und Zahlenskala bei vorhandenen Werten unveränderlich")
		}
		if n > 0 {
			rows, e := tx.QueryContext(ctx, "SELECT value_text,value_number,value_bool,value_media,value_option FROM field_values WHERE collection_id=? AND field_id=?", cid, d.ID)
			if e != nil {
				return d, e
			}
			for rows.Next() {
				var text, media, option sql.NullString
				var number sql.NullInt64
				var boolean sql.NullBool
				if e = rows.Scan(&text, &number, &boolean, &media, &option); e != nil {
					rows.Close()
					return d, e
				}
				var raw json.RawMessage
				if text.Valid {
					raw, _ = json.Marshal(text.String)
				}
				if number.Valid {
					if d.Type == "number" {
						raw, _ = json.Marshal(FormatNumber(number.Int64, old.Config.Scale))
					} else {
						raw, _ = json.Marshal(number.Int64)
					}
				}
				if boolean.Valid {
					raw, _ = json.Marshal(boolean.Bool)
				}
				if media.Valid {
					raw, _ = json.Marshal(media.String)
				}
				if option.Valid {
					found := false
					for _, o := range d.Config.Options {
						if o.ID == option.String {
							found = true
						}
					}
					if !found {
						rows.Close()
						return d, fault.Validation("Verwendete Optionen nur archivieren")
					}
					continue
				}
				if _, e = ValidateValue(d, raw); e != nil {
					rows.Close()
					return d, fault.Validation("Bestehende Werte würden ungültig")
				}
			}
			if e = rows.Close(); e != nil {
				return d, e
			}
		}
		d.Version++
	}
	if len(d.Config.DefaultValue) > 0 && string(d.Config.DefaultValue) != "null" {
		v, err := ValidateValue(d, d.Config.DefaultValue)
		if err != nil {
			return d, fault.Validation(err.Error())
		}
		if v.Media != nil {
			var n int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM media WHERE collection_id=? AND id=?", cid, *v.Media).Scan(&n); err != nil {
				return d, err
			}
			if n != 1 {
				return d, fault.Validation("Standardbild gehört nicht zur Sammlung")
			}
		}
		for _, tid := range v.Tags {
			var n int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM tags WHERE collection_id=? AND field_id=? AND id=?", cid, d.ID, tid).Scan(&n); err != nil {
				return d, err
			}
			if n != 1 {
				return d, fault.Validation("Standardtag gehört nicht zu diesem Feld")
			}
		}
	}
	raw, _ := json.Marshal(d)
	if create {
		_, e = tx.ExecContext(ctx, "INSERT INTO field_definitions(id,collection_id,type,data) VALUES(?,?,?,?)", d.ID, cid, d.Type, string(raw))
	} else {
		_, e = tx.ExecContext(ctx, "UPDATE field_definitions SET type=?,data=? WHERE collection_id=? AND id=?", d.Type, string(raw), cid, d.ID)
	}
	if e != nil {
		return d, e
	}
	if !create {
		rows, e := tx.QueryContext(ctx, "SELECT id FROM field_options WHERE collection_id=? AND field_id=?", cid, d.ID)
		if e != nil {
			return d, e
		}
		remove := []string{}
		for rows.Next() {
			var id string
			rows.Scan(&id)
			found := false
			for _, o := range d.Config.Options {
				if o.ID == id {
					found = true
				}
			}
			if !found {
				remove = append(remove, id)
			}
		}
		rows.Close()
		for _, id := range remove {
			if _, e = tx.ExecContext(ctx, "DELETE FROM field_options WHERE collection_id=? AND field_id=? AND id=?", cid, d.ID, id); e != nil {
				return d, e
			}
		}
	}
	for _, o := range d.Config.Options {
		if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO field_options(collection_id,field_id,id) VALUES(?,?,?)", cid, d.ID, o.ID); e != nil {
			return d, e
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE collections SET schema_version=schema_version+1 WHERE id=?", cid); e != nil {
		return d, e
	}
	return d, nil
}
func Tags(ctx context.Context, q Querier, cid, fid string) ([]Tag, error) {
	d, e := Get(ctx, q, cid, fid)
	if e != nil {
		return nil, e
	}
	if d.Type != "tags" {
		return nil, fault.Validation("Kein Tagfeld")
	}
	rows, e := q.QueryContext(ctx, "SELECT id,name,palette_id,version FROM tags WHERE collection_id=? AND field_id=? ORDER BY normalized_name,id", cid, fid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		t := Tag{FieldID: fid}
		if e = rows.Scan(&t.ID, &t.Name, &t.PaletteID, &t.Version); e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func Starter(ctx context.Context, db *sql.DB, cid string) (map[string]string, error) {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	roles, e := StarterTx(ctx, tx, cid)
	if e != nil {
		return nil, e
	}
	return roles, tx.Commit()
}
func StarterTx(ctx context.Context, tx *sql.Tx, cid string) (map[string]string, error) {
	roles := map[string]string{}
	defs := []Definition{{Label: "Hersteller", Type: "text", Config: Config{Searchable: true, Filterable: true}}, {Label: "Bewertung", Type: "rating", Config: Config{Stars: 5, Filterable: true}}, {Label: "Bild", Type: "image"}, {Label: "Tags", Type: "tags", Config: Config{Searchable: true, Filterable: true}}, {Label: "Farbe", Type: "color"}, {Label: "Menge", Type: "number", Config: Config{Scale: 1, Unit: "ml", Min: "0", Max: "1000000", Filterable: true}}, {Label: "Nikotin", Type: "number", Config: Config{Scale: 1, Unit: "mg/ml", Min: "0", Max: "100", Filterable: true}}, {Label: "Status", Type: "select", Config: Config{Options: []Option{{ID: store.ID(), Label: "In Gebrauch"}, {ID: store.ID(), Label: "Vorrat"}, {ID: store.ID(), Label: "Aufgebraucht"}}, Filterable: true}}, {Label: "Kaufdatum", Type: "date", Config: Config{Filterable: true}}, {Label: "Notizen", Type: "longtext", Config: Config{Searchable: true}}}
	for _, d := range defs {
		d.Visibility = Visibility{Form: true, Detail: true, List: d.Type != "longtext", Grid: d.Type == "rating" || d.Type == "image" || d.Type == "tags" || d.Type == "color"}
		saved, e := SaveTx(ctx, tx, cid, d, true)
		if e != nil {
			return nil, fmt.Errorf("starter fields: %w", e)
		}
		switch d.Label {
		case "Bild":
			roles["image"] = saved.ID
		case "Bewertung":
			roles["rating"] = saved.ID
		case "Farbe":
			roles["color"] = saved.ID
		case "Menge":
			roles["quantity"] = saved.ID
		}
	}
	return roles, nil
}

// CreateWithBackfill creates a required field and populates legacy scalar values
// in the same transaction; omitted raw selects the explicit new-record-only policy.
func CreateWithBackfill(ctx context.Context, db *sql.DB, cid string, d Definition, raw json.RawMessage) (Definition, error) {
	if len(raw) == 0 {
		return Save(ctx, db, cid, d, true)
	}
	if !d.Required {
		return d, fault.Validation("Auffüllwert nur für neue Pflichtfelder")
	}
	switch d.Type {
	case "text", "longtext", "rating", "color", "date", "number", "select", "boolean":
	default:
		return d, fault.Validation("Dieser Typ unterstützt nur Pflicht ab Neuanlage")
	}
	if d.Type == "rating" && d.Config.Stars == 0 {
		d.Config.Stars = 5
	}
	for i := range d.Config.Options {
		old := d.Config.Options[i].ID
		if !UUID(old) {
			d.Config.Options[i].ID = store.ID()
			var selected string
			if d.Type == "select" && json.Unmarshal(raw, &selected) == nil && selected == old {
				raw, _ = json.Marshal(d.Config.Options[i].ID)
			}
		}
	}
	if e := ValidateDefinition(d); e != nil {
		return d, fault.Validation(e.Error())
	}
	v, e := ValidateValue(d, raw)
	if e != nil {
		return d, fault.Validation(e.Error())
	}
	if len(v.Raw) == 0 {
		return d, fault.Validation("Auffüllwert darf nicht leer sein")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return d, e
	}
	defer tx.Rollback()
	d, e = SaveTx(ctx, tx, cid, d, true)
	if e != nil {
		return d, e
	}
	d.RequiredFrom = ""
	data, _ := json.Marshal(d)
	if _, e = tx.ExecContext(ctx, "UPDATE field_definitions SET data=? WHERE collection_id=? AND id=?", string(data), cid, d.ID); e != nil {
		return d, e
	}
	var search any
	if v.Text != nil {
		search = Normalize(*v.Text)
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_text,value_search,value_number,value_bool,value_media,value_option) SELECT ?,id,?,?,?,?,?,?,?,? FROM liquids WHERE collection_id=?", cid, d.ID, d.Type, v.Text, search, v.Number, v.Boolean, v.Media, v.Option, cid); e != nil {
		return d, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE liquids SET version=version+1,updated_at=? WHERE collection_id=?", time.Now().UTC().Format(time.RFC3339Nano), cid); e != nil {
		return d, e
	}
	return d, tx.Commit()
}
