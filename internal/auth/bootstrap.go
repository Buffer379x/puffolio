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
}

// InitializeUserTx creates only missing application state when an account becomes usable.
func InitializeUserTx(ctx context.Context, tx *sql.Tx, u User, roles map[string]string) error {
	p := ApplicationPreferences{Version: 1, Theme: "system", Timezone: "Europe/Berlin", View: "grid", Images: true, ListFields: []string{}, GridFields: []string{}, Roles: roles}
	defs, e := fields.List(ctx, tx, u.CollectionID)
	if e != nil {
		return e
	}
	for _, d := range defs {
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
	widgets := []map[string]any{{"id": store.ID(), "type": "count", "title": "Liquids", "size": "small"}, {"id": store.ID(), "type": "favorites", "title": "Favoriten", "size": "small"}, {"id": store.ID(), "type": "recent", "title": "Zuletzt hinzugefügt", "size": "wide", "limit": 5}}
	b, _ := json.Marshal(widgets)
	_, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO dashboards(user_id,data) VALUES(?,?)", u.ID, string(b))
	return e
}
