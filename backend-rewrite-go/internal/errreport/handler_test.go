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

func TestErrors_Oversize500Internal(t *testing.T) {
	st := &stubIngest{}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	// Body exceeds maxBodyBytes → MaxBytesReader trips. Mirrors Node:
	// PayloadTooLargeError (plain Error, not HttpException) → 500 internal.
	huge := strings.Repeat("a", maxBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"message":"`+huge+`"}`))
	h.Ingest(rec, req)
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "internal" {
		t.Fatalf("code = %v, want internal", body["code"])
	}
	if body["error"] != "request entity too large" {
		t.Fatalf("error = %v, want 'request entity too large'", body["error"])
	}
	if st.got.Message != "" {
		t.Error("oversize must not reach the service")
	}
}

func TestErrors_MaxLengthValidation400(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		wantMsg string
	}{
		{"platform", `{"platform":"` + strings.Repeat("x", 21) + `"}`,
			"platform must be shorter than or equal to 20 characters"},
		{"app_version", `{"platform":"ios","app_version":"` + strings.Repeat("x", 51) + `"}`,
			"app_version must be shorter than or equal to 50 characters"},
		{"severity", `{"platform":"ios","severity":"` + strings.Repeat("x", 21) + `"}`,
			"severity must be shorter than or equal to 20 characters"},
		{"error_type", `{"platform":"ios","error_type":"` + strings.Repeat("x", 201) + `"}`,
			"error_type must be shorter than or equal to 200 characters"},
		{"device_id", `{"platform":"ios","device_id":"` + strings.Repeat("x", 101) + `"}`,
			"device_id must be shorter than or equal to 100 characters"},
		// website > 255 must yield the length-400, NOT a honeypot 201, since
		// ValidationPipe runs before the controller's honeypot check.
		{"website", `{"platform":"ios","website":"` + strings.Repeat("x", 256) + `"}`,
			"website must be shorter than or equal to 255 characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &stubIngest{}
			h := newHandler(st)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(tc.json))
			h.Ingest(rec, req)
			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			var body map[string]any
			json.Unmarshal(rec.Body.Bytes(), &body)
			if body["code"] != "invalid-input" {
				t.Fatalf("code = %v, want invalid-input", body["code"])
			}
			if body["error"] != tc.wantMsg {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantMsg)
			}
			if st.got.Platform != "" || st.got.Website != "" {
				t.Error("invalid input must not reach the service")
			}
		})
	}
}

func TestErrors_AtLimitPasses(t *testing.T) {
	// Every bounded field exactly at its limit → must pass validation and
	// reach the service (which the stub accepts).
	st := &stubIngest{res: &Existing{ID: "ok", DisplayNo: 1, Count: 1, Fingerprint: strings.Repeat("a", 64)}}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	body := `{` +
		`"platform":"` + strings.Repeat("p", 20) + `",` +
		`"app_version":"` + strings.Repeat("v", 50) + `",` +
		`"severity":"` + strings.Repeat("s", 20) + `",` +
		`"error_type":"` + strings.Repeat("e", 200) + `",` +
		`"device_id":"` + strings.Repeat("d", 100) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(body))
	h.Ingest(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201 (at-limit values are valid): %s", rec.Code, rec.Body.String())
	}
}

func TestErrors_MaxLengthUsesUTF16Units(t *testing.T) {
	// A single astral codepoint (😀) is one rune but TWO UTF-16 code units,
	// matching JS String.length. severity max is 20 → 10 emoji = 20 units
	// passes, 11 emoji = 22 units fails.
	st := &stubIngest{res: &Existing{ID: "ok", DisplayNo: 1, Count: 1, Fingerprint: strings.Repeat("a", 64)}}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors",
		strings.NewReader(`{"platform":"ios","severity":"`+strings.Repeat("\U0001F600", 11)+`"}`))
	h.Ingest(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 (11 emoji = 22 UTF-16 units > 20)", rec.Code)
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
