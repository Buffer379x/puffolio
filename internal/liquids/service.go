package liquids

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"strings"
	"time"
	"unicode/utf8"
)

type Liquid struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Favorite  bool                    `json:"favorite"`
	Version   int                     `json:"version"`
	CreatedAt string                  `json:"createdAt"`
	UpdatedAt string                  `json:"updatedAt"`
	DeletedAt *string                 `json:"deletedAt,omitempty"`
	Values    map[string]fields.Value `json:"values"`
	// ShelfLife is derived on read, never stored or exported.
	ShelfLife *ShelfLife `json:"shelfLife,omitempty"`
}
type Input struct {
	Name     *string                  `json:"name"`
	Favorite *bool                    `json:"favorite"`
	Version  int                      `json:"version"`
	Values   map[string]*fields.Value `json:"values"`
}

func Get(ctx context.Context, q fields.Querier, cid, id string) (Liquid, error) {
	l := Liquid{Values: map[string]fields.Value{}}
	e := q.QueryRowContext(ctx, "SELECT id,name,favorite,version,created_at,updated_at,deleted_at FROM liquids WHERE collection_id=? AND id=?", cid, id).Scan(&l.ID, &l.Name, &l.Favorite, &l.Version, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt)
	if e == sql.ErrNoRows {
		return l, fault.NotFound()
	}
	if e != nil {
		return l, e
	}
	rows, e := q.QueryContext(ctx, "SELECT v.field_id,v.type,v.value_text,v.value_number,v.value_bool,v.value_media,v.value_option,f.data FROM field_values v JOIN field_definitions f ON f.collection_id=v.collection_id AND f.id=v.field_id WHERE v.collection_id=? AND liquid_id=?", cid, id)
	if e != nil {
		return l, e
	}
	for rows.Next() {
		var fid, typ, def string
		var txt, media, option sql.NullString
		var num sql.NullInt64
		var boolean sql.NullBool
		if e = rows.Scan(&fid, &typ, &txt, &num, &boolean, &media, &option, &def); e != nil {
			rows.Close()
			return l, e
		}
		var value any
		if txt.Valid {
			value = txt.String
		}
		if num.Valid {
			value = num.Int64
			if typ == "number" {
				var d fields.Definition
				json.Unmarshal([]byte(def), &d)
				value = fields.FormatNumber(num.Int64, d.Config.Scale)
			}
		}
		if boolean.Valid {
			value = boolean.Bool
		}
		if media.Valid {
			value = media.String
		}
		if option.Valid {
			value = option.String
		}
		raw, _ := json.Marshal(value)
		l.Values[fid] = fields.Value{Type: typ, Value: raw}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return l, e
	}
	rows, e = q.QueryContext(ctx, "SELECT field_id,tag_id FROM liquid_tags WHERE collection_id=? AND liquid_id=? ORDER BY field_id,tag_id", cid, id)
	if e != nil {
		return l, e
	}
	tagmap := map[string][]string{}
	for rows.Next() {
		var fid, tid string
		if e = rows.Scan(&fid, &tid); e != nil {
			rows.Close()
			return l, e
		}
		tagmap[fid] = append(tagmap[fid], tid)
	}
	e = rows.Err()
	rows.Close()
	for fid, ids := range tagmap {
		raw, _ := json.Marshal(ids)
		l.Values[fid] = fields.Value{Type: "tags", Value: raw}
	}
	return l, e
}
func Save(ctx context.Context, db *sql.DB, cid, id string, in Input, location *time.Location) (Liquid, error) {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Liquid{}, e
	}
	defer tx.Rollback()
	create := id == ""
	now := time.Now().UTC().Format(time.RFC3339Nano)
	l := Liquid{ID: id, Version: 1, CreatedAt: now, UpdatedAt: now, Values: map[string]fields.Value{}}
	if create {
		l.ID = store.ID()
	} else {
		l, e = Get(ctx, tx, cid, id)
		if e != nil {
			return l, e
		}
		if l.Version != in.Version {
			return l, fault.Conflict()
		}
		if l.DeletedAt != nil {
			return l, fault.Validation("Liquid liegt im Papierkorb")
		}
		l.Version++
		l.UpdatedAt = now
	}
	if in.Name != nil {
		l.Name = strings.TrimSpace(*in.Name)
	}
	if l.Name == "" || utf8.RuneCountInString(l.Name) > 200 {
		return l, fault.Field("name", fmt.Errorf("Name erforderlich (maximal 200 Zeichen)"))
	}
	if in.Favorite != nil {
		l.Favorite = *in.Favorite
	}
	defs, e := fields.List(ctx, tx, cid)
	if e != nil {
		return l, e
	}
	defmap := map[string]fields.Definition{}
	for _, d := range defs {
		defmap[d.ID] = d
	}
	if in.Values == nil {
		in.Values = map[string]*fields.Value{}
	}
	if create {
		for _, d := range defs {
			if d.Archived {
				continue
			}
			if _, ok := in.Values[d.ID]; ok {
				continue
			}
			raw := d.Config.DefaultValue
			if d.Config.DefaultToday {
				if location == nil {
					location = time.UTC
				}
				raw, _ = json.Marshal(time.Now().In(location).Format("2006-01-02"))
			}
			if len(raw) > 0 && string(raw) != "null" {
				v := fields.Value{Type: d.Type, Value: raw}
				in.Values[d.ID] = &v
			}
		}
	}
	normalized := map[string]fields.Validated{}
	for fid, value := range in.Values {
		d, ok := defmap[fid]
		if !ok {
			return l, fault.Field(fid, fmt.Errorf("Unbekanntes Feld"))
		}
		old, exists := l.Values[fid]
		if value != nil && exists && value.Type == old.Type && string(value.Value) == string(old.Value) {
			continue
		}
		if d.Archived {
			return l, fault.Field(fid, fmt.Errorf("Feld archiviert"))
		}
		if value == nil || string(value.Value) == "null" {
			normalized[fid] = fields.Validated{}
			delete(l.Values, fid)
			continue
		}
		if value.Type != d.Type {
			return l, fault.Field(fid, fmt.Errorf("Falscher Feldtyp"))
		}
		validated, e := fields.ValidateValue(d, value.Value)
		if e != nil {
			return l, fault.Field(fid, e)
		}
		normalized[fid] = validated
		if len(validated.Raw) == 0 {
			delete(l.Values, fid)
		} else {
			l.Values[fid] = fields.Value{Type: d.Type, Value: validated.Raw}
		}
	}
	for _, d := range defs {
		_, set := l.Values[d.ID]
		required, err := fields.RequiredForRecord(d, l.CreatedAt)
		if err != nil {
			return l, err
		}
		if create && d.Required && !d.Archived {
			required = true
		}
		if required && !set {
			return l, fault.Field(d.ID, fmt.Errorf("Pflichtfeld"))
		}
	}
	if create {
		_, e = tx.ExecContext(ctx, "INSERT INTO liquids(id,collection_id,name,normalized_name,favorite,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", l.ID, cid, l.Name, fields.Normalize(l.Name), l.Favorite, l.Version, l.CreatedAt, l.UpdatedAt)
	} else {
		_, e = tx.ExecContext(ctx, "UPDATE liquids SET name=?,normalized_name=?,favorite=?,version=?,updated_at=? WHERE collection_id=? AND id=?", l.Name, fields.Normalize(l.Name), l.Favorite, l.Version, l.UpdatedAt, cid, l.ID)
	}
	if e != nil {
		return l, e
	}
	for fid, v := range normalized {
		if _, e = tx.ExecContext(ctx, "DELETE FROM field_values WHERE collection_id=? AND liquid_id=? AND field_id=?", cid, l.ID, fid); e != nil {
			return l, e
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM liquid_tags WHERE collection_id=? AND liquid_id=? AND field_id=?", cid, l.ID, fid); e != nil {
			return l, e
		}
		if len(v.Raw) == 0 {
			continue
		}
		if defmap[fid].Type == "tags" {
			for _, tid := range v.Tags {
				var n int
				if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM tags WHERE collection_id=? AND field_id=? AND id=?", cid, fid, tid).Scan(&n); e != nil {
					return l, e
				}
				if n != 1 {
					return l, fault.Field(fid, fmt.Errorf("Unbekannter Tag"))
				}
				if _, e = tx.ExecContext(ctx, "INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES(?,?,?,?)", cid, l.ID, fid, tid); e != nil {
					return l, e
				}
			}
			continue
		}
		if v.Media != nil {
			var n int
			if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM media WHERE collection_id=? AND id=?", cid, *v.Media).Scan(&n); e != nil {
				return l, e
			}
			if n != 1 {
				return l, fault.Field(fid, fmt.Errorf("Unbekanntes Bild"))
			}
		}
		var search any
		if v.Text != nil {
			search = fields.Normalize(*v.Text)
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_text,value_search,value_number,value_bool,value_media,value_option) VALUES(?,?,?,?,?,?,?,?,?,?)", cid, l.ID, fid, defmap[fid].Type, v.Text, search, v.Number, v.Boolean, v.Media, v.Option)
		if e != nil {
			return l, e
		}
	}
	return l, tx.Commit()
}
func Trash(ctx context.Context, db *sql.DB, cid, id string, version int, restore bool) (Liquid, error) {
	l, e := Get(ctx, db, cid, id)
	if e != nil {
		return l, e
	}
	if l.Version != version {
		return l, fault.Conflict()
	}
	var deleted any
	if !restore {
		deleted = time.Now().UTC().Format(time.RFC3339Nano)
	}
	r, e := db.ExecContext(ctx, "UPDATE liquids SET deleted_at=?,version=version+1,updated_at=? WHERE collection_id=? AND id=? AND version=?", deleted, time.Now().UTC().Format(time.RFC3339Nano), cid, id, version)
	if e != nil {
		return l, e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return l, fault.Conflict()
	}
	return Get(ctx, db, cid, id)
}

// Permanent removes only an already-trashed liquid. Media are immutable shared
// references and are left for the reference-aware 24-hour cleanup pass.
func Permanent(ctx context.Context, db *sql.DB, cid, id string, version int) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var current int
	var deleted sql.NullString
	e = tx.QueryRowContext(ctx, "SELECT version,deleted_at FROM liquids WHERE collection_id=? AND id=?", cid, id).Scan(&current, &deleted)
	if e == sql.ErrNoRows {
		return fault.NotFound()
	}
	if e != nil {
		return e
	}
	if current != version {
		return fault.Conflict()
	}
	if !deleted.Valid {
		return fault.New(409, "trash_required", "Liquid muss bereits im Papierkorb liegen")
	}
	res, e := tx.ExecContext(ctx, "DELETE FROM liquids WHERE collection_id=? AND id=? AND version=? AND deleted_at IS NOT NULL", cid, id, version)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fault.Conflict()
	}
	return tx.Commit()
}
