package appsettings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reuses fakeRepo / fakeValidator / fakeSender + ptr/ptrInt from
// usecase_test.go / repo_pg_test.go — do NOT redeclare them here.

// errRepo is a minimal Repo whose Patch always fails, to exercise the
// repo-failure → 500 path. Kept local so the shared fakeRepo stays clean.
type errRepo struct{}

func (errRepo) GetOrCreate(context.Context) (AppSettings, error) {
	return AppSettings{ID: "1"}, nil
}
func (errRepo) Patch(context.Context, Patch) (AppSettings, error) {
	return AppSettings{}, errors.New("db down")
}

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

// PATCH /admin/settings naming an unregisterable payment method.
// Node's assertMethodsRegisterable throws BadRequestException → 400, body
// `error` = the validator's EXACT message. The Go handler must map the typed
// InvalidSettingsError to 400 invalid-input with the message byte-identical.
func TestPatch_BadPaymentMethods400(t *testing.T) {
	const msg = "Cannot enable payment method(s) with no backend strategy: bogus"
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{err: errors.New(msg)}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.Patch(rec, httptest.NewRequest(http.MethodPatch, "/admin/settings",
		strings.NewReader(`{"payment_methods_enabled":"bogus"}`)))

	if rec.Code != 400 {
		t.Fatalf("status=%d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.String())
	if body["code"] != "invalid-input" {
		t.Fatalf("code=%v, want invalid-input; body=%s", body["code"], rec.Body.String())
	}
	if body["error"] != msg {
		t.Fatalf("error=%q, want byte-identical %q", body["error"], msg)
	}
}

// PATCH /admin/settings where the repo (DB) fails. Node's TypeORM throws a
// plain error → AllExceptionsFilter 500. The Go handler must map a bare repo
// error to 500 internal (NOT 400 — it is not an InvalidSettingsError).
func TestPatch_RepoError500(t *testing.T) {
	h := NewHandler(NewService(errRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.Patch(rec, httptest.NewRequest(http.MethodPatch, "/admin/settings",
		strings.NewReader(`{"environment":"prod"}`)))

	if rec.Code != 500 {
		t.Fatalf("status=%d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.String())
	if body["code"] != "internal" {
		t.Fatalf("code=%v, want internal; body=%s", body["code"], rec.Body.String())
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

func TestTestEmail_OK201(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.TestEmail(rec, httptest.NewRequest(http.MethodPost, "/admin/settings/test-email", strings.NewReader(`{"to":"a@b.com"}`)))
	if rec.Code != 201 {
		t.Fatalf("status=%d, want 201", rec.Code)
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "sent", "to")
	body := decodeBody(t, raw)
	if body["sent"] != true || body["to"] != "a@b.com" {
		t.Fatalf("body=%s", raw)
	}
}
