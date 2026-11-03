package backup

import (
	"bytes"
	"context"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"github.com/Buffer379x/puffolio/internal/liquids"
	"os"
	"path/filepath"
	"testing"
)

func TestRequiredCutoffChronologyForLegacyEditAndExport(t *testing.T) {
	for _, tc := range []struct{ name, created, cutoff string }{{"whole before fraction", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00.1Z"}, {"short fraction before longer", "2026-01-01T00:00:00.1Z", "2026-01-01T00:00:00.100000001Z"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := reviewStore(t)
			u := reviewUser(t, s, "legacy-user")
			l := reviewLiquid(t, s, u.CollectionID, nil)
			s.DB.Exec("UPDATE liquids SET created_at=? WHERE id=?", tc.created, l.ID)
			reviewRequired(t, s, u.CollectionID, "Later required", tc.cutoff)
			name := "Editable legacy"
			if _, e := liquids.Save(context.Background(), s.DB, u.CollectionID, l.ID, liquids.Input{Name: &name, Version: l.Version}, nil); e != nil {
				t.Error("chronologically older liquid was blocked", e)
			}
			var archive bytes.Buffer
			if e := Export(context.Background(), s, u.CollectionID, &archive, "test"); e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(t.TempDir(), "legacy.zip")
			os.WriteFile(p, archive.Bytes(), 0600)
			if _, e := Preview(p, WebLimits()); e != nil {
				t.Error("valid legacy export rejected by cutoff", e)
			}
		})
	}
}
func TestEquivalentFractionPrecisionStillRequiresExistingValue(t *testing.T) {
	s := reviewStore(t)
	u := reviewUser(t, s, "equal-user")
	d := reviewRequired(t, s, u.CollectionID, "Required", "2026-01-01T00:00:00.1Z")
	l := reviewLiquid(t, s, u.CollectionID, &d)
	s.DB.Exec("UPDATE liquids SET created_at=? WHERE id=?", "2026-01-01T00:00:00.100000000Z", l.ID)
	_, e := liquids.Save(context.Background(), s.DB, u.CollectionID, l.ID, liquids.Input{Version: l.Version, Values: map[string]*fields.Value{d.ID: nil}}, nil)
	if e == nil || fault.Wrap(e).Status != 422 {
		t.Fatalf("equal timestamps must retain required value, got %v", e)
	}
	reviewExport(t, s, u.CollectionID)
}
func TestImportAllowsChronologicallyExemptRequiredRules(t *testing.T) {
	for _, incomingRequired := range []bool{true, false} {
		name := "destination rule"
		if incomingRequired {
			name = "incoming rule"
		}
		t.Run(name, func(t *testing.T) {
			s := reviewStore(t)
			source := reviewUser(t, s, "source-user")
			target := reviewUser(t, s, "target-user")
			sl := reviewLiquid(t, s, source.CollectionID, nil)
			tl := reviewLiquid(t, s, target.CollectionID, nil)
			s.DB.Exec("UPDATE liquids SET created_at=? WHERE id IN (?,?)", "2026-01-01T00:00:00Z", sl.ID, tl.ID)
			cid := target.CollectionID
			if incomingRequired {
				cid = source.CollectionID
			}
			reviewRequired(t, s, cid, "New only", "2026-01-01T00:00:00.1Z")
			p := reviewExport(t, s, source.CollectionID)
			if _, e := PreviewForCollection(context.Background(), s, target.CollectionID, p, WebLimits()); e != nil {
				t.Fatal("compatible preview rejected", e)
			}
			if _, e := Import(context.Background(), s, target.CollectionID, p, WebLimits()); e != nil {
				t.Fatal("compatible import rejected", e)
			}
			page, e := liquids.List(context.Background(), s.DB, target.CollectionID, liquids.Query{Page: 1, PageSize: 24})
			if e != nil || len(page.Items) != 2 {
				t.Fatal(page, e)
			}
			for _, l := range page.Items {
				name := "Still legacy"
				if _, e = liquids.Save(context.Background(), s.DB, target.CollectionID, l.ID, liquids.Input{Name: &name, Version: l.Version}, nil); e != nil {
					t.Fatal("compatible liquid became uneditable", e)
				}
			}
			reviewExport(t, s, target.CollectionID)
		})
	}
}
func TestImportUsesLatestChronologicalRecordAcrossFractionPrecision(t *testing.T) {
	for _, incomingRequired := range []bool{true, false} {
		name := "destination cutoff"
		if incomingRequired {
			name = "incoming cutoff"
		}
		t.Run(name, func(t *testing.T) {
			s := reviewStore(t)
			source := reviewUser(t, s, "source-user")
			target := reviewUser(t, s, "target-user")
			if incomingRequired {
				d := reviewRequired(t, s, source.CollectionID, "Source required", "2026-01-01T00:00:00.05Z")
				l := reviewLiquid(t, s, source.CollectionID, &d)
				s.DB.Exec("UPDATE liquids SET created_at=? WHERE id=?", "2026-01-01T00:00:00.1Z", l.ID)
				for _, created := range []string{"2026-01-01T00:00:00Z", "2026-01-01T00:00:00.1Z"} {
					old := reviewLiquid(t, s, target.CollectionID, nil)
					s.DB.Exec("UPDATE liquids SET created_at=? WHERE id=?", created, old.ID)
				}
			} else {
				d := reviewRequired(t, s, target.CollectionID, "Target required", "2026-01-01T00:00:00.05Z")
				reviewLiquid(t, s, target.CollectionID, &d)
				for _, created := range []string{"2026-01-01T00:00:00Z", "2026-01-01T00:00:00.1Z"} {
					old := reviewLiquid(t, s, source.CollectionID, nil)
					s.DB.Exec("UPDATE liquids SET created_at=? WHERE id=?", created, old.ID)
				}
			}
			p := reviewExport(t, s, source.CollectionID)
			reviewRejectImport(t, s, target, p)
		})
	}
}
