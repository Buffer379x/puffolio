package liquids

import "encoding/json"

// WithoutTag returns the filter with a deleted tag taken out, and whether that changed it. A
// condition left without tags goes, and so does a group left without conditions. The given
// filter stays untouched. A malformed value is left alone: it is not this tag's to remove.
func WithoutTag(f *Filter, fieldID, tagID string) (*Filter, bool) {
	if f == nil {
		return nil, false
	}
	if f.Op != "" {
		kept, changed := make([]Filter, 0, len(f.Conditions)), false
		for i := range f.Conditions {
			next, c := WithoutTag(&f.Conditions[i], fieldID, tagID)
			changed = changed || c
			if next != nil {
				kept = append(kept, *next)
			}
		}
		if !changed {
			return f, false
		}
		if len(kept) == 0 {
			return nil, true
		}
		out := *f
		out.Conditions = kept
		return &out, true
	}
	if f.FieldID != fieldID || f.Operator != "any" && f.Operator != "all" && f.Operator != "none" {
		return f, false
	}
	var ids []string
	if json.Unmarshal(f.Value, &ids) != nil {
		return f, false
	}
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != tagID {
			kept = append(kept, id)
		}
	}
	if len(kept) == len(ids) {
		return f, false
	}
	if len(kept) == 0 {
		return nil, true
	}
	out := *f
	out.Value, _ = json.Marshal(kept)
	return &out, true
}
