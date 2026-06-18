package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeTemplatesRepo satisfies AdminTemplatesService's consumer-side port. It
// returns a fixed customization map so the merge logic can be asserted without
// a DB.
type fakeTemplatesRepo struct {
	custom map[string]DBTemplate
}

func (f *fakeTemplatesRepo) ListCustomizations(ctx context.Context) (map[string]DBTemplate, error) {
	if f.custom == nil {
		return map[string]DBTemplate{}, nil
	}
	return f.custom, nil
}

func newTemplatesHandler(repo *fakeTemplatesRepo) *AdminTemplatesHandler {
	return NewAdminTemplatesHandler(NewAdminTemplatesService(repo))
}

// assertTemplatesKeyOrder fails unless the JSON keys appear in raw in the given
// order. Local helper (string-scan, like the sibling row-order tests).
func assertTemplatesKeyOrder(t *testing.T, raw string, keys ...string) {
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

// TestTemplatesList_BareArraySixInOrder: GET /admin/email-templates returns a
// bare JSON array of all 6 builtins in EMAIL_TEMPLATES order. verification's
// variables = ["name","code"].
func TestTemplatesList_BareArraySixInOrder(t *testing.T) {
	h := newTemplatesHandler(&fakeTemplatesRepo{})
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/email-templates", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		t.Fatalf("response is not a bare array: %s", raw)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("body not a JSON array: %v (%s)", err, raw)
	}
	if len(rows) != 6 {
		t.Fatalf("len = %d, want 6", len(rows))
	}
	wantKeys := []string{
		"verification", "password-reset", "subscription-activated",
		"subscription-expired", "renewal-reminder", "payment-failed",
	}
	for i, wk := range wantKeys {
		if rows[i]["key"] != wk {
			t.Fatalf("rows[%d].key = %v, want %q", i, rows[i]["key"], wk)
		}
	}

	vars, ok := rows[0]["variables"].([]any)
	if !ok || len(vars) != 2 || vars[0] != "name" || vars[1] != "code" {
		t.Fatalf("verification variables = %v, want [name code]", rows[0]["variables"])
	}
}

// TestTemplatesList_Customization: a DB row for subscription-expired overrides
// subject/html and sets customized=true, while default_subject still equals the
// builtin default.
func TestTemplatesList_Customization(t *testing.T) {
	h := newTemplatesHandler(&fakeTemplatesRepo{custom: map[string]DBTemplate{
		"subscription-expired": {Subject: "<custom>", HTML: "<custom html>"},
	}})
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/email-templates", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("body not a JSON array: %v", err)
	}

	var expired map[string]any
	for _, row := range rows {
		if row["key"] == "subscription-expired" {
			expired = row
		}
	}
	if expired == nil {
		t.Fatalf("subscription-expired row missing")
	}
	if expired["customized"] != true {
		t.Fatalf("customized = %v, want true", expired["customized"])
	}
	if expired["subject"] != "<custom>" {
		t.Fatalf("subject = %v, want <custom>", expired["subject"])
	}
	if expired["html"] != "<custom html>" {
		t.Fatalf("html = %v, want <custom html>", expired["html"])
	}
	if expired["default_subject"] != builtinTemplates["subscription-expired"].subject {
		t.Fatalf("default_subject = %v, want builtin default", expired["default_subject"])
	}
}

// TestTemplatesList_UncustomizedFallsBack: a key with no DB row has
// customized=false and subject == default_subject (builtin default).
func TestTemplatesList_UncustomizedFallsBack(t *testing.T) {
	h := newTemplatesHandler(&fakeTemplatesRepo{custom: map[string]DBTemplate{
		"subscription-expired": {Subject: "<custom>", HTML: "<custom html>"},
	}})
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/email-templates", nil))

	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("body not a JSON array: %v", err)
	}
	var verification map[string]any
	for _, row := range rows {
		if row["key"] == "verification" {
			verification = row
		}
	}
	if verification == nil {
		t.Fatalf("verification row missing")
	}
	if verification["customized"] != false {
		t.Fatalf("customized = %v, want false", verification["customized"])
	}
	if verification["subject"] != verification["default_subject"] {
		t.Fatalf("subject %v != default_subject %v", verification["subject"], verification["default_subject"])
	}
	if verification["subject"] != builtinTemplates["verification"].subject {
		t.Fatalf("subject = %v, want builtin default", verification["subject"])
	}
}

// TestTemplatesList_KeyOrder: per-row JSON key order is exactly key, label,
// variables, subject, html, customized, default_subject, default_html.
func TestTemplatesList_KeyOrder(t *testing.T) {
	h := newTemplatesHandler(&fakeTemplatesRepo{})
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/email-templates", nil))

	raw := rec.Body.String()
	assertTemplatesKeyOrder(t, raw,
		"key", "label", "variables", "subject", "html",
		"customized", "default_subject", "default_html")
}
