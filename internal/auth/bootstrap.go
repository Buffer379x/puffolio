package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
)

type ApplicationPreferences struct {
	Version    int               `json:"version"`
	Theme      string            `json:"theme"`
	Timezone   string            `json:"timezone"`
	View       string            `json:"view"`
	Images     bool              `json:"images"`
	ListFields []string          `json:"listFields"`
	GridFields []string          `json:"gridFields"`
	Roles      map[string]string `json:"roles"`
	// ShelfLifeMonths runs from the date field in Roles["purchase"], unless the number
	// field in Roles["shelfOverride"] holds a liquid's own value. 0 means no default.
	ShelfLifeMonths int `json:"shelfLifeMonths"`
	// ConsumedOption is the option of the select field in Roles["status"] that means
	// a liquid is used up: it then has no shelf life and never counts as expired.
	ConsumedOption string `json:"consumedOption,omitempty"`
}

// InitializeUserTx creates only missing application state when an account becomes usable.
func InitializeUserTx(ctx context.Context, tx *sql.Tx, u User, roles map[string]string) error {
	p := ApplicationPreferences{Version: 1, Theme: "system", Timezone: "Europe/Berlin", View: "grid", Images: true, ListFields: []string{}, GridFields: []string{}, Roles: roles}
	if roles["purchase"] != "" {
		p.ShelfLifeMonths = 60
	}
	defs, e := fields.List(ctx, tx, u.CollectionID)
	if e != nil {
		return e
	}
	for _, d := range defs {
		if d.ID == roles["status"] {
			for _, option := range d.Config.Options {
				if option.Label == "Aufgebraucht" {
					p.ConsumedOption = option.ID
				}
			}
		}
		if d.Visibility.List {
			p.ListFields = append(p.ListFields, d.ID)
		}
		if d.Visibility.Grid {
			p.GridFields = append(p.GridFields, d.ID)
		}
	}
	raw, _ := json.Marshal(p)
	if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO user_preferences(user_id,data) VALUES(?,?)", u.ID, string(raw)); e != nil {
		return e
	}
	widgets := defaultWidgets(defs, roles)
	b, _ := json.Marshal(widgets)
	_, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO dashboards(user_id,data) VALUES(?,?)", u.ID, string(b))
	return e
}

// defaultWidgets opens on a useful dashboard: every widget refers to a field by
// ID, so an account without starter fields only gets those that need none.
func defaultWidgets(defs []fields.Definition, roles map[string]string) []map[string]any {
	widget := func(kind, title, size string, extra map[string]any) map[string]any {
		w := map[string]any{"id": store.ID(), "type": kind, "title": title, "size": size}
		for k, v := range extra {
			w[k] = v
		}
		return w
	}
	first := func(kind string) *fields.Definition {
		for i := range defs {
			if defs[i].Type == kind && !defs[i].Archived {
				return &defs[i]
			}
		}
		return nil
	}
	out := []map[string]any{widget("count", "Liquids", "small", nil), widget("favorites", "Favoriten", "small", nil)}
	if rating := roles["rating"]; rating != "" {
		out = append(out, widget("average", "Bewertung", "small", map[string]any{"fieldId": rating}))
	}
	purchase := roles["purchase"]
	if purchase != "" {
		out = append(out, widget("expired", "Abgelaufen", "small", nil))
	}
	out = append(out, widget("recent", "Zuletzt hinzugefügt", "wide", map[string]any{"limit": 5}))
	if tags := first("tags"); tags != nil {
		out = append(out, widget("tags", tags.Label, "medium", map[string]any{"fieldId": tags.ID, "limit": 5}))
	}
	if purchase != "" {
		out = append(out, widget("dates", "Zuletzt gekauft", "wide", map[string]any{"fieldId": purchase, "limit": 5, "window": "past"}))
		out = append(out, widget("expiring", "Läuft bald ab", "medium", map[string]any{"limit": 5}))
	}
	return out
}
