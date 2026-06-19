package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// next records whether the wrapped handler ran and returns 200.
func corsNext(ran *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*ran = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORS_SimpleRequestSetsWildcardOrigin(t *testing.T) {
	var ran bool
	req := httptest.NewRequest(http.MethodGet, "/plans", nil)
	req.Header.Set("Origin", "https://draftright.info")
	rec := httptest.NewRecorder()

	CORS(corsNext(&ran)).ServeHTTP(rec, req)

	if !ran {
		t.Fatal("next handler should run for a non-preflight request")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q, want *", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestCORS_NoOriginStillSetsWildcard(t *testing.T) {
	// Node's cors emits ACAO:* even without an Origin header.
	var ran bool
	req := httptest.NewRequest(http.MethodGet, "/plans", nil)
	rec := httptest.NewRecorder()

	CORS(corsNext(&ran)).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q, want *", got)
	}
}

func TestCORS_PreflightWithRequestHeaders(t *testing.T) {
	var ran bool
	req := httptest.NewRequest(http.MethodOptions, "/rewrite", nil)
	req.Header.Set("Origin", "https://draftright.info")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	rec := httptest.NewRecorder()

	CORS(corsNext(&ran)).ServeHTTP(rec, req)

	if ran {
		t.Fatal("next handler must NOT run for an OPTIONS preflight")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q, want *", got)
	}
	if got := h.Get("Access-Control-Allow-Methods"); got != "GET,HEAD,PUT,PATCH,POST,DELETE" {
		t.Fatalf("ACAM = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Headers"); got != "authorization,content-type" {
		t.Fatalf("ACAH = %q, want reflected request headers", got)
	}
	if got := h.Get("Vary"); got != "Access-Control-Request-Headers" {
		t.Fatalf("Vary = %q", got)
	}
}

func TestCORS_PreflightWithoutRequestHeadersOmitsAllowHeaders(t *testing.T) {
	var ran bool
	req := httptest.NewRequest(http.MethodOptions, "/rewrite", nil)
	req.Header.Set("Origin", "https://draftright.info")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	CORS(corsNext(&ran)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	h := rec.Header()
	if _, ok := h["Access-Control-Allow-Headers"]; ok {
		t.Fatalf("Allow-Headers must be omitted when no request headers were sent")
	}
	// Vary is always present on a preflight (allowedHeaders is reflected).
	if got := h.Get("Vary"); got != "Access-Control-Request-Headers" {
		t.Fatalf("Vary = %q", got)
	}
}
