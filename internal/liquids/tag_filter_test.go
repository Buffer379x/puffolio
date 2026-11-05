package liquids

import (
	"encoding/json"
	"testing"
)

func TestWithoutTag(t *testing.T) {
	tagged := func(field, operator string, ids ...string) Filter {
		raw, _ := json.Marshal(ids)
		return Filter{FieldID: field, Operator: operator, Value: raw}
	}
	group := func(op string, conditions ...Filter) Filter {
		return Filter{Op: op, Conditions: conditions}
	}
	other := Filter{FieldID: "f", Operator: "notEmpty"}
	for name, c := range map[string]struct {
		in      Filter
		want    *Filter
		changed bool
	}{
		"any keeps the others":      {tagged("f", "any", "a", "b"), ptr(tagged("f", "any", "b")), true},
		"all keeps the others":      {tagged("f", "all", "a", "b"), ptr(tagged("f", "all", "b")), true},
		"none keeps the others":     {tagged("f", "none", "a", "b"), ptr(tagged("f", "none", "b")), true},
		"last id drops condition":   {tagged("f", "any", "a"), nil, true},
		"tag not in the condition":  {tagged("f", "any", "b"), ptr(tagged("f", "any", "b")), false},
		"another field":             {tagged("g", "any", "a"), ptr(tagged("g", "any", "a")), false},
		"operator without ids":      {other, &other, false},
		"group keeps the rest":      {group("and", tagged("f", "any", "a"), other), ptr(group("and", other)), true},
		"emptied group goes":        {group("or", tagged("f", "any", "a")), nil, true},
		"nested group":              {group("and", group("or", tagged("f", "any", "a", "b"), other)), ptr(group("and", group("or", tagged("f", "any", "b"), other))), true},
		"nested emptied group goes": {group("and", other, group("or", tagged("f", "all", "a"))), ptr(group("and", other)), true},
		"group without the tag":     {group("and", other), ptr(group("and", other)), false},
	} {
		t.Run(name, func(t *testing.T) {
			in, _ := json.Marshal(c.in)
			got, changed := WithoutTag(&c.in, "f", "a")
			if changed != c.changed {
				t.Fatalf("changed = %v", changed)
			}
			if a, b := mustJSON(got), mustJSON(c.want); a != b {
				t.Fatalf("got %s, want %s", a, b)
			}
			if after, _ := json.Marshal(c.in); string(after) != string(in) {
				t.Fatalf("input changed to %s", after)
			}
		})
	}
	if got, changed := WithoutTag(nil, "f", "a"); got != nil || changed {
		t.Fatal("no filter stays no filter")
	}
	broken := Filter{FieldID: "f", Operator: "any", Value: json.RawMessage(`"a"`)}
	if _, changed := WithoutTag(&broken, "f", "a"); changed {
		t.Fatal("a malformed value is left alone")
	}
}
func ptr(f Filter) *Filter { return &f }
func mustJSON(f *Filter) string {
	raw, _ := json.Marshal(f)
	return string(raw)
}
