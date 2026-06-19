package main

import "strings"

// substitute returns a copy of f with every {{key}} in headers and body
// replaced by vars[key]. Unknown placeholders are left untouched so a
// missing token surfaces as a real request error, not a silent empty value.
// The input fixture is never mutated (headers map is copied).
func substitute(f fixture, vars map[string]string) fixture {
	repl := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{{"+k+"}}", v)
		}
		return s
	}
	out := f
	if f.Headers != nil {
		out.Headers = make(map[string]string, len(f.Headers))
		for k, v := range f.Headers {
			out.Headers[k] = repl(v)
		}
	}
	if len(f.Body) > 0 {
		out.Body = []byte(repl(string(f.Body)))
	}
	return out
}
