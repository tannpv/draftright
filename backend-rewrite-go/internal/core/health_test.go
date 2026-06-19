package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubLogLevel implements the LogLevelReader the health handler needs.
type stubLogLevel struct {
	level string
	err   error
}

func (s stubLogLevel) ClientLogLevel(context.Context) (string, error) { return s.level, s.err }

func TestHealth_ShapeMatchesNode(t *testing.T) {
	h := NewHealthHandler(stubLogLevel{level: "warnings"}, "2.0.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	want := map[string]any{"app": "draftright", "version": "2.0.0", "status": "ok", "client_log_level": "warnings"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%q] = %v, want %v", k, body[k], v)
		}
	}
}

func TestHealth_DBErrorFallsBackToInfo(t *testing.T) {
	h := NewHealthHandler(stubLogLevel{err: context.DeadlineExceeded}, "2.0.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("a DB hiccup must not fail /health; status = %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["client_log_level"] != "info" {
		t.Errorf("fallback level = %v, want info", body["client_log_level"])
	}
}
