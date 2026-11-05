package fields

import (
	"encoding/json"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Visibility struct {
	Form   bool `json:"form"`
	Detail bool `json:"detail"`
	List   bool `json:"list"`
	Grid   bool `json:"grid"`
}
type Option struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Archived  bool   `json:"archived,omitempty"`
	PaletteID string `json:"paletteId,omitempty"`
}
type Config struct {
	Scale        int             `json:"scale,omitempty"`
	Unit         string          `json:"unit,omitempty"`
	Min          string          `json:"min,omitempty"`
	Max          string          `json:"max,omitempty"`
	Stars        int             `json:"stars,omitempty"`
	Options      []Option        `json:"options,omitempty"`
	DefaultValue json.RawMessage `json:"defaultValue,omitempty"`
	DefaultToday bool            `json:"defaultToday,omitempty"`
	Searchable   bool            `json:"searchable,omitempty"`
	Filterable   bool            `json:"filterable,omitempty"`
}
type Definition struct {
	ID           string     `json:"id"`
	Label        string     `json:"label"`
	Type         string     `json:"type"`
	Position     int        `json:"position"`
	Version      int        `json:"version"`
	Required     bool       `json:"required"`
	RequiredFrom string     `json:"requiredFrom,omitempty"`
	Archived     bool       `json:"archived"`
	Visibility   Visibility `json:"visibility"`
	Config       Config     `json:"config"`
}
type Value struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}
type Validated struct {
	Text    *string
	Number  *int64
	Boolean *bool
	Media   *string
	Option  *string
	Tags    []string
	Raw     json.RawMessage
}
type Tag struct {
	ID        string `json:"id"`
	FieldID   string `json:"fieldId"`
	Name      string `json:"name"`
	PaletteID string `json:"paletteId"`
	Version   int    `json:"version"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func UUID(s string) bool        { return uuidPattern.MatchString(s) }
func Normalize(s string) string { return cases.Fold().String(norm.NFC.String(strings.TrimSpace(s))) }

// PaletteIDs are the tag colours; migrations/002_palette.sql allows the same ones.
var PaletteIDs = []string{
	"red", "coral", "apricot", "tangerine", "orange", "honey", "sand", "amber",
	"olive", "pistachio", "lime", "moss", "green", "mint", "jade", "teal",
	"aqua", "cyan", "ice", "sky", "blue", "cornflower", "periwinkle", "indigo",
	"lavender", "violet", "orchid", "pink", "magenta", "rose", "raspberry", "slate",
}

func ValidPalette(s string) bool { return slices.Contains(PaletteIDs, s) }
func ParseNumber(s string, scale int) (int64, error) {
	if scale < 0 || scale > 3 || !decimalPattern.MatchString(s) {
		return 0, fmt.Errorf("Ungültige Zahl")
	}
	parts := strings.SplitN(s, ".", 2)
	fraction := ""
	if len(parts) > 1 {
		fraction = parts[1]
	}
	if len(fraction) > scale {
		return 0, fmt.Errorf("Zu viele Nachkommastellen")
	}
	digits := parts[0] + fraction + strings.Repeat("0", scale-len(fraction))
	return strconv.ParseInt(digits, 10, 64)
}
func FormatNumber(n int64, scale int) string {
	s := strconv.FormatInt(n, 10)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for len(s) <= scale {
		s = "0" + s
	}
	if scale > 0 {
		s = s[:len(s)-scale] + "." + s[len(s)-scale:]
	}
	if negative {
		s = "-" + s
	}
	return s
}
func ValidateDefinition(d Definition) error {
	if strings.TrimSpace(d.Label) == "" || utf8.RuneCountInString(d.Label) > 100 {
		return fmt.Errorf("Feldname erforderlich (maximal 100 Zeichen)")
	}
	if d.Required && !d.Visibility.Form && !d.Archived {
		return fmt.Errorf("Pflichtfelder müssen im Formular sichtbar sein")
	}
	switch d.Type {
	case "text", "longtext", "image", "tags", "color", "boolean":
	case "rating":
		if d.Config.Stars != 3 && d.Config.Stars != 5 && d.Config.Stars != 10 {
			return fmt.Errorf("Sterneskala muss 3, 5 oder 10 sein")
		}
	case "number":
		if d.Config.Scale < 0 || d.Config.Scale > 3 {
			return fmt.Errorf("Skala muss 0 bis 3 sein")
		}
		for _, s := range []string{d.Config.Min, d.Config.Max} {
			if s != "" {
				if _, e := ParseNumber(s, d.Config.Scale); e != nil {
					return e
				}
			}
		}
		if d.Config.Min != "" && d.Config.Max != "" {
			a, _ := ParseNumber(d.Config.Min, d.Config.Scale)
			b, _ := ParseNumber(d.Config.Max, d.Config.Scale)
			if a > b {
				return fmt.Errorf("Minimum größer als Maximum")
			}
		}
	case "date":
		for _, s := range []string{d.Config.Min, d.Config.Max} {
			if s != "" {
				if _, e := time.Parse("2006-01-02", s); e != nil {
					return fmt.Errorf("Ungültige Datumsgrenze")
				}
			}
		}
		if d.Config.Min != "" && d.Config.Max != "" && d.Config.Min > d.Config.Max {
			return fmt.Errorf("Ungültiger Datumsbereich")
		}
	case "select":
		seen := map[string]bool{}
		if len(d.Config.Options) > 200 {
			return fmt.Errorf("Maximal 200 Optionen")
		}
		for _, o := range d.Config.Options {
			if !UUID(o.ID) || strings.TrimSpace(o.Label) == "" || seen[o.ID] || o.PaletteID != "" && !ValidPalette(o.PaletteID) {
				return fmt.Errorf("Ungültige Auswahloption")
			}
			seen[o.ID] = true
		}
	default:
		return fmt.Errorf("Unbekannter Feldtyp")
	}
	if d.Config.DefaultToday && d.Type != "date" {
		return fmt.Errorf("Heute nur für Datum")
	}
	if len(d.Config.DefaultValue) > 0 && string(d.Config.DefaultValue) != "null" {
		_, e := ValidateValue(d, d.Config.DefaultValue)
		return e
	}
	return nil
}
func ValidateValue(d Definition, raw json.RawMessage) (v Validated, err error) {
	v.Raw = raw
	if len(raw) == 0 || string(raw) == "null" {
		return v, nil
	}
	fail := func() (Validated, error) { return Validated{}, fmt.Errorf("Ungültiger Wert für %s", d.Label) }
	switch d.Type {
	case "text", "longtext", "color", "date", "image", "select", "number":
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return fail()
		}
		switch d.Type {
		case "text":
			s = strings.TrimSpace(s)
			if utf8.RuneCountInString(s) > 500 {
				return fail()
			}
			if s == "" {
				return Validated{}, nil
			}
			v.Text = &s
		case "longtext":
			if utf8.RuneCountInString(s) > 10000 {
				return fail()
			}
			if s == "" {
				return Validated{}, nil
			}
			v.Text = &s
		case "color":
			if !colorPattern.MatchString(s) {
				return fail()
			}
			s = strings.ToUpper(s)
			v.Text = &s
		case "date":
			if len(s) != 10 {
				return fail()
			}
			if _, e := time.Parse("2006-01-02", s); e != nil {
				return fail()
			}
			if d.Config.Min != "" && s < d.Config.Min || d.Config.Max != "" && s > d.Config.Max {
				return fail()
			}
			v.Text = &s
		case "image":
			if !UUID(s) {
				return fail()
			}
			v.Media = &s
		case "select":
			found := false
			for _, o := range d.Config.Options {
				if o.ID == s && !o.Archived {
					found = true
				}
			}
			if !found {
				return fail()
			}
			v.Option = &s
		case "number":
			n, e := ParseNumber(s, d.Config.Scale)
			if e != nil {
				return fail()
			}
			if d.Config.Min != "" {
				m, _ := ParseNumber(d.Config.Min, d.Config.Scale)
				if n < m {
					return fail()
				}
			}
			if d.Config.Max != "" {
				m, _ := ParseNumber(d.Config.Max, d.Config.Scale)
				if n > m {
					return fail()
				}
			}
			v.Number = &n
			s = FormatNumber(n, d.Config.Scale)
		}
		v.Raw, _ = json.Marshal(s)
	case "rating":
		var n int64
		if json.Unmarshal(raw, &n) != nil {
			return fail()
		}
		stars := d.Config.Stars
		if stars == 0 {
			stars = 5
		}
		if n < 1 || n > int64(stars) {
			return fail()
		}
		v.Number = &n
	case "boolean":
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return fail()
		}
		v.Boolean = &b
	case "tags":
		var ids []string
		if json.Unmarshal(raw, &ids) != nil || len(ids) > 200 {
			return fail()
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !UUID(id) || seen[id] {
				return fail()
			}
			seen[id] = true
		}
		v.Tags = ids
		if len(ids) == 0 {
			return Validated{}, nil
		}
	default:
		return fail()
	}
	return v, nil
}
