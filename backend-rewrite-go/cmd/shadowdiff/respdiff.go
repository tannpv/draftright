package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// parityHeaders is the allowlist of response headers the gate compares
// Node-vs-Go on EVERY fixture. The body differ parses to `any` and so was
// structurally blind to a total absence of CORS — that let the Go port ship
// with no Access-Control-Allow-Origin and pass 129/129 (issue #47). These are
// the browser-critical + content-negotiation headers that must match
// byte-for-byte; volatile/hop-by-hop headers (Date, Content-Length,
// Connection, X-Request-Id, ...) are deliberately excluded.
//
// Allow-Methods / Allow-Headers appear only on an OPTIONS preflight, so they
// are exercised by the dedicated CORS preflight fixture (compared here when
// present on either side — absent-on-both is parity).
var parityHeaders = []string{
	"Access-Control-Allow-Origin",
	"Access-Control-Allow-Credentials",
	"Access-Control-Allow-Methods",
	"Access-Control-Allow-Headers",
	"Content-Type",
	"Www-Authenticate",
}

// diffHeaders compares the parity-allowlist headers (plus Set-Cookie shape)
// between the Node and Go responses. A header absent on BOTH sides is parity;
// present on one only, or with a differing value, is a diff. Names in ignore
// (lower-cased) are skipped — the per-fixture escape hatch for the rare
// endpoint whose header legitimately differs (e.g. /metrics Content-Type).
func diffHeaders(node, goh http.Header, ignore map[string]bool) []string {
	var diffs []string
	for _, name := range parityHeaders {
		if ignore[strings.ToLower(name)] {
			continue
		}
		n, g := node.Get(name), goh.Get(name)
		if n != g {
			diffs = append(diffs, fmt.Sprintf("header %s: node=%q go=%q", name, n, g))
		}
	}
	// Set-Cookie carries per-request values (a freshly-minted session), so
	// compare the SHAPE — the sorted set of cookie names — not the values.
	if !ignore["set-cookie"] {
		if nc, gc := cookieNames(node), cookieNames(goh); nc != gc {
			diffs = append(diffs, fmt.Sprintf("Set-Cookie names: node=%q go=%q", nc, gc))
		}
	}
	return diffs
}

// cookieNames returns the sorted, comma-joined names of every Set-Cookie on
// the response (empty string when none). Values are intentionally dropped.
func cookieNames(h http.Header) string {
	var names []string
	for _, c := range h.Values("Set-Cookie") {
		if i := strings.IndexByte(c, '='); i > 0 {
			names = append(names, c[:i])
		}
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// diffRawBody compares two response bodies BYTE-for-byte, after trimming a
// single trailing newline (Go's json.NewEncoder.Encode appends "\n"; Node's
// res.json() does not — that lone difference is not a parity break). Unlike
// diffJSON (which unmarshals into `any` and is blind to key order), this
// surfaces a JSON key-order regression. Opt-in per fixture via
// compare_raw_body — use ONLY for deterministic bodies (no per-request tokens
// or ids), since there is no ignore_value_of equivalent here.
func diffRawBody(node, goBody []byte) []string {
	n := bytes.TrimRight(node, "\r\n")
	g := bytes.TrimRight(goBody, "\r\n")
	if bytes.Equal(n, g) {
		return nil
	}
	return []string{fmt.Sprintf("raw body (bytes/key order):\n     node=%s\n     go=  %s", n, g)}
}
