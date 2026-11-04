package liquids

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/fault"
	"github.com/Buffer379x/puffolio/internal/fields"
	"strings"
)

type Filter struct {
	Op         string          `json:"op,omitempty"`
	Conditions []Filter        `json:"conditions,omitempty"`
	FieldID    string          `json:"fieldId,omitempty"`
	Operator   string          `json:"operator,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	Value2     json.RawMessage `json:"value2,omitempty"`
}
type Query struct {
	Search, Sort, Direction string
	Favorite, Trash         bool
	Page, PageSize          int
	Filters                 *Filter
	Shelf                   *ShelfRules
}
type Page struct {
	Items    []Liquid `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

func EscapeLike(s string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(s)
}
func List(ctx context.Context, db *sql.DB, cid string, q Query) (Page, error) {
	p := Page{Items: []Liquid{}, Page: q.Page, PageSize: q.PageSize}
	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 24
	}
	if p.Page < 1 || p.Page > 1000000 || p.PageSize < 1 || p.PageSize > 100 {
		return p, fault.Validation("Ungültige Seitengröße")
	}
	defs, e := fields.List(ctx, db, cid)
	if e != nil {
		return p, e
	}
	dm := map[string]fields.Definition{}
	for _, d := range defs {
		dm[d.ID] = d
	}
	where := "l.collection_id=?"
	args := []any{cid}
	if q.Trash {
		where += " AND l.deleted_at IS NOT NULL"
	} else {
		where += " AND l.deleted_at IS NULL"
	}
	if q.Favorite {
		where += " AND l.favorite=1"
	}
	if len(q.Search) > 1000 {
		return p, fault.Validation("Suchtext zu lang")
	}
	if q.Search != "" {
		term := "%" + EscapeLike(fields.Normalize(q.Search)) + "%"
		where += " AND (l.normalized_name LIKE ? ESCAPE '\\' OR EXISTS(SELECT 1 FROM field_values sv JOIN field_definitions sf ON sf.id=sv.field_id AND sf.collection_id=sv.collection_id WHERE sv.collection_id=l.collection_id AND sv.liquid_id=l.id AND json_extract(sf.data,'$.config.searchable')=1 AND json_extract(sf.data,'$.archived')=0 AND sv.value_search LIKE ? ESCAPE '\\') OR EXISTS(SELECT 1 FROM liquid_tags st JOIN tags t ON t.collection_id=st.collection_id AND t.id=st.tag_id JOIN field_definitions sf ON sf.id=st.field_id AND sf.collection_id=st.collection_id WHERE st.collection_id=l.collection_id AND st.liquid_id=l.id AND json_extract(sf.data,'$.config.searchable')=1 AND json_extract(sf.data,'$.archived')=0 AND t.normalized_name LIKE ? ESCAPE '\\'))"
		args = append(args, term, term, term)
	}
	if q.Filters != nil {
		clause, values, e := BuildFilter(*q.Filters, dm)
		if e != nil {
			return p, e
		}
		where += " AND (" + clause + ")"
		args = append(args, values...)
	}
	sort := q.Sort
	if sort == "" {
		sort = "createdAt"
	}
	expr := ""
	orderArgs := []any{}
	switch sort {
	case "name":
		expr = germanSort("l.normalized_name")
	case "createdAt":
		expr = "l.created_at"
	case "updatedAt":
		expr = "l.updated_at"
	case "favorite":
		expr = "l.favorite"
	case "shelf":
		expr = "NULL"
		if q.Shelf != nil {
			expr, orderArgs = q.Shelf.RemainingSQL()
		}
	default:
		if !strings.HasPrefix(sort, "field:") {
			return p, fault.Validation("Ungültige Sortierung")
		}
		fid := strings.TrimPrefix(sort, "field:")
		d, ok := dm[fid]
		if !ok || d.Archived {
			return p, fault.Validation("Sortierfeld fehlt oder archiviert")
		}
		column := ""
		switch d.Type {
		case "number", "rating":
			column = "value_number"
		case "text":
			column = "value_search"
		case "date":
			column = "value_text"
		case "boolean":
			column = "value_bool"
		default:
			return p, fault.Validation("Feldtyp nicht sortierbar")
		}
		expr = "(SELECT " + column + " FROM field_values ov WHERE ov.collection_id=l.collection_id AND ov.liquid_id=l.id AND ov.field_id=?)"
		if d.Type == "text" {
			expr = germanSort(expr)
		}
		orderArgs = append(orderArgs, fid)
	}
	direction := strings.ToUpper(q.Direction)
	if direction == "" {
		direction = "DESC"
	}
	if direction != "ASC" && direction != "DESC" {
		return p, fault.Validation("Ungültige Sortierrichtung")
	}
	if e = db.QueryRowContext(ctx, "SELECT count(*) FROM liquids l WHERE "+where, args...).Scan(&p.Total); e != nil {
		return p, e
	}
	allargs := append([]any{}, args...)
	allargs = append(allargs, orderArgs...)
	allargs = append(allargs, orderArgs...)
	allargs = append(allargs, p.PageSize, (p.Page-1)*p.PageSize)
	rows, e := db.QueryContext(ctx, "SELECT l.id FROM liquids l WHERE "+where+" ORDER BY "+expr+" IS NULL ASC,"+expr+" "+direction+",l.id ASC LIMIT ? OFFSET ?", allargs...)
	if e != nil {
		return p, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return p, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return p, e
	}
	for _, id := range ids {
		l, e := Get(ctx, db, cid, id)
		if e != nil {
			return p, e
		}
		p.Items = append(p.Items, l)
	}
	q.Shelf.Apply(p.Items)
	return p, nil
}
func BuildFilter(f Filter, defs map[string]fields.Definition) (string, []any, error) {
	leaves := 0
	return buildFilter(f, defs, 1, &leaves)
}
func buildFilter(f Filter, defs map[string]fields.Definition, depth int, leaves *int) (string, []any, error) {
	bad := func() (string, []any, error) { return "", nil, fault.Validation("Ungültiger Filter") }
	if depth > 3 {
		return bad()
	}
	if f.Op != "" {
		if (f.Op != "and" && f.Op != "or") || len(f.Conditions) == 0 || f.FieldID != "" || len(f.Conditions) > 20 {
			return bad()
		}
		parts := []string{}
		args := []any{}
		for _, child := range f.Conditions {
			s, a, e := buildFilter(child, defs, depth+1, leaves)
			if e != nil {
				return "", nil, e
			}
			parts = append(parts, "("+s+")")
			args = append(args, a...)
		}
		return strings.Join(parts, " "+strings.ToUpper(f.Op)+" "), args, nil
	}
	*leaves++
	if *leaves > 20 {
		return bad()
	}
	if f.FieldID == "favorite" {
		var b bool
		if f.Operator != "eq" || len(f.Conditions) > 0 || string(f.Value) == "null" || json.Unmarshal(f.Value, &b) != nil {
			return bad()
		}
		return "l.favorite=?", []any{b}, nil
	}
	d, ok := defs[f.FieldID]
	if !ok || d.Archived {
		return bad()
	}
	if len(f.Conditions) > 0 {
		return bad()
	}
	prefix := "EXISTS(SELECT 1 FROM field_values fv WHERE fv.collection_id=l.collection_id AND fv.liquid_id=l.id AND fv.field_id=?"
	args := []any{f.FieldID}
	switch d.Type {
	case "date", "number", "rating":
		// Range indexes can repeatedly scan a field for each outer COUNT row.
		// The existing PK binds owner, liquid and field to at most one value.
		prefix = "EXISTS(SELECT 1 FROM field_values fv INDEXED BY sqlite_autoindex_field_values_1 WHERE fv.collection_id=l.collection_id AND fv.liquid_id=l.id AND fv.field_id=?"
	case "tags":
		prefix = "EXISTS(SELECT 1 FROM liquid_tags fv WHERE fv.collection_id=l.collection_id AND fv.liquid_id=l.id AND fv.field_id=?"
	}
	if f.Operator == "empty" {
		return "NOT " + prefix + ")", args, nil
	}
	if f.Operator == "notEmpty" {
		return prefix + ")", args, nil
	}
	if d.Type == "tags" || d.Type == "select" && f.Operator == "any" {
		if f.Operator != "any" && f.Operator != "all" && f.Operator != "none" {
			return bad()
		}
		var ids []string
		if json.Unmarshal(f.Value, &ids) != nil || len(ids) < 1 || len(ids) > 200 {
			return bad()
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !fields.UUID(id) || seen[id] {
				return bad()
			}
			seen[id] = true
		}
		col := "tag_id"
		if d.Type == "select" {
			col = "value_option"
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		for _, id := range ids {
			args = append(args, id)
		}
		if f.Operator == "all" {
			return "(SELECT count(*) FROM liquid_tags fv WHERE fv.collection_id=l.collection_id AND fv.liquid_id=l.id AND fv.field_id=? AND fv.tag_id IN (" + marks + "))=?", append(args, len(ids)), nil
		}
		clause := prefix + " AND fv." + col + " IN (" + marks + "))"
		if f.Operator == "none" {
			clause = "NOT " + clause
		}
		return clause, args, nil
	}
	column := ""
	switch d.Type {
	case "text", "longtext", "color", "date":
		column = "value_text"
	case "number", "rating":
		column = "value_number"
	case "boolean":
		column = "value_bool"
	case "select":
		column = "value_option"
	default:
		return bad()
	}
	op := f.Operator
	allowed := false
	switch d.Type {
	case "text", "longtext":
		allowed = op == "eq" || op == "contains"
	case "number", "rating":
		allowed = op == "eq" || op == "gte" || op == "lte" || op == "between"
	case "date":
		allowed = op == "eq" || op == "before" || op == "after" || op == "between"
	default:
		allowed = op == "eq"
	}
	if !allowed {
		return bad()
	}
	parse := func(raw json.RawMessage) (any, error) {
		if d.Type == "number" {
			var s string
			if json.Unmarshal(raw, &s) != nil {
				return nil, fmt.Errorf("number string required")
			}
			return fields.ParseNumber(s, d.Config.Scale)
		}
		if d.Type == "rating" {
			var n int64
			e := json.Unmarshal(raw, &n)
			return n, e
		}
		v, e := fields.ValidateValue(d, raw)
		if e != nil {
			return nil, e
		}
		if v.Text != nil {
			return *v.Text, nil
		}
		if v.Boolean != nil {
			return *v.Boolean, nil
		}
		if v.Option != nil {
			return *v.Option, nil
		}
		return nil, fmt.Errorf("missing filter value")
	}
	v, e := parse(f.Value)
	if e != nil {
		return bad()
	}
	args = append(args, v)
	if op == "contains" {
		s, ok := v.(string)
		if !ok {
			return bad()
		}
		args[len(args)-1] = "%" + EscapeLike(fields.Normalize(s)) + "%"
		return prefix + " AND fv.value_search LIKE ? ESCAPE '\\')", args, nil
	}
	sqlop := map[string]string{"eq": "=", "gte": ">=", "lte": "<=", "before": "<", "after": ">"}[op]
	if op == "between" {
		v2, e := parse(f.Value2)
		if e != nil {
			return bad()
		}
		args = append(args, v2)
		return prefix + " AND fv." + column + " BETWEEN ? AND ?)", args, nil
	}
	return prefix + " AND fv." + column + sqlop + " ?)", args, nil
}

// ValidateFilterShape bounds stored expressions even when their field references
// have been archived or deleted and can only be reported at query time.
func ValidateFilterShape(f Filter) error {
	leaves := 0
	var walk func(Filter, int) error
	walk = func(n Filter, depth int) error {
		if depth > 3 {
			return fault.Validation("Filter maximal drei Ebenen")
		}
		if n.Op != "" {
			if n.Op != "and" && n.Op != "or" || len(n.Conditions) == 0 || len(n.Conditions) > 20 || n.FieldID != "" || n.Operator != "" {
				return fault.Validation("Ungültige Filtergruppe")
			}
			for _, child := range n.Conditions {
				if e := walk(child, depth+1); e != nil {
					return e
				}
			}
			return nil
		}
		leaves++
		if leaves > 20 || n.FieldID == "" || len(n.Conditions) > 0 {
			return fault.Validation("Filter maximal 20 Bedingungen")
		}
		switch n.Operator {
		case "eq", "contains", "empty", "notEmpty", "gte", "lte", "before", "after", "between", "any", "all", "none":
			return nil
		default:
			return fault.Validation("Unbekannter Filteroperator")
		}
	}
	return walk(f, 1)
}

// germanSort orders umlauts beside their base letter. Case-folded text compares
// by code point, which would otherwise file "Äpfel" after "Zitrone".
func germanSort(column string) string {
	for _, pair := range [][2]string{{"ä", "a"}, {"ö", "o"}, {"ü", "u"}, {"ß", "ss"}} {
		column = "replace(" + column + ",'" + pair[0] + "','" + pair[1] + "')"
	}
	return column
}
