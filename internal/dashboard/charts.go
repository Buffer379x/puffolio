package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"math"
	"sort"
	"time"
)

const valueJoin = " FROM liquids l JOIN field_values fv ON fv.collection_id=l.collection_id AND fv.liquid_id=l.id WHERE "

// distribution splits the collection by one field: the largest parts, the rest as
// "Weitere", and the liquids without a value, so the parts always add up.
func distribution(ctx context.Context, db *sql.DB, res Result, d fields.Definition, where string, args []any, limit int) (Result, error) {
	var total int
	if e := db.QueryRowContext(ctx, "SELECT count(*) FROM liquids l WHERE "+where, args...).Scan(&total); e != nil {
		return res, e
	}
	res.Count = &total
	segments := []Segment{}
	withValue := 0
	if d.Type == "tags" {
		rows, e := db.QueryContext(ctx, "SELECT t.id,t.name,t.palette_id,count(*) FROM liquids l JOIN liquid_tags lt ON lt.collection_id=l.collection_id AND lt.liquid_id=l.id JOIN tags t ON t.collection_id=lt.collection_id AND t.field_id=lt.field_id AND t.id=lt.tag_id WHERE "+where+" AND lt.field_id=? GROUP BY t.id,t.name,t.palette_id", append(append([]any{}, args...), d.ID)...)
		if e != nil {
			return res, e
		}
		for rows.Next() {
			var s Segment
			if e = rows.Scan(&s.ID, &s.Label, &s.PaletteID, &s.Count); e != nil {
				rows.Close()
				return res, e
			}
			segments = append(segments, s)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return res, e
		}
		if e = db.QueryRowContext(ctx, "SELECT count(DISTINCT l.id) FROM liquids l JOIN liquid_tags lt ON lt.collection_id=l.collection_id AND lt.liquid_id=l.id WHERE "+where+" AND lt.field_id=?", append(append([]any{}, args...), d.ID)...).Scan(&withValue); e != nil {
			return res, e
		}
	} else {
		var key, label string
		switch d.Type {
		case "text":
			// Spelling differences in case do not split a manufacturer in two.
			key, label = "fv.value_search", "MAX(fv.value_text)"
		case "select":
			key, label = "fv.value_option", "fv.value_option"
		case "rating":
			key, label = "fv.value_number", "fv.value_number"
		case "boolean":
			key, label = "fv.value_bool", "fv.value_bool"
		default:
			return res, fault.Validation("Text-, Auswahl-, Bewertungs-, Ja/Nein- oder Tagfeld erforderlich")
		}
		rows, e := db.QueryContext(ctx, "SELECT "+label+",count(*)"+valueJoin+where+" AND fv.field_id=? GROUP BY "+key, append(append([]any{}, args...), d.ID)...)
		if e != nil {
			return res, e
		}
		for rows.Next() {
			var raw any
			var n int
			if e = rows.Scan(&raw, &n); e != nil {
				rows.Close()
				return res, e
			}
			s := Segment{ID: fmt.Sprint(raw), Label: fmt.Sprint(raw), Count: n}
			switch d.Type {
			case "text":
				if b, ok := raw.([]byte); ok {
					s.ID, s.Label = string(b), string(b)
				}
			case "select":
				for _, option := range d.Config.Options {
					if option.ID == s.ID {
						s.Label, s.PaletteID = option.Label, option.PaletteID
					}
				}
			case "rating":
				s.Label += " Sterne"
				if s.ID == "1" {
					s.Label = "1 Stern"
				}
			case "boolean":
				s.Label = "Nein"
				if s.ID == "1" {
					s.Label = "Ja"
				}
			}
			segments = append(segments, s)
			withValue += n
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return res, e
		}
	}
	sort.SliceStable(segments, func(i, j int) bool {
		if segments[i].Count != segments[j].Count {
			return segments[i].Count > segments[j].Count
		}
		return segments[i].Label < segments[j].Label
	})
	if len(segments) > limit {
		rest := 0
		for _, s := range segments[limit:] {
			rest += s.Count
		}
		segments = append(segments[:limit:limit], Segment{ID: "other", Label: "Weitere", Count: rest})
	}
	if missing := total - withValue; missing > 0 {
		segments = append(segments, Segment{ID: "none", Label: "Ohne Angabe", Count: missing})
	}
	res.Segments = segments
	return res, nil
}

