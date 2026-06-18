package appsettings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reuses fakeRepo / fakeValidator / fakeSender + ptr/ptrInt from
// usecase_test.go / repo_pg_test.go — do NOT redeclare them here.

func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	return body
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

// GET /admin/settings → 200, body is the full settings row.
func TestGet_Returns200(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/admin/settings", nil))

	if rec.Code != 200 {
		t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"environment"`) {
		t.Fatalf("body missing environment key: %s", rec.Body.String())
	}
}

// PATCH /admin/settings with a valid patch → 200, full settings row.
func TestPatch_Returns200(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.Patch(rec, httptest.NewRequest(http.MethodPatch, "/admin/settings",
		strings.NewReader(`{"environment":"prod"}`)))

	if rec.Code != 200 {
		t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"environment"`) {
		t.Fatalf("body missing environment key: %s", rec.Body.String())
	}
}

// POST /admin/settings/test-email with a recipient missing '@'.
// Node throws a PLAIN Error (not a NestException) → AllExceptionsFilter
// classifies it as 500 / internal. Byte-identical parity requires 500.
func TestTestEmail_BadRecipient500(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.TestEmail(rec, httptest.NewRequest(http.MethodPost, "/admin/settings/test-email", strings.NewReader(`{"to":"bad"}`)))
	if rec.Code != 500 {
		t.Fatalf("status=%d, want 500", rec.Code)
	}
	body := decodeBody(t, rec.Body.String())
	if body["code"] != "internal" || body["error"] != "Valid recipient email required" {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestTestEmail_OK200(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.TestEmail(rec, httptest.NewRequest(http.MethodPost, "/admin/settings/test-email", strings.NewReader(`{"to":"a@b.com"}`)))
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "sent", "to")
	body := decodeBody(t, raw)
	if body["sent"] != true || body["to"] != "a@b.com" {
		t.Fatalf("body=%s", raw)
	}
}
