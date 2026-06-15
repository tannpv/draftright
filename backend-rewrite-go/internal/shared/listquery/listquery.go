// Package listquery ports backend/src/common/list-query.ts — the query
// coercion + dynamic WHERE/ORDER/LIMIT builder shared by every admin list
// endpoint. Pure (no DB): Build returns SQL fragments + args the caller runs
// via pgxpool. The allow-listed sort map is the SQL-injection guard.
package listquery

import "strings"

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
