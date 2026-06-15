package feedback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// reuses fakeRepo / newFakeRepo / voteKey from usecase_test.go.

func newHandlerT(repo Repo, v *auth.Verifier) *Handler {
	return NewHandler(NewService(repo), v)
}

func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	return body
}

// (a) honeypot — non-empty website → 201 { id:null, message } with NO service call.
func TestFeedback_HoneypotDropsWithoutCreate(t *testing.T) {
	repo := newFakeRepo()
	h := newHandlerT(repo, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback",
		strings.NewReader(`{"kind":"bug","description":"d","source":"web","website":"spam"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "id", "message")
	body := decodeBody(t, raw)
	if body["id"] != nil {
		t.Fatalf("honeypot id must be null, got %v", body["id"])
	}
	if body["message"] != "Received. Thanks!" {
		t.Fatalf("honeypot message = %q, want %q", body["message"], "Received. Thanks!")
	}
	if _, hasRef := body["ref"]; hasRef {
		t.Error("honeypot response must NOT contain ref")
	}
	if repo.inserted != nil {
		t.Error("honeypot must not reach the service")
	}
}

// (b) @IsIn on kind — bad value → 400 verbatim message.
func TestFeedback_BadKind400(t *testing.T) {
	repo := newFakeRepo()
	h := newHandlerT(repo, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback",
		strings.NewReader(`{"kind":"x","description":"d","source":"web"}`))
	h.Create(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeBody(t, rec.Body.String())
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
	if body["error"] != "kind must be one of the following values: bug, feature" {
		t.Fatalf("400 message not byte-identical to Node: %v", body["error"])
	}
	if repo.inserted != nil {
		t.Error("invalid input must not reach the service")
	}
}

// (c) happy feature → 201 { id, ref:"FR-<n>", message }.
func TestFeedback_CreateFeature201(t *testing.T) {
	repo := newFakeRepo()
	h := newHandlerT(repo, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback",
		strings.NewReader(`{"kind":"feature","title":"Dark mode","target_platform":"mobile","description":"please","source":"web"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "id", "ref", "message")
	body := decodeBody(t, raw)
	if body["id"] != "new-id" {
		t.Fatalf("id = %v, want new-id", body["id"])
	}
	if body["ref"] != "FR-7" {
		t.Fatalf("ref = %v, want FR-7", body["ref"])
	}
	if body["message"] != "Feature request received. Thanks! Reference: FR-7" {
		t.Fatalf("message = %q", body["message"])
	}
}

// (c) happy bug → BUG-<n>, noun "Bug report".
func TestFeedback_CreateBug201(t *testing.T) {
	repo := newFakeRepo()
	h := newHandlerT(repo, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback",
		strings.NewReader(`{"kind":"bug","description":"broken","source":"web"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.String())
	if body["ref"] != "BUG-7" {
		t.Fatalf("ref = %v, want BUG-7", body["ref"])
	}
	if body["message"] != "Bug report received. Thanks! Reference: BUG-7" {
		t.Fatalf("message = %q", body["message"])
	}
}

// (d) GET /feedback → 200 { rows, total }, each row's viewerHasVoted LAST.
func TestFeedback_List200(t *testing.T) {
	repo := newFakeRepo()
	repo.listRows = []FeatureRow{{ID: "r1", Kind: "feature"}}
	repo.listTotal = 1
	h := newHandlerT(repo, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feedback", nil)
	h.List(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "rows", "total")
	idIdx := strings.Index(raw, `"id"`)
	vhvIdx := strings.Index(raw, `"viewerHasVoted"`)
	if vhvIdx < 0 || vhvIdx < idIdx {
		t.Fatalf("viewerHasVoted must appear (last) in the row: %s", raw)
	}
	var body struct {
		Rows  []map[string]any `json:"rows"`
		Total int64            `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Total != 1 || len(body.Rows) != 1 {
		t.Fatalf("rows/total = %+v", body)
	}
}

// (e) vote with no JWT → 401 "sign in to vote", code invalid-token.
func TestFeedback_VoteNoJWT401(t *testing.T) {
	h := newHandlerT(newFakeRepo(), nil)
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/feedback/abc/vote", nil), "abc")
	h.Vote(rec, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body := decodeBody(t, rec.Body.String())
	if body["error"] != "sign in to vote" {
		t.Fatalf("error = %v, want 'sign in to vote'", body["error"])
	}
	if body["code"] != "invalid-token" {
		t.Fatalf("code = %v, want invalid-token (Node UnauthorizedException → inferCode(401))", body["code"])
	}
}

// (e) vote with JWT → 200 { vote_count, hasVoted }.
func TestFeedback_VoteWithJWT200(t *testing.T) {
	const secret = "test-secret"
	signer := auth.NewSigner(secret)
	verifier := auth.NewVerifier(secret)
	tok, err := signer.Sign(auth.Claims{Sub: "user-1"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	repo.features["abc"] = true
	h := newHandlerT(repo, verifier)
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/feedback/abc/vote", nil), "abc")
	req.Header.Set("Authorization", "Bearer "+tok)
	h.Vote(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "vote_count", "hasVoted")
	body := decodeBody(t, raw)
	if body["vote_count"] != float64(1) {
		t.Fatalf("vote_count = %v, want 1", body["vote_count"])
	}
	if body["hasVoted"] != true {
		t.Fatalf("hasVoted = %v, want true", body["hasVoted"])
	}
}

// (e) vote on non-feature id → 404 "feature request not found".
func TestFeedback_VoteNonFeature404(t *testing.T) {
	const secret = "test-secret"
	signer := auth.NewSigner(secret)
	verifier := auth.NewVerifier(secret)
	tok, _ := signer.Sign(auth.Claims{Sub: "user-1"}, time.Hour)
	repo := newFakeRepo() // no features → not found
	h := newHandlerT(repo, verifier)
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/feedback/abc/vote", nil), "abc")
	req.Header.Set("Authorization", "Bearer "+tok)
	h.Vote(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := decodeBody(t, rec.Body.String())
	if body["error"] != "feature request not found" {
		t.Fatalf("error = %v, want 'feature request not found'", body["error"])
	}
	if body["code"] != "not-found" {
		t.Fatalf("code = %v, want not-found", body["code"])
	}
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
