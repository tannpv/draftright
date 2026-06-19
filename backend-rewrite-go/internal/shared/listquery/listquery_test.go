package listquery

import (
	"net/url"
	"reflect"
	"testing"
)

func intp(i int) *int { return &i }

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Query
	}{
		{"empty", "", Query{}},
		{"search", "search=hi", Query{Search: "hi"}},
		{"status active", "status=active", Query{Status: "active"}},
		{"status bogus dropped", "status=bogus", Query{}},
		{"sort_order lowercased", "sort_order=asc", Query{SortOrder: "ASC"}},
		{"sort_order bogus dropped", "sort_order=sideways", Query{}},
		{"sort_by", "sort_by=name", Query{SortBy: "name"}},
		{"page+limit", "page=3&limit=25", Query{Page: intp(3), Limit: intp(25)}},
		{"page jsParseInt 12abc", "page=12abc", Query{Page: intp(12)}},
		{"page non-numeric absent", "page=abc", Query{}},
		{"page float truncates", "limit=2.5", Query{Limit: intp(2)}},
		{"page negative", "page=-5", Query{Page: intp(-5)}},
		{"page empty absent", "page=", Query{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := url.ParseQuery(tt.raw)
			got := Parse(v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestJsParseInt(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"12", 12, true}, {"12abc", 12, true}, {"  3", 3, true},
		{"2.5", 2, true}, {"-5", -5, true}, {"+7", 7, true},
		{"abc", 0, false}, {"", 0, false}, {"-", 0, false}, {"   ", 0, false},
	}
	for _, tt := range tests {
		got, ok := jsParseInt(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("jsParseInt(%q) = (%d,%v), want (%d,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBuild(t *testing.T) {
	cols := []string{"u.email", "u.name"}
	allow := map[string]string{"name": "u.name", "created": "u.created_at"}

	t.Run("defaults: no search/status/sort", func(t *testing.T) {
		b := Build(Query{}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "" {
			t.Errorf("Where = %q, want empty", b.Where)
		}
		if len(b.Args) != 0 {
			t.Errorf("Args = %v, want none", b.Args)
		}
		if b.Order != "ORDER BY u.created_at DESC" {
			t.Errorf("Order = %q", b.Order)
		}
		if b.Limit != 10 || b.Offset != 0 {
			t.Errorf("Limit=%d Offset=%d, want 10/0", b.Limit, b.Offset)
		}
	})

	t.Run("search OR-clause, single arg", func(t *testing.T) {
		b := Build(Query{Search: "  bob  "}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "WHERE (u.email ILIKE $1 OR u.name ILIKE $1)" {
			t.Errorf("Where = %q", b.Where)
		}
		if len(b.Args) != 1 || b.Args[0] != "%bob%" {
			t.Errorf("Args = %v, want [%%bob%%]", b.Args)
		}
	})

	t.Run("status active → is_active true", func(t *testing.T) {
		b := Build(Query{Status: "active"}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "WHERE u.is_active = $1" {
			t.Errorf("Where = %q", b.Where)
		}
		if len(b.Args) != 1 || b.Args[0] != true {
			t.Errorf("Args = %v, want [true]", b.Args)
		}
	})

	t.Run("status all → no filter", func(t *testing.T) {
		b := Build(Query{Status: "all"}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "" {
			t.Errorf("Where = %q, want empty", b.Where)
		}
	})

	t.Run("search + status combine, placeholders increment", func(t *testing.T) {
		b := Build(Query{Search: "x", Status: "inactive"}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "WHERE (u.email ILIKE $1 OR u.name ILIKE $1) AND u.is_active = $2" {
			t.Errorf("Where = %q", b.Where)
		}
		if len(b.Args) != 2 || b.Args[0] != "%x%" || b.Args[1] != false {
			t.Errorf("Args = %v", b.Args)
		}
	})

	t.Run("sort allow-list + ASC", func(t *testing.T) {
		b := Build(Query{SortBy: "name", SortOrder: "ASC"}, cols, allow, "u.created_at", "u.is_active")
		if b.Order != "ORDER BY u.name ASC" {
			t.Errorf("Order = %q", b.Order)
		}
	})

	t.Run("sort_by not in allow-list → default DESC", func(t *testing.T) {
		b := Build(Query{SortBy: "password; DROP TABLE", SortOrder: "ASC"}, cols, allow, "u.created_at", "u.is_active")
		if b.Order != "ORDER BY u.created_at ASC" {
			t.Errorf("Order = %q, injection must fall to default field (order still honored)", b.Order)
		}
	})

	t.Run("limit cap 100, page offset math", func(t *testing.T) {
		b := Build(Query{Page: intp(3), Limit: intp(250)}, cols, allow, "u.created_at", "u.is_active")
		if b.Limit != 100 {
			t.Errorf("Limit = %d, want 100", b.Limit)
		}
		if b.Offset != 200 {
			t.Errorf("Offset = %d, want (3-1)*100=200", b.Offset)
		}
	})

	t.Run("page/limit zero floor to 1", func(t *testing.T) {
		b := Build(Query{Page: intp(0), Limit: intp(0)}, cols, allow, "u.created_at", "u.is_active")
		if b.Limit != 1 || b.Offset != 0 {
			t.Errorf("Limit=%d Offset=%d, want 1/0", b.Limit, b.Offset)
		}
	})

	t.Run("status disabled when statusCol empty", func(t *testing.T) {
		b := Build(Query{Status: "active"}, cols, allow, "u.created_at", "")
		if b.Where != "" {
			t.Errorf("Where = %q, want empty when statusCol disabled", b.Where)
		}
	})
}
