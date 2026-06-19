// Package listquery ports backend/src/common/list-query.ts — the query
// coercion + dynamic WHERE/ORDER/LIMIT builder shared by every admin list
// endpoint. Pure (no DB): Build returns SQL fragments + args the caller runs
// via pgxpool. The allow-listed sort map is the SQL-injection guard.
package listquery

import (
	"fmt"
	"strings"
)

// Query is the coerced shape of the admin frontend's list params. Page/Limit
// are *int so "absent" is distinct from a parsed 0 (matches Node's
// undefined-vs-0 distinction before the `|| 1` / `|| 10` defaults).
type Query struct {
	Search    string // "" when absent
	Status    string // "active" | "inactive" | "all" | "" (unset)
	SortBy    string
	SortOrder string // "ASC" | "DESC" | "" (unset)
	Page      *int
	Limit     *int
}

// Parse mirrors parseListQuery: status restricted to the allow-list,
// sort_order uppercased then allow-listed, page/limit via jsParseInt.
func Parse(v map[string][]string) Query {
	get := func(k string) string {
		if vs := v[k]; len(vs) > 0 {
			return vs[0]
		}
		return ""
	}
	var q Query
	q.Search = get("search")
	if s := get("status"); s == "active" || s == "inactive" || s == "all" {
		q.Status = s
	}
	q.SortBy = get("sort_by")
	if so := strings.ToUpper(get("sort_order")); so == "ASC" || so == "DESC" {
		q.SortOrder = so
	}
	if n, ok := jsParseInt(get("page")); ok {
		q.Page = &n
	}
	if n, ok := jsParseInt(get("limit")); ok {
		q.Limit = &n
	}
	return q
}

// Built is the SQL fragments Build produces. The caller composes:
//
//	rows : SELECT <cols> FROM <base> {Where} {Order} LIMIT {Limit} OFFSET {Offset}
//	count: SELECT count(*) FROM <base> {Where}
//
// both with Args. Where is "" or starts with "WHERE "; Order always set.
type Built struct {
	Where  string
	Args   []any
	Order  string
	Limit  int
	Offset int
}

// Build mirrors applyListQuery. searchCols + sortAllow values are
// caller-supplied literals (alias.field), never user input. statusCol == ""
// disables the status filter (Node's statusColumn = null).
func Build(q Query, searchCols []string, sortAllow map[string]string, defaultSort, statusCol string) Built {
	var clauses []string
	var args []any

	if term := strings.TrimSpace(q.Search); term != "" && len(searchCols) > 0 {
		args = append(args, "%"+term+"%")
		ph := fmt.Sprintf("$%d", len(args))
		ors := make([]string, len(searchCols))
		for i, c := range searchCols {
			ors[i] = c + " ILIKE " + ph
		}
		clauses = append(clauses, "("+strings.Join(ors, " OR ")+")")
	}

	if statusCol != "" && q.Status != "" && q.Status != "all" {
		args = append(args, q.Status == "active")
		clauses = append(clauses, fmt.Sprintf("%s = $%d", statusCol, len(args)))
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	field := defaultSort
	if mapped := sortAllow[q.SortBy]; mapped != "" {
		field = mapped
	}
	order := "DESC"
	if q.SortOrder == "ASC" {
		order = "ASC"
	}

	page := 1
	if q.Page != nil && *q.Page > 1 {
		page = *q.Page
	}
	limit := 10
	if q.Limit != nil {
		limit = *q.Limit
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	return Built{
		Where:  where,
		Args:   args,
		Order:  "ORDER BY " + field + " " + order,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

// JSParseInt exposes jsParseInt for bespoke (non-listquery) admin handlers
// that mirror Node's parseInt(). Same semantics as listquery's page/limit parse.
func JSParseInt(s string) (int, bool) { return jsParseInt(s) }

// jsParseInt replicates JavaScript parseInt(s, 10): skip leading ASCII
// whitespace, accept one optional +/- sign, consume base-10 digits, and STOP
// at the first non-digit (so "12abc"→12). Returns ok=false when no digit is
// consumed (JS returns NaN, which the caller treats as absent).
func jsParseInt(s string) (int, bool) {
	i, n := 0, len(s)
	for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	neg := false
	if i < n && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	start := i
	val := 0
	for i < n && s[i] >= '0' && s[i] <= '9' {
		val = val*10 + int(s[i]-'0')
		i++
	}
	if i == start {
		return 0, false
	}
	if neg {
		val = -val
	}
	return val, true
}
