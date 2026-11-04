package liquids

import (
	"context"
	"encoding/json"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/store"
	"strings"
	"testing"
	"time"
)

func shelfLiquid(values map[string]string) Liquid {
	l := Liquid{Values: map[string]fields.Value{}}
	for id, value := range values {
		raw, _ := json.Marshal(value)
		l.Values[id] = fields.Value{Value: raw}
	}
	return l
}
func TestShelfLifeFollowsTheCalendar(t *testing.T) {
	rules := func(today string, months int) *ShelfRules {
		return &ShelfRules{PurchaseField: "date", OverrideField: "own", StatusField: "status", ConsumedOption: "used", DefaultMonths: months, Today: today}
	}
	bought := shelfLiquid(map[string]string{"date": "2026-01-01"})
	for _, c := range []struct {
		today, expires string
		percent        int
		expired        bool
	}{
		{"2026-01-01", "2031-01-01", 100, false},
		{"2025-06-01", "2031-01-01", 100, false},
		{"2027-01-01", "2031-01-01", 80, false},
		{"2030-12-31", "2031-01-01", 0, false},
		{"2031-01-01", "2031-01-01", 0, true},
		{"2040-06-01", "2031-01-01", 0, true},
	} {
		got := rules(c.today, 60).Of(bought)
		if got == nil || got.Percent != c.percent || got.ExpiresOn != c.expires || got.Expired != c.expired {
			t.Fatalf("%s: %+v", c.today, got)
		}
	}
	if got := rules("2027-01-01", 0).Of(bought); got != nil {
		t.Fatalf("no storage time: %+v", got)
	}
	if got := rules("2027-01-01", 60).Of(shelfLiquid(nil)); got != nil {
		t.Fatalf("no purchase date: %+v", got)
	}
	own := shelfLiquid(map[string]string{"date": "2026-01-01", "own": "24"})
	if got := rules("2027-01-01", 60).Of(own); got.Percent != 50 || got.ExpiresOn != "2028-01-01" {
		t.Fatalf("own storage time wins: %+v", got)
	}
	if got := rules("2027-01-01", 0).Of(own); got == nil || got.Percent != 50 {
		t.Fatalf("own storage time works without a default: %+v", got)
	}
	if got := rules("2027-01-01", 60).Of(shelfLiquid(map[string]string{"date": "2026-01-01", "status": "used"})); got != nil {
		t.Fatalf("consumed liquid: %+v", got)
	}
	if got := rules("2027-01-01", 60).Of(shelfLiquid(map[string]string{"date": "2026-01-01", "status": "other"})); got == nil {
		t.Fatal("another status must keep the bar")
	}
	for _, c := range [][3]string{{"2026-01-31", "1", "2026-02-28"}, {"2028-01-31", "1", "2028-02-29"}, {"2024-02-29", "12", "2025-02-28"}, {"2026-12-15", "1", "2027-01-15"}} {
		day, _ := time.Parse(dateLayout, c[0])
		months := map[string]int{"1": 1, "12": 12}[c[1]]
		if got := addMonths(day, months).Format(dateLayout); got != c[2] {
			t.Fatalf("%v: %s", c, got)
		}
	}
}

// The database must reach the same verdict as the bar, or widgets and sorting drift.
func TestShelfSQLMatchesTheBar(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := auth.New(s).CreateUser(ctx, "alice", "Alice", "user", "A-long-test-password")
	if e != nil {
		t.Fatal(e)
	}
	date, _ := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Kauf", Type: "date"}, true)
	own, _ := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Eigene", Type: "number"}, true)
	status, e := fields.Save(ctx, s.DB, u.CollectionID, fields.Definition{Label: "Status", Type: "select", Config: fields.Config{Options: []fields.Option{{Label: "Vorrat"}, {Label: "Aufgebraucht"}}}}, true)
	if e != nil {
		t.Fatal(e)
	}
	stock, used := status.Config.Options[0].ID, status.Config.Options[1].ID
	add := func(name, day, months, option string) {
		values := map[string]*fields.Value{}
		put := func(d fields.Definition, v string) {
			if v != "" {
				raw, _ := json.Marshal(v)
				values[d.ID] = &fields.Value{Type: d.Type, Value: raw}
			}
		}
		put(date, day)
		put(own, months)
		put(status, option)
		if _, e := Save(ctx, s.DB, u.CollectionID, "", Input{Name: &name, Values: values}, nil); e != nil {
			t.Fatal(name, e)
		}
	}
	add("ablaufend", "2022-01-01", "", stock)     // 60 months, 20 %
	add("eigene-kurz", "2026-07-01", "12", stock) // 12 months, 50 %
	add("frisch", "2027-01-01", "", stock)        // 60 months, 100 %
	add("monatsende", "2026-01-31", "13", stock)  // clamps to 2027-02-28, 0 % soon
	add("abgelaufen", "2020-01-01", "", stock)    // over
	add("aufgebraucht", "2020-01-01", "", used)   // no bar despite its age
	add("ohne-datum", "", "", stock)              // no bar
	rules := &ShelfRules{PurchaseField: date.ID, OverrideField: own.ID, StatusField: status.ID, ConsumedOption: used, DefaultMonths: 60, Today: "2027-01-01"}
	page, e := List(ctx, s.DB, u.CollectionID, Query{Sort: "shelf", Direction: "asc", Page: 1, PageSize: 24, Shelf: rules})
	if e != nil {
		t.Fatal(e)
	}
	names := []string{}
	percent := -1
	for _, l := range page.Items {
		names = append(names, l.Name)
		if life := rules.Of(l); life != nil {
			if life.Percent < percent {
				t.Fatalf("order disagrees with the bar: %s %d after %d", l.Name, life.Percent, percent)
			}
			percent = life.Percent
		}
		query, args := rules.ExpirySQL()
		var expiry *string
		if e := s.DB.QueryRowContext(ctx, "SELECT "+query+" FROM liquids l WHERE l.id=?", append(args, l.ID)...).Scan(&expiry); e != nil {
			t.Fatal(e)
		}
		life := rules.Of(l)
		if (life == nil) != (expiry == nil) || life != nil && life.ExpiresOn != *expiry {
			t.Fatalf("%s: bar %+v, database %v", l.Name, life, expiry)
		}
	}
	joined := strings.Join(names, ",")
	if !strings.HasPrefix(joined, "abgelaufen,") || !strings.HasSuffix(joined, "aufgebraucht,ohne-datum") && !strings.HasSuffix(joined, "ohne-datum,aufgebraucht") {
		t.Fatalf("expired first, no bar last: %s", joined)
	}
	if _, e := List(ctx, s.DB, u.CollectionID, Query{Sort: "shelf", Page: 1, PageSize: 24}); e != nil {
		t.Fatal("sorting by shelf life without rules must not fail: ", e)
	}
}
