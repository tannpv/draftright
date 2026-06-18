package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// fakeTemplatesRepo satisfies AdminTemplatesService's consumer-side port. It
// returns a fixed customization map so the merge logic can be asserted without
// a DB.
type fakeTemplatesRepo struct {
	custom map[string]DBTemplate

	// recorded write activity
	upserted   bool
	upsertKey  string
	upsertSubj string
	upsertHTML string
	deleted    bool
	deletedKey string
}

func (f *fakeTemplatesRepo) ListCustomizations(ctx context.Context) (map[string]DBTemplate, error) {
	if f.custom == nil {
		return map[string]DBTemplate{}, nil
	}
	return f.custom, nil
}

func (f *fakeTemplatesRepo) Upsert(ctx context.Context, key, subject, html string) error {
	f.upserted = true
	f.upsertKey = key
	f.upsertSubj = subject
	f.upsertHTML = html
	return nil
}

func (f *fakeTemplatesRepo) Delete(ctx context.Context, key string) error {
	f.deleted = true
	f.deletedKey = key
	return nil
}

// routeWithKey injects a chi route param so chi.URLParam(r,"key") resolves.
func routeWithKey(req *http.Request, key string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", key)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
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

// TestUpdateTemplate_UnknownKey404: PATCH with an unknown builtin key → 404
// not-found "Unknown template"; the repo is NOT upserted.
func TestUpdateTemplate_UnknownKey404(t *testing.T) {
	repo := &fakeTemplatesRepo{}
	h := newTemplatesHandler(repo)
	req := httptest.NewRequest(http.MethodPatch, "/admin/email-templates/nope",
		strings.NewReader(`{"subject":"x"}`))
	req = routeWithKey(req, "nope")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"code":"not-found"`) {
		t.Fatalf("body missing code not-found: %s", raw)
	}
	if !strings.Contains(raw, `"error":"Unknown template"`) {
		t.Fatalf("body missing error Unknown template: %s", raw)
	}
	if repo.upserted {
		t.Fatalf("repo was upserted for unknown key")
	}
}

// TestUpdateTemplate_OK: PATCH a real key → 200 {"ok":true}; the repo records
// the upsert with the right subject/html.
func TestUpdateTemplate_OK(t *testing.T) {
	repo := &fakeTemplatesRepo{}
	h := newTemplatesHandler(repo)
	req := httptest.NewRequest(http.MethodPatch, "/admin/email-templates/verification",
		strings.NewReader(`{"subject":"Hi","html":"<b>x</b>"}`))
	req = routeWithKey(req, "verification")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("body missing ok:true: %s", rec.Body.String())
	}
	if !repo.upserted {
		t.Fatalf("repo was not upserted")
	}
	if repo.upsertKey != "verification" || repo.upsertSubj != "Hi" || repo.upsertHTML != "<b>x</b>" {
		t.Fatalf("upsert args = (%q,%q,%q), want (verification,Hi,<b>x</b>)",
			repo.upsertKey, repo.upsertSubj, repo.upsertHTML)
	}
}

// TestResetTemplate_OK: DELETE a real key → 200 {"ok":true}; the repo records
// the delete. An unknown key is idempotent — still 200, NO 404.
func TestResetTemplate_OK(t *testing.T) {
	repo := &fakeTemplatesRepo{}
	h := newTemplatesHandler(repo)
	req := httptest.NewRequest(http.MethodDelete, "/admin/email-templates/verification", nil)
	req = routeWithKey(req, "verification")
	rec := httptest.NewRecorder()
	h.Reset(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("body missing ok:true: %s", rec.Body.String())
	}
	if !repo.deleted || repo.deletedKey != "verification" {
		t.Fatalf("delete = (%v,%q), want (true,verification)", repo.deleted, repo.deletedKey)
	}

	// Unknown key is idempotent — still 200 {ok:true}, no 404.
	repo2 := &fakeTemplatesRepo{}
	h2 := newTemplatesHandler(repo2)
	req2 := httptest.NewRequest(http.MethodDelete, "/admin/email-templates/nope", nil)
	req2 = routeWithKey(req2, "nope")
	rec2 := httptest.NewRecorder()
	h2.Reset(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("unknown-key reset status = %d, want 200; body=%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), `"ok":true`) {
		t.Fatalf("unknown-key reset body missing ok:true: %s", rec2.Body.String())
	}
}

// TestPreviewTemplate_RendersSubjectHTML: GET preview of a real key → 200,
// {subject,html} in order, with {{vars}} substituted (html greets {{name}} → Tan).
func TestPreviewTemplate_RendersSubjectHTML(t *testing.T) {
	repo := &fakeTemplatesRepo{}
	h := newTemplatesHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/admin/email-templates/verification/preview", nil)
	req = routeWithKey(req, "verification")
	rec := httptest.NewRecorder()
	h.Preview(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertTemplatesKeyOrder(t, raw, "subject", "html")

	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	if body["subject"] != builtinTemplates["verification"].subject {
		t.Fatalf("subject = %v, want builtin verification subject", body["subject"])
	}
	html, _ := body["html"].(string)
	if !strings.Contains(html, "Tan") {
		t.Fatalf("html did not substitute {{name}} → Tan: %s", html)
	}
}

// TestPreviewTemplate_UnknownKey404: GET preview of an unknown key → 404.
func TestPreviewTemplate_UnknownKey404(t *testing.T) {
	repo := &fakeTemplatesRepo{}
	h := newTemplatesHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/admin/email-templates/nope/preview", nil)
	req = routeWithKey(req, "nope")
	rec := httptest.NewRecorder()
	h.Preview(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"not-found"`) {
		t.Fatalf("body missing code not-found: %s", rec.Body.String())
	}
}

// TestPreviewTemplate_UsesCustomization: a DB customization for verification
// overrides the subject; the preview reflects the CUSTOM subject (DB-override-aware).
func TestPreviewTemplate_UsesCustomization(t *testing.T) {
	repo := &fakeTemplatesRepo{custom: map[string]DBTemplate{
		"verification": {Subject: "Custom subject for {{name}}", HTML: "<p>custom {{name}}</p>"},
	}}
	h := newTemplatesHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/admin/email-templates/verification/preview", nil)
	req = routeWithKey(req, "verification")
	rec := httptest.NewRecorder()
	h.Preview(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["subject"] != "Custom subject for Tan" {
		t.Fatalf("subject = %v, want custom (Custom subject for Tan)", body["subject"])
	}
	html, _ := body["html"].(string)
	if !strings.Contains(html, "custom") {
		t.Fatalf("html did not use custom template: %s", html)
	}
}
