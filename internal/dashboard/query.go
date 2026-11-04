package dashboard

import (
	"context"
	"database/sql"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"
)

type Widget struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Size      string          `json:"size"`
	FieldID   string          `json:"fieldId,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	Threshold string          `json:"threshold,omitempty"`
	Inclusive bool            `json:"inclusive,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	Window    string          `json:"window,omitempty"`
	Filters   *liquids.Filter `json:"filters,omitempty"`
}
type TagCount struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PaletteID string `json:"paletteId"`
	Count     int    `json:"count"`
}
type Result struct {
	ID    string           `json:"id"`
	Value string           `json:"value,omitempty"`
	Count *int             `json:"count,omitempty"`
	Items []liquids.Liquid `json:"items,omitempty"`
	Tags  []TagCount       `json:"tags,omitempty"`
	Error string           `json:"error,omitempty"`
}

func Validate(widgets []Widget) error {
	if len(widgets) > 30 {
		return fault.Validation("Maximal 30 Widgets")
	}
	seen := map[string]bool{}
	for _, w := range widgets {
		if w.Filters != nil {
			if e := liquids.ValidateFilterShape(*w.Filters); e != nil {
				return e
			}
		}
		if w.ID == "" || seen[w.ID] || utf8.RuneCountInString(w.Title) > 100 {
			return fault.Validation("Ungültiges Widget")
		}
		seen[w.ID] = true
		switch w.Type {
		case "count", "sum", "favorites", "average", "recent", "low", "tags", "dates", "expired", "expiring":
		default:
			return fault.Validation("Unbekannter Widgettyp")
		}
		if w.Size != "small" && w.Size != "medium" && w.Size != "wide" {
			return fault.Validation("Ungültige Widgetgröße")
		}
		if w.Limit != 0 && w.Limit != 3 && w.Limit != 5 && w.Limit != 10 {
			return fault.Validation("Ungültiges Widgetlimit")
		}
		if w.Window != "" && w.Window != "past" && w.Window != "upcoming" {
			return fault.Validation("Ungültiger Datumszeitraum")
		}
	}
	return nil
}
func Query(ctx context.Context, db *sql.DB, cid string, widgets []Widget) ([]Result, error) {
	return QueryAt(ctx, db, cid, widgets, Options{Today: time.Now().UTC().Format("2006-01-02")})
}

// Options carry what relative widgets need: Today is the calendar date in the
// account's timezone, Shelf the storage-time rules (nil when not set up).
type Options struct {
	Today string
	Shelf *liquids.ShelfRules
}

