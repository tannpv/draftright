package errreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubIngest struct {
	res *Existing
	err error
	got CreateErrorReport
	uid string
}

func (s *stubIngest) Ingest(_ context.Context, in CreateErrorReport, uid string) (*Existing, error) {
	s.got, s.uid = in, uid
	return s.res, s.err
}

func newHandler(s ingestService) *Handler { return &Handler{svc: s, verifier: nil} }

func TestErrors_HoneypotDropsWithoutRef(t *testing.T) {
	st := &stubIngest{}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"platform":"ios","website":"spam"}`))
	h.Ingest(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	raw := rec.Body.String()
	// Node key order: ok, id, fingerprint, count, first_seen_at.
	assertKeyOrder(t, raw, "ok", "id", "fingerprint", "count", "first_seen_at")
	var body map[string]any
	json.Unmarshal([]byte(raw), &body)
	if body["ok"] != true || body["id"] != nil || body["count"] != float64(0) || body["fingerprint"] != nil {
		t.Fatalf("honeypot body wrong: %v", body)
	}
	if _, hasRef := body["ref"]; hasRef {
		t.Error("honeypot response must NOT contain ref")
	}
	if st.got.Platform != "" {
		t.Error("honeypot must not reach the service")
	}
}

func TestErrors_NormalIngest201WithRef(t *testing.T) {
	st := &stubIngest{res: &Existing{ID: "abc", DisplayNo: 42, Count: 1, Fingerprint: strings.Repeat("a", 64)}}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"platform":"ios","message":"boom"}`))
	h.Ingest(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	raw := rec.Body.String()
	// Node key order: ok, id, ref, fingerprint, count, first_seen_at.
	assertKeyOrder(t, raw, "ok", "id", "ref", "fingerprint", "count", "first_seen_at")
	var body map[string]any
	json.Unmarshal([]byte(raw), &body)
	if body["ref"] != "ERR-42" || body["id"] != "abc" || body["count"] != float64(1) {
		t.Fatalf("body = %v", body)
	}
	fp, ok := body["fingerprint"].(string)
	if !ok || len(fp) != 64 {
		t.Fatalf("fingerprint must be a 64-char hex string, got %v", body["fingerprint"])
	}
}

func TestErrors_BadPlatform400(t *testing.T) {
	st := &stubIngest{err: ErrInvalidPlatform}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"platform":"symbian"}`))
	h.Ingest(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if body["error"] != "platform must be one of: ios, android, macos, windows, linux, web" {
		t.Fatalf("400 message not byte-identical to Node: %v", body["error"])
	}
}

// assertKeyOrder fails unless the JSON keys appear in raw in the given order.
func assertKeyOrder(t *testing.T, raw string, keys ...string) {
	t.Helper()
	prev := -1
	for _, k := range keys {
		idx := strings.Index(raw, `"`+k+`"`)
		if idx < 0 {
			t.Fatalf("key %q missing from body: %s", k, raw)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in body: %s", k, raw)
		}
		prev = idx
	}
}
