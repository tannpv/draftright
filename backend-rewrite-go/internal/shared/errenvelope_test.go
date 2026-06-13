package shared

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusForCode_MatchesNodeTable(t *testing.T) {
	cases := map[string]int{
		"invalid-input":        http.StatusBadRequest,          // 400
		"invalid-token":        http.StatusUnauthorized,        // 401
		"user-not-found":       http.StatusUnauthorized,        // 401
		"quota-exceeded":       http.StatusPaymentRequired,     // 402
		"forbidden":            http.StatusForbidden,           // 403
		"not-found":            http.StatusNotFound,            // 404
		"conflict":             http.StatusConflict,            // 409
		"rate-limited":         http.StatusTooManyRequests,     // 429
		"provider-failed":      http.StatusBadGateway,          // 502
		"provider-unavailable": http.StatusServiceUnavailable,  // 503
		"internal":             http.StatusInternalServerError, // 500
		"something-unmapped":   http.StatusInternalServerError, // default 500
	}
	for code, want := range cases {
		if got := StatusForCode(code); got != want {
			t.Errorf("StatusForCode(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestWriteError_EnvelopeShapeWithRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := context.WithValue(context.Background(), requestIDCtxKey{}, "req-77")
	req := httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(ctx)

	WriteError(rec, req, "quota-exceeded", "daily limit reached")

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rec.Code)
	}
	var body struct {
		Error     string `json:"error"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Error != "daily limit reached" || body.Code != "quota-exceeded" || body.RequestID != "req-77" {
		t.Fatalf("envelope = %+v, want {error:daily limit reached, code:quota-exceeded, request_id:req-77}", body)
	}
}