// ratingColumns counts each step of the scale, keeping empty steps so the shape is honest.
func ratingColumns(ctx context.Context, db *sql.DB, res Result, d fields.Definition, where string, args []any) (Result, error) {
	if d.Type != "rating" {
		return res, fault.Validation("Bewertungsfeld erforderlich")
	}
	rows, e := db.QueryContext(ctx, "SELECT fv.value_number,count(*)"+valueJoin+where+" AND fv.field_id=? GROUP BY fv.value_number", append(append([]any{}, args...), d.ID)...)
	if e != nil {
		return res, e
	}
	defer rows.Close()
	counts := map[int64]int{}
	total := 0
	for rows.Next() {
		var value int64
		var n int
		if e = rows.Scan(&value, &n); e != nil {
			return res, e
		}
		counts[value] = n
		total += n
	}
	if e = rows.Err(); e != nil {
		return res, e
	}
	stars := d.Config.Stars
	if stars == 0 {
		stars = 5
	}
	for i := 1; i <= stars; i++ {
		res.Segments = append(res.Segments, Segment{ID: fmt.Sprint(i), Label: fmt.Sprint(i), Count: counts[int64(i)]})
	}
	res.Count = &total
	return res, nil
}

// shelfStatus groups by the share of time left, with the same rule the bar uses.
func shelfStatus(ctx context.Context, db *sql.DB, res Result, where string, args []any, o Options) (Result, error) {
	total := 0
	res.Count = &total
	if o.Shelf == nil {
		return res, nil
	}
	remaining, remainingArgs := o.Shelf.RemainingSQL()
	rows, e := db.QueryContext(ctx, "SELECT "+remaining+" FROM liquids l WHERE "+where, append(append([]any{}, remainingArgs...), args...)...)
	if e != nil {
		return res, e
	}
	defer rows.Close()
	res.Segments = []Segment{{ID: "fresh", Label: "Ab 50 %"}, {ID: "half", Label: "Unter 50 %"}, {ID: "soon", Label: "Unter 20 %"}, {ID: "expired", Label: "Abgelaufen"}}
	for rows.Next() {
		var left sql.NullFloat64
		if e = rows.Scan(&left); e != nil {
			return res, e
		}
		if !left.Valid {
			continue
		}
		total++
		percent := math.Round(math.Min(left.Float64, 1) * 100)
		switch {
		case left.Float64 <= 0:
			res.Segments[3].Count++
		case percent < 20:
			res.Segments[2].Count++
		case percent < 50:
			res.Segments[1].Count++
		default:
			res.Segments[0].Count++
		}
	}
	return res, rows.Err()
}

// timeline counts a date field per month over the last twelve months, empty months included.
func timeline(ctx context.Context, db *sql.DB, res Result, d fields.Definition, where string, args []any, o Options) (Result, error) {
	if d.Type != "date" {
		return res, fault.Validation("Datumsfeld erforderlich")
	}
	today, e := time.Parse("2006-01-02", o.Today)
	if e != nil {
		return res, e
	}
	month := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	first := month.AddDate(0, -11, 0).Format("2006-01")
	rows, e := db.QueryContext(ctx, "SELECT substr(fv.value_text,1,7),count(*)"+valueJoin+where+" AND fv.field_id=? AND substr(fv.value_text,1,7) BETWEEN ? AND ? GROUP BY 1", append(append([]any{}, args...), d.ID, first, month.Format("2006-01"))...)
	if e != nil {
		return res, e
	}
	defer rows.Close()
	counts := map[string]int{}
	total := 0
	for rows.Next() {
		var key string
		var n int
		if e = rows.Scan(&key, &n); e != nil {
			return res, e
		}
		counts[key] = n
		total += n
	}
	if e = rows.Err(); e != nil {
		return res, e
	}
	for i := 11; i >= 0; i-- {
		m := month.AddDate(0, -i, 0)
		res.Segments = append(res.Segments, Segment{ID: m.Format("2006-01"), Label: m.Format("01/06"), Count: counts[m.Format("2006-01")]})
	}
	res.Count = &total
	return res, nil
}
