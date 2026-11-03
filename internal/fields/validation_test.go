package fields

import (
	"encoding/json"
	"testing"
)

func TestAllTypesAndRanges(t *testing.T) {
	cases := []struct {
		typ    string
		config Config
		raw    string
		valid  bool
	}{
		{"text", Config{}, `"Öl"`, true}, {"longtext", Config{}, `"a\nb"`, true}, {"rating", Config{Stars: 5}, `0`, false}, {"rating", Config{Stars: 5}, `5`, true},
		{"number", Config{Scale: 1, Min: "0", Max: "10"}, `"0.0"`, true}, {"number", Config{Scale: 1}, `"1.23"`, false}, {"number", Config{Scale: 0}, `"9223372036854775808"`, false},
		{"boolean", Config{}, `false`, true}, {"color", Config{}, `"#aAbB00"`, true}, {"color", Config{}, `"red"`, false}, {"date", Config{}, `"2028-02-29"`, true}, {"date", Config{}, `"2027-02-29"`, false},
		{"image", Config{}, `"01900000-0000-7000-8000-000000000001"`, true}, {"tags", Config{}, `["01900000-0000-7000-8000-000000000001","01900000-0000-7000-8000-000000000001"]`, false},
		{"select", Config{Options: []Option{{ID: "01900000-0000-7000-8000-000000000001", Label: "A"}}}, `"01900000-0000-7000-8000-000000000001"`, true},
	}
	for _, c := range cases {
		t.Run(c.typ+c.raw, func(t *testing.T) {
			_, e := ValidateValue(Definition{Type: c.typ, Config: c.config}, json.RawMessage(c.raw))
			if (e == nil) != c.valid {
				t.Fatalf("valid=%v err=%v", c.valid, e)
			}
		})
	}
}
func TestTagPaletteRejectsUnknownID(t *testing.T) {
	if ValidPalette("#ffffff") || !ValidPalette("lime") {
		t.Fatal("invalid palette acceptance")
	}
}
func TestScaledNumbersExact(t *testing.T) {
	n, e := ParseNumber("12.5", 1)
	if e != nil || n != 125 {
		t.Fatal(n, e)
	}
	if FormatNumber(-5, 2) != "-0.05" {
		t.Fatal("negative decimals lost")
	}
}
