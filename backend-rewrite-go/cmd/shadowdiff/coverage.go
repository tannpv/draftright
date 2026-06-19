package main

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
)

// loadRoutes reads "METHOD /path" lines (blank lines + #comments ignored) from
// the canonical route inventory file.
func loadRoutes(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

// routeToRegexp turns "GET /admin/users/{id}" into a matcher that accepts any
// concrete segment in place of {id}.
func routeToRegexp(route string) *regexp.Regexp {
	parts := strings.SplitN(route, " ", 2)
	method, path := parts[0], parts[1]
	esc := regexp.QuoteMeta(path)
	esc = regexp.MustCompile(`\\\{[^/}]+\\\}`).ReplaceAllString(esc, `[^/]+`)
	return regexp.MustCompile("^" + method + " " + esc + "$")
}

// missingRoutes returns the canonical routes that no fixture exercises. A
// fixture's "METHOD /concrete/path" must match a route template (params allowed).
func missingRoutes(routes []string, fixtures []fixture) []string {
	var missing []string
	for _, route := range routes {
		re := routeToRegexp(route)
		covered := false
		for _, f := range fixtures {
			if re.MatchString(f.Method + " " + f.Path) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	return missing
}
