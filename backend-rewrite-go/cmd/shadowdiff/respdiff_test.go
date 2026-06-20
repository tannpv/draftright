package main

import (
	"net/http"
	"testing"
)

func hdr(pairs ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(pairs); i += 2 {
		h.Add(pairs[i], pairs[i+1])
	}
	return h
}

func TestDiffHeaders_CORSPresentBothSides(t *testing.T) {
	n := hdr("Access-Control-Allow-Origin", "*", "Content-Type", "application/json; charset=utf-8")
	g := hdr("Access-Control-Allow-Origin", "*", "Content-Type", "application/json; charset=utf-8")
	if d := diffHeaders(n, g, nil); len(d) != 0 {
		t.Fatalf("expected parity, got %v", d)
	}
}

func TestDiffHeaders_MissingCORSOnGoIsCaught(t *testing.T) {
	// The exact #47 regression: Node sends ACAO, Go ships with none.
	n := hdr("Access-Control-Allow-Origin", "*")
	g := hdr()
	d := diffHeaders(n, g, nil)
	if len(d) != 1 || d[0] != `header Access-Control-Allow-Origin: node="*" go=""` {
		t.Fatalf("CORS gap not caught: %v", d)
	}
}

func TestDiffHeaders_AbsentOnBothIsParity(t *testing.T) {
	if d := diffHeaders(hdr(), hdr(), nil); len(d) != 0 {
		t.Fatalf("absent-on-both must be parity, got %v", d)
	}
}

func TestDiffHeaders_ContentTypeMismatchCaught(t *testing.T) {
	n := hdr("Content-Type", "application/json; charset=utf-8")
	g := hdr("Content-Type", "application/json")
	d := diffHeaders(n, g, nil)
	if len(d) != 1 {
		t.Fatalf("content-type mismatch not caught: %v", d)
	}
}

func TestDiffHeaders_IgnoreSuppresses(t *testing.T) {
	n := hdr("Content-Type", "text/plain; version=0.0.4")
	g := hdr("Content-Type", "text/plain; charset=utf-8")
	if d := diffHeaders(n, g, map[string]bool{"content-type": true}); len(d) != 0 {
		t.Fatalf("ignore_headers should suppress, got %v", d)
	}
}

func TestDiffHeaders_SetCookieShapeNotValue(t *testing.T) {
	// Same cookie name, different (per-request) value → parity.
	n := hdr("Set-Cookie", "sid=abc123; HttpOnly")
	g := hdr("Set-Cookie", "sid=zzz999; HttpOnly")
	if d := diffHeaders(n, g, nil); len(d) != 0 {
		t.Fatalf("Set-Cookie value should not matter, got %v", d)
	}
	// Different cookie NAME → diff.
	g2 := hdr("Set-Cookie", "other=zzz999; HttpOnly")
	if d := diffHeaders(n, g2, nil); len(d) != 1 {
		t.Fatalf("Set-Cookie name mismatch not caught: %v", d)
	}
}

func TestDiffRawBody_KeyOrderRegressionCaught(t *testing.T) {
	// Node declaration order vs Go map-sorted order — parsed diff would PASS,
	// raw must FAIL.
	node := []byte(`{"id":"u1","email":"a@b.c","role":"user"}`)
	gob := []byte(`{"email":"a@b.c","id":"u1","role":"user"}` + "\n")
	if d := diffRawBody(node, gob); len(d) != 1 {
		t.Fatalf("key-order regression not caught: %v", d)
	}
}

func TestDiffRawBody_TrailingNewlineNormalized(t *testing.T) {
	// Identical key order, Go's lone trailing "\n" must NOT be a diff.
	node := []byte(`{"app":"draftright","status":"ok"}`)
	gob := []byte(`{"app":"draftright","status":"ok"}` + "\n")
	if d := diffRawBody(node, gob); len(d) != 0 {
		t.Fatalf("trailing newline must be normalized, got %v", d)
	}
}

func TestLowerSet(t *testing.T) {
	if lowerSet(nil) != nil {
		t.Fatal("nil slice → nil set")
	}
	m := lowerSet([]string{"Content-Type", "X-Foo"})
	if !m["content-type"] || !m["x-foo"] {
		t.Fatalf("lowerSet wrong: %v", m)
	}
}
