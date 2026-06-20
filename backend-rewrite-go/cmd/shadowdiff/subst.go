package main

import "strings"

// substitute returns a copy of f with every {{key}} in the path, headers and
// body replaced by vars[key]. Unknown placeholders are left untouched so a
// missing token surfaces as a real request error, not a silent empty value.
// The input fixture is never mutated (headers map is copied). Path
// substitution lets a fixture target a bootstrap-derived id (e.g.
// /admin/admin-users/{{admin_id}} for the #32 self-delete guard).
func substitute(f fixture, vars map[string]string) fixture {
	repl := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{{"+k+"}}", v)
		}
		return s
	}
	out := f
	out.Path = repl(f.Path)
	if f.Headers != nil {
		out.Headers = make(map[string]string, len(f.Headers))
		for k, v := range f.Headers {
			out.Headers[k] = repl(v)
		}
	}
	if len(f.Body) > 0 {
		out.Body = []byte(repl(string(f.Body)))
	}
	if f.RawBody != "" {
		out.RawBody = repl(f.RawBody)
	}
	return out
}
