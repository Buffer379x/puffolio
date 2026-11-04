package liquids

import (
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/fields"
	"math"
	"time"
)

// ShelfLife is how much of a liquid's storage time is left.
type ShelfLife struct {
	// Percent is 100 on the purchase date and 0 once the storage time is over.
	Percent   int    `json:"percent"`
	ExpiresOn string `json:"expiresOn"`
	Expired   bool   `json:"expired"`
}

// ShelfRules is the single definition of shelf life. Of computes it for display;
// the SQL builders express the same rule for sorting and counting, so the bar, the
// widgets and the table order cannot disagree. A consumed liquid has no shelf life.
type ShelfRules struct {
	PurchaseField  string
	OverrideField  string
	OverrideScale  int
	StatusField    string
	ConsumedOption string
	DefaultMonths  int
	Today          string
}

const dateLayout = "2006-01-02"

// addMonths clamps to the end of the month: 31 January plus one month is the end of
// February. SQLite's 'floor' modifier used by the SQL builders behaves identically.
func addMonths(day time.Time, months int) time.Time {
	first := time.Date(day.Year(), day.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(day.Day(), last)-1)
}
func stringValue(l Liquid, field string) string {
	if field == "" {
		return ""
	}
	var s string
	if json.Unmarshal(l.Values[field].Value, &s) != nil {
		return ""
	}
	return s
}
func (r *ShelfRules) months(l Liquid) int {
	if n, e := fields.ParseNumber(stringValue(l, r.OverrideField), r.OverrideScale); e == nil && n > 0 {
		for i := 0; i < r.OverrideScale; i++ {
			n /= 10
		}
		if n > 0 {
			return int(n)
		}
	}
	return r.DefaultMonths
}

// Of returns nil when no bar applies: no date, no storage time, or consumed.
func (r *ShelfRules) Of(l Liquid) *ShelfLife {
	if r == nil || r.ConsumedOption != "" && stringValue(l, r.StatusField) == r.ConsumedOption {
		return nil
	}
	purchase, e := time.Parse(dateLayout, stringValue(l, r.PurchaseField))
	months := r.months(l)
	today, todayErr := time.Parse(dateLayout, r.Today)
	if e != nil || todayErr != nil || months <= 0 {
		return nil
	}
	expires := addMonths(purchase, months)
	life := &ShelfLife{ExpiresOn: expires.Format(dateLayout)}
	left := expires.Sub(today).Hours() / expires.Sub(purchase).Hours()
	if left <= 0 {
		life.Expired = true
		return life
	}
	life.Percent = int(math.Round(math.Min(left, 1) * 100))
	return life
}

// Apply fills in the shelf life of each liquid.
func (r *ShelfRules) Apply(items []Liquid) {
	for i := range items {
		items[i].ShelfLife = r.Of(items[i])
	}
}

// valueSQL selects one stored value of the liquid aliased l.
func valueSQL(column, field string) (string, []any) {
	return "(SELECT " + column + " FROM field_values sv WHERE sv.collection_id=l.collection_id AND sv.liquid_id=l.id AND sv.field_id=?)", []any{field}
}

// ExpirySQL is the expiry date of the liquid aliased l, NULL when Of would return nil.
func (r *ShelfRules) ExpirySQL() (string, []any) {
	months, args := "NULLIF(?,0)", []any{r.DefaultMonths}
	if r.OverrideField != "" {
		factor := int64(1)
		for i := 0; i < r.OverrideScale; i++ {
			factor *= 10
		}
		override, overrideArgs := valueSQL("sv.value_number/?", r.OverrideField)
		months = "COALESCE(NULLIF(MAX(" + override + ",0),0),NULLIF(?,0))"
		args = append([]any{factor}, append(overrideArgs, r.DefaultMonths)...)
	}
	purchase, purchaseArgs := valueSQL("sv.value_text", r.PurchaseField)
	expiry := "date(" + purchase + ",'+'||" + months + "||' months','floor')"
	args = append(purchaseArgs, args...)
	if r.ConsumedOption == "" {
		return expiry, args
	}
	status, statusArgs := valueSQL("sv.value_option", r.StatusField)
	return "CASE WHEN " + status + "=? THEN NULL ELSE " + expiry + " END", append(append(statusArgs, r.ConsumedOption), args...)
}

// RemainingSQL orders by the same share of time left that the bar shows.
func (r *ShelfRules) RemainingSQL() (string, []any) {
	expiry, expiryArgs := r.ExpirySQL()
	purchase, purchaseArgs := valueSQL("sv.value_text", r.PurchaseField)
	args := append([]any{}, expiryArgs...)
	args = append(args, r.Today)
	args = append(args, expiryArgs...)
	args = append(args, purchaseArgs...)
	return "((julianday(" + expiry + ")-julianday(?))/(julianday(" + expiry + ")-julianday(" + purchase + ")))", args
}
