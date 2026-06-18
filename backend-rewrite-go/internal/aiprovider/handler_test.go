package aiprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// reuses fakeRepo / fakeFactory / fakeCompleter from usecase_test.go.

func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	return body
}

// routeWithID injects a chi route param so chi.URLParam(r,"id") resolves.
func routeWithID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
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

// 1. GET /admin/ai-providers with an empty repo → 200, body trimmed == "[]".
func TestList_ReturnsArray(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeFactory{}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ai-providers", nil)
	h.List(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("body = %q, want %q", got, "[]")
	}
}

// 2. POST /admin/ai-providers/:id/test when GetByID → ErrNotFound → 200,
// key order success,error, success==false && error=="Provider not found".
func TestTestRoute_NotFound(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{provider: nil}, fakeFactory{}))
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/admin/ai-providers/x/test", nil), "x")
	h.Test(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "success", "error")
	body := decodeBody(t, raw)
	if body["success"] != false {
		t.Fatalf("success = %v, want false", body["success"])
	}
	if body["error"] != "Provider not found" {
		t.Fatalf("error = %v, want 'Provider not found'", body["error"])
	}
}

// 3. listquery config: sort alias + status col + search compose the intended SQL.
func TestPaginatedConfig_SortAliasAndStatus(t *testing.T) {
	q := listquery.Parse(map[string][]string{
		"sort_by": {"name"},
		"status":  {"inactive"},
		"search":  {"gpt"},
	})
	b := listquery.Build(q, aiSearchCols, aiSortAllow, "created_at", "is_active")

	if !strings.Contains(b.Order, "name") {
		t.Fatalf("order %q does not contain 'name'", b.Order)
	}
	if !strings.Contains(b.Where, "is_active") {
		t.Fatalf("where %q does not contain 'is_active'", b.Where)
	}
}

// 4. POST /admin/ai-providers (Create) pins the create status code (201).
func TestCreate_Returns201(t *testing.T) {
	repo := &fakeRepo{}
	h := NewHandler(NewService(repo, fakeFactory{}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/ai-providers",
		strings.NewReader(`{"name":"GPT","type":"openai","endpoint_url":"https://x","model":"gpt-4"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	// temperature absent → default "0.3" handed to the repo.
	if repo.inserted.Temperature != "0.3" {
		t.Fatalf("inserted temperature = %q, want %q", repo.inserted.Temperature, "0.3")
	}
}

// 5. DELETE /admin/ai-providers/:id → 200 { "success": true }.
func TestDelete_SuccessBody(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeFactory{}))
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodDelete, "/admin/ai-providers/x", nil), "x")
	h.Delete(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"success":true}` {
		t.Fatalf("body = %q, want %q", got, `{"success":true}`)
	}
}
