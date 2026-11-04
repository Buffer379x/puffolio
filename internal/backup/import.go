package backup

import (
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/dashboard"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"github.com/Buffer379x/puffolio/internal/media"
	"github.com/Buffer379x/puffolio/internal/store"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func validateCollection(c Collection, a *checkedArchive) error {
	ids := map[string]bool{}
	unique := func(id string) bool {
		if !fields.UUID(id) || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	dm := map[string]fields.Definition{}
	for _, d := range c.Fields {
		if !unique(d.ID) {
			return fault.Validation("Ungültige oder doppelte Feld-ID")
		}
		if e := fields.ValidateDefinition(d); e != nil {
			return fault.Validation(e.Error())
		}
		if d.RequiredFrom != "" {
			if _, e := time.Parse(time.RFC3339Nano, d.RequiredFrom); e != nil {
				return fault.Validation("Ungültige Pflichtfeldregel")
			}
		}
		for _, o := range d.Config.Options {
			if !unique(o.ID) {
				return fault.Validation("Doppelte Options-ID")
			}
		}
		dm[d.ID] = d
	}
	tags := map[string]fields.Tag{}
	names := map[string]bool{}
	for _, t := range c.Tags {
		d, ok := dm[t.FieldID]
		name := fields.Normalize(t.Name)
		key := t.FieldID + ":" + name
		if !ok || d.Type != "tags" || !unique(t.ID) || !fields.ValidPalette(t.PaletteID) || name == "" || utf8.RuneCountInString(t.Name) > 100 || names[key] {
			return fault.Validation("Ungültiger Tag oder unbekannte Palette")
		}
		names[key] = true
		tags[t.ID] = t
	}
	mm := map[string]media.Media{}
	for _, m := range c.Media {
		if !unique(m.ID) || m.Width < 1 || m.Height < 1 || m.Width > 1600 || m.Height > 1600 {
			return fault.Validation("Ungültige Mediendaten")
		}
		if e := media.ValidateStoredCrop(m.Crop, m.Width, m.Height); e != nil {
			return e
		}
		for _, name := range []string{"master.jpg", "image.jpg", "thumbnail.jpg"} {
			f, ok := a.files["media/"+m.ID+"/"+name]
			if !ok || f.UncompressedSize64 > media.MaxBytes {
				return fault.Validation("Mediendatei fehlt oder zu groß")
			}
			r, e := f.Open()
			if e != nil {
				return e
			}
			cfg, format, e := image.DecodeConfig(r)
			r.Close()
			if e != nil || format != "jpeg" || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > media.MaxPixels {
				return fault.Validation("Ungültiges Archivbild")
			}
			if name == "master.jpg" && (cfg.Width != m.Width || cfg.Height != m.Height) {
				return fault.Validation("Mastermaße stimmen nicht überein")
			}
			// Square since 0.2.0; exports from 0.1.0 carry 4:3 images.
			if name != "master.jpg" && cfg.Width != cfg.Height && cfg.Width*3 != cfg.Height*4 {
				return fault.Validation("Bild hat falsches Seitenverhältnis")
			}
			if cfg.Width > 1600 || cfg.Height > 1600 || name == "thumbnail.jpg" && (cfg.Width > 480 || cfg.Height > 480) {
				return fault.Validation("Archivbild überschreitet bereinigte Bildmaße")
			}
			r, e = f.Open()
			if e != nil {
				return e
			}
			_, decodedFormat, decodeErr := image.Decode(r)
			r.Close()
			if decodeErr != nil || decodedFormat != "jpeg" {
				return fault.Validation("Archivbild ist unvollständig oder beschädigt")
			}
		}
		mm[m.ID] = m
	}
	if len(a.files) != 2+len(c.Media)*3 {
		return fault.Validation("Unzugeordnete Archivdateien")
	}
	for _, d := range c.Fields {
		if len(d.Config.DefaultValue) == 0 || string(d.Config.DefaultValue) == "null" {
			continue
		}
		v, e := fields.ValidateValue(d, d.Config.DefaultValue)
		if e != nil {
			return fault.Validation(e.Error())
		}
		if v.Media != nil {
			if _, ok := mm[*v.Media]; !ok {
				return fault.Validation("Standardbild fehlt im Archiv")
			}
		}
		for _, tid := range v.Tags {
			t, ok := tags[tid]
			if !ok || t.FieldID != d.ID {
				return fault.Validation("Ungültige Standardtagreferenz")
			}
		}
	}
	var prefs struct {
		Theme, Timezone, View  string
		Images                 bool
		ListFields, GridFields []string
		Roles                  map[string]string
	}
	if json.Unmarshal(c.Preferences, &prefs) != nil || prefs.Theme != "system" && prefs.Theme != "light" && prefs.Theme != "dark" || prefs.View != "grid" && prefs.View != "list" {
		return fault.Validation("Ungültige importierte Einstellungen")
	}
	if _, e := time.LoadLocation(prefs.Timezone); e != nil {
		return fault.Validation("Ungültige importierte Zeitzone")
	}
	var widgets []dashboard.Widget
	if e := json.Unmarshal(c.Dashboard, &widgets); e != nil {
		return fault.Validation("Ungültiges Dashboard")
	}
	if e := dashboard.Validate(widgets); e != nil {
		return e
	}
	if len(c.Views) > 100 {
		return fault.Validation("Zu viele gespeicherte Ansichten")
	}
	for _, raw := range c.Views {
		var v struct {
			ID, Name                string
			Version                 int
			Search, Sort, Direction string
			Filters                 *liquids.Filter
		}
		if json.Unmarshal(raw, &v) != nil || !fields.UUID(v.ID) || strings.TrimSpace(v.Name) == "" || utf8.RuneCountInString(v.Name) > 100 {
			return fault.Validation("Ungültige gespeicherte Ansicht")
		}
		if v.Filters != nil {
			if e := liquids.ValidateFilterShape(*v.Filters); e != nil {
				return e
			}
		}
	}
	return nil
}
func Import(ctx context.Context, s *store.Store, cid, filename string, limits Limits) (counts Counts, err error) {
	a, e := checkArchive(filename, limits)
	if e != nil {
		return counts, e
	}
	defer a.reader.Close()
	var expanded int64
	for _, f := range a.manifest.Files {
		if f.Size > (1<<62)/4-expanded {
			return counts, fault.Validation("Import zu groß")
		}
		expanded += f.Size
	}
	if e = store.RequireSpace(s.Dir, expanded*4+(16<<20)); e != nil {
		return counts, fault.Validation(e.Error())
	}
	c, e := a.collection()
	if e != nil {
		return counts, e
	}
	counts = Counts{c.liquidCount, len(c.Fields), len(c.Media)}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return counts, e
	}
	defer tx.Rollback()
	uid, n, e := checkDestination(ctx, tx, cid, c)
	if e != nil {
		return counts, e
	}
	mapping := map[string]string{}
	remap := func(id string) string {
		if id == "" {
			return ""
		}
		if v, ok := mapping[id]; ok {
			return v
		}
		v := store.ID()
		mapping[id] = v
		return v
	}
	dm := map[string]fields.Definition{}
	for _, d := range c.Fields {
		dm[d.ID] = d
		remap(d.ID)
		for _, o := range d.Config.Options {
			remap(o.ID)
		}
	}
	for _, t := range c.Tags {
		remap(t.ID)
	}
	for _, m := range c.Media {
		remap(m.ID)
	}
	staged, e := os.MkdirTemp(filepath.Join(s.Dir, "media"), ".import-")
	if e != nil {
		return counts, e
	}
	defer os.RemoveAll(staged)
	installed := []string{}
	defer func() {
		if err != nil {
			for _, p := range installed {
				os.RemoveAll(p)
			}
		}
	}()
	for _, m := range c.Media {
		old := m.ID
		m.ID = remap(old)
		m.URL = "/api/v1/media/" + m.ID
		m.MasterURL = m.URL + "?variant=master"
		m.ThumbnailURL = m.URL + "?variant=thumbnail"
		for _, name := range []string{"master.jpg", "image.jpg", "thumbnail.jpg"} {
			if e = extract(ctx, a.files["media/"+old+"/"+name], filepath.Join(staged, m.ID, name)); e != nil {
				return counts, e
			}
		}
		raw, _ := json.Marshal(m)
		if _, e = tx.ExecContext(ctx, "INSERT INTO media(id,collection_id,data,created_at) VALUES(?,?,?,?)", m.ID, cid, string(raw), time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
			return counts, e
		}
	}
	for i, original := range c.Fields {
		d := original
		d.Config.Options = append([]fields.Option(nil), original.Config.Options...)
		d.ID = remap(d.ID)
		d.Version = 1
		d.Position = n + i
		for j := range d.Config.Options {
			d.Config.Options[j].ID = remap(d.Config.Options[j].ID)
		}
		if len(d.Config.DefaultValue) > 0 {
			d.Config.DefaultValue = remapValue(original, d.Config.DefaultValue, remap)
		}
		raw, _ := json.Marshal(d)
		if _, e = tx.ExecContext(ctx, "INSERT INTO field_definitions(id,collection_id,type,data) VALUES(?,?,?,?)", d.ID, cid, d.Type, string(raw)); e != nil {
			return counts, e
		}
		for _, o := range d.Config.Options {
			if _, e = tx.ExecContext(ctx, "INSERT INTO field_options(collection_id,field_id,id) VALUES(?,?,?)", cid, d.ID, o.ID); e != nil {
				return counts, e
			}
		}
	}
	for _, tag := range c.Tags {
		if _, e = tx.ExecContext(ctx, "INSERT INTO tags(id,collection_id,field_id,name,normalized_name,palette_id,version) VALUES(?,?,?,?,?,?,1)", remap(tag.ID), cid, remap(tag.FieldID), tag.Name, fields.Normalize(tag.Name), tag.PaletteID); e != nil {
			return counts, e
		}
	}
	e = a.eachLiquid(func(l liquids.Liquid) error {
		lid := remap(l.ID)
		if _, e = tx.ExecContext(ctx, "INSERT INTO liquids(id,collection_id,name,normalized_name,favorite,version,created_at,updated_at,deleted_at) VALUES(?,?,?,?,?,1,?,?,?)", lid, cid, l.Name, fields.Normalize(l.Name), l.Favorite, l.CreatedAt, l.UpdatedAt, l.DeletedAt); e != nil {
			return e
		}
		for fid, value := range l.Values {
			d := dm[fid]
			d.Config.Options = append([]fields.Option(nil), d.Config.Options...)
			for i := range d.Config.Options {
				d.Config.Options[i].Archived = false
			}
			v, e := fields.ValidateValue(d, value.Value)
			if e != nil {
				return e
			}
			if value.Type == "tags" {
				for _, tid := range v.Tags {
					if _, e = tx.ExecContext(ctx, "INSERT INTO liquid_tags(collection_id,liquid_id,field_id,tag_id) VALUES(?,?,?,?)", cid, lid, remap(fid), remap(tid)); e != nil {
						return e
					}
				}
				continue
			}
			if v.Media != nil {
				id := remap(*v.Media)
				v.Media = &id
			}
			if v.Option != nil {
				id := remap(*v.Option)
				v.Option = &id
			}
			var search any
			if v.Text != nil {
				search = fields.Normalize(*v.Text)
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO field_values(collection_id,liquid_id,field_id,type,value_text,value_search,value_number,value_bool,value_media,value_option) VALUES(?,?,?,?,?,?,?,?,?,?)", cid, lid, remap(fid), d.Type, v.Text, search, v.Number, v.Boolean, v.Media, v.Option); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return counts, e
	}
	prefs, e := remapPreferences(c.Preferences, remap)
	if e != nil {
		return counts, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO user_preferences(user_id,data) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET data=excluded.data,version=user_preferences.version+1", uid, string(prefs)); e != nil {
		return counts, e
	}
	widgets, e := remapWidgets(c.Dashboard, dm, remap)
	if e != nil {
		return counts, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO dashboards(user_id,data) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET data=excluded.data,version=dashboards.version+1", uid, string(widgets)); e != nil {
		return counts, e
	}
	for _, raw := range c.Views {
		view, e := remapView(raw, dm, remap)
		if e != nil {
			return counts, e
		}
		var obj map[string]json.RawMessage
		json.Unmarshal(view, &obj)
		var id string
		json.Unmarshal(obj["id"], &id)
		if _, e = tx.ExecContext(ctx, "INSERT INTO saved_views(id,user_id,version,data) VALUES(?,?,1,?)", id, uid, string(view)); e != nil {
			return counts, e
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE collections SET schema_version=schema_version+1 WHERE id=?", cid); e != nil {
		return counts, e
	}
	for _, m := range c.Media {
		id := remap(m.ID)
		dest := filepath.Join(s.Dir, "media", id)
		if e = os.Rename(filepath.Join(staged, id), dest); e != nil {
			return counts, e
		}
		installed = append(installed, dest)
	}
	err = tx.Commit()
	return counts, err
}
func remapValue(d fields.Definition, raw json.RawMessage, remap func(string) string) json.RawMessage {
	switch d.Type {
	case "image", "select":
		var id string
		if json.Unmarshal(raw, &id) == nil && id != "" {
			b, _ := json.Marshal(remap(id))
			return b
		}
	case "tags":
		var ids []string
		if json.Unmarshal(raw, &ids) == nil {
			for i, id := range ids {
				ids[i] = remap(id)
			}
			b, _ := json.Marshal(ids)
			return b
		}
	}
	return raw
}
func remapFilter(f *liquids.Filter, dm map[string]fields.Definition, remap func(string) string) {
	if f == nil {
		return
	}
	for i := range f.Conditions {
		remapFilter(&f.Conditions[i], dm, remap)
	}
	if f.FieldID == "favorite" {
		return
	}
	if f.FieldID != "" {
		d := dm[f.FieldID]
		if d.Type == "select" && f.Operator == "any" {
			var ids []string
			if json.Unmarshal(f.Value, &ids) == nil {
				for i, id := range ids {
					ids[i] = remap(id)
				}
				f.Value, _ = json.Marshal(ids)
			}
		} else {
			f.Value = remapValue(d, f.Value, remap)
		}
		f.Value2 = remapValue(d, f.Value2, remap)
		f.FieldID = remap(f.FieldID)
	}
}
func remapWidgets(raw json.RawMessage, dm map[string]fields.Definition, remap func(string) string) (json.RawMessage, error) {
	var widgets []dashboard.Widget
	if e := json.Unmarshal(raw, &widgets); e != nil {
		return nil, e
	}
	for i := range widgets {
		widgets[i].ID = remap(widgets[i].ID)
		widgets[i].FieldID = remap(widgets[i].FieldID)
		remapFilter(widgets[i].Filters, dm, remap)
	}
	return json.Marshal(widgets)
}
func remapPreferences(raw json.RawMessage, remap func(string) string) (json.RawMessage, error) {
	var p map[string]json.RawMessage
	if e := json.Unmarshal(raw, &p); e != nil {
		return nil, e
	}
	for _, key := range []string{"listFields", "gridFields"} {
		var ids []string
		json.Unmarshal(p[key], &ids)
		for i, id := range ids {
			ids[i] = remap(id)
		}
		p[key], _ = json.Marshal(ids)
	}
	var roles map[string]string
	json.Unmarshal(p["roles"], &roles)
	for k, id := range roles {
		roles[k] = remap(id)
	}
	p["roles"], _ = json.Marshal(roles)
	var consumed string
	if json.Unmarshal(p["consumedOption"], &consumed) == nil && consumed != "" {
		p["consumedOption"], _ = json.Marshal(remap(consumed))
	}
	p["version"] = json.RawMessage("1")
	return json.Marshal(p)
}
func remapView(raw json.RawMessage, dm map[string]fields.Definition, remap func(string) string) (json.RawMessage, error) {
	var v struct {
		Favorite  bool            `json:"favorite"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Search    string          `json:"search"`
		Filters   *liquids.Filter `json:"filters,omitempty"`
		Sort      string          `json:"sort"`
		Direction string          `json:"direction"`
		Version   int             `json:"version"`
	}
	if e := json.Unmarshal(raw, &v); e != nil {
		return nil, e
	}
	v.ID = remap(v.ID)
	v.Version = 1
	remapFilter(v.Filters, dm, remap)
	if strings.HasPrefix(v.Sort, "field:") {
		v.Sort = "field:" + remap(strings.TrimPrefix(v.Sort, "field:"))
	}
	return json.Marshal(v)
}

func liquidValidator(c Collection) func(liquids.Liquid) error {
	dm := map[string]fields.Definition{}
	tags := map[string]fields.Tag{}
	mm := map[string]media.Media{}
	ids := map[string]bool{}
	for _, d := range c.Fields {
		dm[d.ID] = d
		ids[d.ID] = true
		for _, o := range d.Config.Options {
			ids[o.ID] = true
		}
	}
	for _, t := range c.Tags {
		tags[t.ID] = t
		ids[t.ID] = true
	}
	for _, m := range c.Media {
		mm[m.ID] = m
		ids[m.ID] = true
	}
	unique := func(id string) bool {
		if !fields.UUID(id) || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	return func(l liquids.Liquid) error {
		if !unique(l.ID) || strings.TrimSpace(l.Name) == "" || utf8.RuneCountInString(l.Name) > 200 {
			return fault.Validation("Ungültiges Liquid")
		}
		for _, date := range []string{l.CreatedAt, l.UpdatedAt} {
			if _, e := time.Parse(time.RFC3339Nano, date); e != nil {
				return fault.Validation("Ungültiger Zeitstempel")
			}
		}
		if l.DeletedAt != nil {
			if _, e := time.Parse(time.RFC3339Nano, *l.DeletedAt); e != nil {
				return fault.Validation("Ungültiger Papierkorbzeitstempel")
			}
		}
		for fid, v := range l.Values {
			d, ok := dm[fid]
			if !ok || v.Type != d.Type {
				return fault.Validation("Unbekanntes Feld oder falscher Werttyp")
			}
			d.Config.Options = append([]fields.Option(nil), d.Config.Options...)
			for i := range d.Config.Options {
				d.Config.Options[i].Archived = false
			}
			value, e := fields.ValidateValue(d, v.Value)
			if e != nil {
				return fault.Validation(e.Error())
			}
			if len(value.Raw) == 0 {
				return fault.Validation("Leere Wertzeilen sind ungültig")
			}
			if value.Media != nil {
				if _, ok := mm[*value.Media]; !ok {
					return fault.Validation("Unbekannte Medienreferenz")
				}
			}
			for _, tid := range value.Tags {
				tag, ok := tags[tid]
				if !ok || tag.FieldID != fid {
					return fault.Validation("Ungültige Tagzuordnung")
				}
			}
		}
		for _, d := range c.Fields {
			_, has := l.Values[d.ID]
			required, e := fields.RequiredForRecord(d, l.CreatedAt)
			if e != nil {
				return fault.Validation(e.Error())
			}
			if required && !has {
				return fault.Validation("Pflichtfeldwert fehlt")
			}
		}
		return nil
	}
}