// QueryAt resolves relative date windows at query time. A date widget with a
// window stays current without anyone editing it; a stored from/to range would
// silently go stale.
func QueryAt(ctx context.Context, db *sql.DB, cid string, widgets []Widget, o Options) ([]Result, error) {
	if len(widgets) > 30 {
		return nil, fault.Validation("Maximal 30 Widgets")
	}
	defs, e := fields.List(ctx, db, cid)
	if e != nil {
		return nil, e
	}
	dm := map[string]fields.Definition{}
	for _, d := range defs {
		dm[d.ID] = d
	}
	out := make([]Result, 0, len(widgets))
	for _, w := range widgets {
		res, e := queryOne(ctx, db, cid, w, dm, o)
		if e != nil {
			res = Result{ID: w.ID, Error: e.Error()}
			if fault.Wrap(e).Status == 500 {
				res.Error = "Abfrage fehlgeschlagen"
			}
		}
		out = append(out, res)
	}
	return out, nil
}
func queryOne(ctx context.Context, db *sql.DB, cid string, w Widget, dm map[string]fields.Definition, o Options) (Result, error) {
	res := Result{ID: w.ID}
	where := "l.collection_id=? AND l.deleted_at IS NULL"
	args := []any{cid}
	if w.Filters != nil {
		q, a, e := liquids.BuildFilter(*w.Filters, dm)
		if e != nil {
			return res, e
		}
		where += " AND (" + q + ")"
		args = append(args, a...)
	}
	limit := w.Limit
	if limit == 0 {
		limit = 5
	}
	if limit != 3 && limit != 5 && limit != 10 {
		return res, fault.Validation("Ungültiges Limit")
	}
	var d fields.Definition
	if w.Type != "count" && w.Type != "favorites" && w.Type != "recent" && w.Type != "expired" && w.Type != "expiring" {
		var ok bool
		d, ok = dm[w.FieldID]
		if !ok || d.Archived {
			return res, fault.Validation("Feld fehlt oder ist archiviert")
		}
	}
	switch w.Type {
	case "expired":
		count := 0
		res.Count = &count
		if o.Shelf == nil {
			return res, nil
		}
		expiry, expiryArgs := o.Shelf.ExpirySQL()
		e := db.QueryRowContext(ctx, "SELECT count(*) FROM liquids l WHERE "+where+" AND "+expiry+"<=?", append(append(args, expiryArgs...), o.Today)...).Scan(&count)
		return res, e
	case "expiring":
		res.Items = []liquids.Liquid{}
		if o.Shelf == nil {
			return res, nil
		}
		expiry, expiryArgs := o.Shelf.ExpirySQL()
		queryArgs := append(append([]any{}, args...), expiryArgs...)
		queryArgs = append(append(queryArgs, expiryArgs...), limit)
		rows, e := db.QueryContext(ctx, "SELECT l.id FROM liquids l WHERE "+where+" AND "+expiry+" IS NOT NULL ORDER BY "+expiry+" ASC,l.id LIMIT ?", queryArgs...)
		if e != nil {
			return res, e
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return res, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return res, e
		}
		for _, id := range ids {
			l, e := liquids.Get(ctx, db, cid, id)
			if e != nil {
				return res, e
			}
			res.Items = append(res.Items, l)
		}
		o.Shelf.Apply(res.Items)
		return res, nil
	case "count", "favorites":
		if w.Type == "favorites" {
			where += " AND l.favorite=1"
		}
		var count int
		e := db.QueryRowContext(ctx, "SELECT count(*) FROM liquids l WHERE "+where, args...).Scan(&count)
		res.Count = &count
		return res, e
	case "sum", "average":
		if w.Type == "sum" && d.Type != "number" || w.Type == "average" && d.Type != "number" && d.Type != "rating" {
			return res, fault.Validation("Passendes Zahlenfeld erforderlich")
		}
		rows, e := db.QueryContext(ctx, "SELECT fv.value_number FROM liquids l JOIN field_values fv ON fv.collection_id=l.collection_id AND fv.liquid_id=l.id WHERE "+where+" AND fv.field_id=? AND fv.value_number IS NOT NULL", append(args, d.ID)...)
		if e != nil {
			return res, e
		}
		total := new(big.Int)
		count := 0
		for rows.Next() {
			var n int64
			if e = rows.Scan(&n); e != nil {
				rows.Close()
				return res, e
			}
			total.Add(total, big.NewInt(n))
			count++
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return res, e
		}
		res.Count = &count
		if w.Type == "sum" && count == 0 {
			return res, nil
		}
		if w.Type == "average" {
			// Nothing rated yet is a normal state, not a broken widget.
			if count == 0 {
				return res, nil
			}
			denom := big.NewInt(int64(count))
			if d.Type == "number" {
				for i := 0; i < d.Config.Scale; i++ {
					denom.Mul(denom, big.NewInt(10))
				}
			}
			res.Value = new(big.Rat).SetFrac(total, denom).FloatString(2)
		} else {
			res.Value = formatBig(total, d.Config.Scale)
		}
		res.Count = &count
		return res, nil
	case "tags":
		if d.Type != "tags" {
			return res, fault.Validation("Tagfeld erforderlich")
		}
		rows, e := db.QueryContext(ctx, "SELECT t.id,t.name,t.palette_id,count(*) FROM liquids l JOIN liquid_tags lt ON lt.collection_id=l.collection_id AND lt.liquid_id=l.id JOIN tags t ON t.collection_id=lt.collection_id AND t.field_id=lt.field_id AND t.id=lt.tag_id WHERE "+where+" AND lt.field_id=? GROUP BY t.id,t.name,t.palette_id ORDER BY count(*) DESC,t.normalized_name,t.id LIMIT ?", append(args, d.ID, limit)...)
		if e != nil {
			return res, e
		}
		res.Tags = []TagCount{}
		for rows.Next() {
			var tag TagCount
			if e = rows.Scan(&tag.ID, &tag.Name, &tag.PaletteID, &tag.Count); e != nil {
				rows.Close()
				return res, e
			}
			res.Tags = append(res.Tags, tag)
		}
		e = rows.Err()
		rows.Close()
		return res, e
	case "recent", "low", "dates":
		source := "liquids l"
		if w.Type != "recent" {
			source += " JOIN field_values window_values ON window_values.collection_id=l.collection_id AND window_values.liquid_id=l.id"
		}
		sort := "l.created_at DESC,l.id"
		if w.Type == "low" {
			if d.Type != "number" {
				return res, fault.Validation("Zahlenfeld erforderlich")
			}
			n, e := fields.ParseNumber(w.Threshold, d.Config.Scale)
			if e != nil {
				return res, fault.Validation("Ungültiger Schwellenwert")
			}
			op := "<"
			if w.Inclusive {
				op = "<="
			}
			where += " AND window_values.field_id=? AND window_values.value_number" + op + "?"
			args = append(args, d.ID, n)
			sort = "window_values.value_number ASC,l.id"
		}
		if w.Type == "dates" {
			if d.Type != "date" {
				return res, fault.Validation("Datumsfeld erforderlich")
			}
			switch w.Window {
			case "past":
				where += " AND window_values.field_id=? AND window_values.value_text<=?"
				args = append(args, d.ID, o.Today)
				sort = "window_values.value_text DESC,l.id"
			case "upcoming":
				where += " AND window_values.field_id=? AND window_values.value_text>=?"
				args = append(args, d.ID, o.Today)
				sort = "window_values.value_text ASC,l.id"
			default:
				for _, date := range []string{w.From, w.To} {
					if _, e := time.Parse("2006-01-02", date); e != nil {
						return res, fault.Validation("Gültiges Datumsfenster erforderlich")
					}
				}
				if w.From > w.To {
					return res, fault.Validation("Ungültiges Datumsfenster")
				}
				where += " AND window_values.field_id=? AND window_values.value_text BETWEEN ? AND ?"
				args = append(args, d.ID, w.From, w.To)
				sort = "window_values.value_text ASC,l.id"
			}
		}
		var count int
		if e := db.QueryRowContext(ctx, "SELECT count(*) FROM "+source+" WHERE "+where, args...).Scan(&count); e != nil {
			return res, e
		}
		res.Count = &count
		queryArgs := append([]any{}, args...)
		queryArgs = append(queryArgs, limit)
		rows, e := db.QueryContext(ctx, "SELECT l.id FROM "+source+" WHERE "+where+" ORDER BY "+sort+" LIMIT ?", queryArgs...)
		if e != nil {
			return res, e
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return res, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return res, e
		}
		res.Items = []liquids.Liquid{}
		for _, id := range ids {
			l, e := liquids.Get(ctx, db, cid, id)
			if e != nil {
				return res, e
			}
			res.Items = append(res.Items, l)
		}
		o.Shelf.Apply(res.Items)
		return res, nil
	default:
		return res, fault.Validation("Unbekannter Widgettyp")
	}
}
func formatBig(n *big.Int, scale int) string {
	s := n.String()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for len(s) <= scale {
		s = "0" + s
	}
	if scale > 0 {
		s = s[:len(s)-scale] + "." + s[len(s)-scale:]
	}
	if neg {
		s = "-" + s
	}
	return s
}
