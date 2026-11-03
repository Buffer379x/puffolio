package backup

import (
	"context"
	"database/sql"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"time"
)

// IDs are remapped, so incoming records cannot supply existing field values and
// existing records cannot supply incoming field values. Reject incompatible
// active required rules instead of inventing values or weakening either policy.
func checkDestination(ctx context.Context, q fields.Querier, cid string, c Collection) (uid string, position int, err error) {
	if err = q.QueryRowContext(ctx, "SELECT user_id FROM collections WHERE id=?", cid).Scan(&uid); err == sql.ErrNoRows {
		return "", 0, fault.NotFound()
	}
	if err != nil {
		return "", 0, err
	}
	defs, err := fields.List(ctx, q, cid)
	if err != nil {
		return "", 0, err
	}
	position = len(defs)
	if position+len(c.Fields) > 100 {
		return "", 0, fault.Validation("Import würde Grenze von 100 Feldern überschreiten")
	}
	var views int
	if err = q.QueryRowContext(ctx, "SELECT count(*) FROM saved_views WHERE user_id=?", uid).Scan(&views); err != nil {
		return "", 0, err
	}
	if views+len(c.Views) > 100 {
		return "", 0, fault.Validation("Import würde Grenze von 100 gespeicherten Ansichten überschreiten")
	}
	if c.liquidCount > 0 {
		for _, d := range defs {
			applies, e := fields.RequiredForRecord(d, c.latestLiquidCreatedAt.Format(time.RFC3339Nano))
			if e != nil {
				return "", 0, fault.Validation(e.Error())
			}
			if applies {
				return "", 0, fault.Validation("Import abgelehnt: Importierten Liquids fehlen Pflichtwerte der Zielsammlung")
			}
		}
	}
	needsExisting := false
	for _, d := range c.Fields {
		if d.Required && !d.Archived {
			needsExisting = true
			break
		}
	}
	if !needsExisting {
		return uid, position, nil
	}
	rows, err := q.QueryContext(ctx, "SELECT created_at FROM liquids WHERE collection_id=?", cid)
	if err != nil {
		return "", 0, err
	}
	var latest time.Time
	hasExisting := false
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return "", 0, err
		}
		created, e := time.Parse(time.RFC3339Nano, raw)
		if e != nil {
			rows.Close()
			return "", 0, fault.Validation("Vorhandener Erstellungszeitpunkt ist ungültig")
		}
		if !hasExisting || created.After(latest) {
			latest = created
			hasExisting = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", 0, err
	}
	if hasExisting {
		for _, d := range c.Fields {
			applies, e := fields.RequiredForRecord(d, latest.Format(time.RFC3339Nano))
			if e != nil {
				return "", 0, fault.Validation(e.Error())
			}
			if applies {
				return "", 0, fault.Validation("Import abgelehnt: Neue Pflichtfelder würden bestehende Liquids unvollständig machen")
			}
		}
	}
	return uid, position, nil
}
