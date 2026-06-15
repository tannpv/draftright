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
