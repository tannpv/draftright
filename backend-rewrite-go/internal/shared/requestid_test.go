package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_MintsAndStampsResponseHeader(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if seen == "" {
		t.Fatal("handler saw empty request id in context")
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Fatalf("response header X-Request-Id = %q, want %q (the context value)", got, seen)
	}
}

func TestRequestID_HonoursInboundHeader(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "inbound-123")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "inbound-123" {
		t.Fatalf("request id = %q, want inbound-123 (existing header reused)", seen)
	}
}
