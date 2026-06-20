package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/plans"
)

// fakeAdminUserRepo satisfies userAdminRepo. listRows/total feed List; getFull
// + getErr feed Get/Update's full-entity read.
type fakeAdminUserRepo struct {
	listRows []UserListRow
	total    int
	getFull  UserDetail
	getErr   error
	lastList ListUsersParams
	updated  UserPatchAdmin
	updateID string
}

func (f *fakeAdminUserRepo) ListUsers(ctx context.Context, p ListUsersParams) ([]UserListRow, int, error) {
	f.lastList = p
	return f.listRows, f.total, nil
}
func (f *fakeAdminUserRepo) GetFull(ctx context.Context, id string) (UserDetail, error) {
	return f.getFull, f.getErr
}
func (f *fakeAdminUserRepo) Update(ctx context.Context, id string, p UserPatchAdmin) error {
	f.updateID = id
	f.updated = p
	return nil
}

// fakeSubReader / fakeUsage / fakeRecent are the three sibling ports.
type fakeSubReader struct{ sub *AdminSubView }

func (f *fakeSubReader) ActiveSubByUser(ctx context.Context, userID string) (*AdminSubView, error) {
	return f.sub, nil
}

type fakeUsage struct{ count int }

func (f *fakeUsage) CountToday(ctx context.Context, userID string) (int, error) {
	return f.count, nil
}

type fakeRecent struct{ rows []RecentUsageRow }

func (f *fakeRecent) RecentUsageByUser(ctx context.Context, userID string) ([]RecentUsageRow, error) {
	return f.rows, nil
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

// decodeBody unmarshals into a generic map for value assertions.
func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	return m
}

// routeWithID attaches a chi RouteContext carrying :id so chi.URLParam resolves.
func routeWithID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newHandler(repo *fakeAdminUserRepo, sub *fakeSubReader, usage *fakeUsage, recent *fakeRecent) *AdminHandler {
	return NewAdminHandler(NewAdminService(repo, usage, sub, recent))
}

// TestListUsers_Shape: GET /admin/users → 200 { users, total }; an absent sub
// yields "plan":"None"; per-row key order is asserted.
func TestListUsers_Shape(t *testing.T) {
	repo := &fakeAdminUserRepo{
		listRows: []UserListRow{{
			ID: "u1", Email: "a@b.c", Name: "Al", Role: "user", IsActive: true,
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}},
		total: 1,
	}
	// no active sub → plan falls to "None".
	h := newHandler(repo, &fakeSubReader{sub: nil}, &fakeUsage{count: 4}, &fakeRecent{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/users", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "users", "total")
	// per-row key order
	assertKeyOrder(t, raw, "id", "email", "name", "role", "is_active", "plan", "usage_today", "created_at")

	var body struct {
		Users []map[string]any `json:"users"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	if body.Total != 1 {
		t.Fatalf("total = %d, want 1", body.Total)
	}
	if got := body.Users[0]["plan"]; got != "None" {
		t.Fatalf("plan = %v, want \"None\" (no active sub)", got)
	}
	if got := body.Users[0]["usage_today"]; got != float64(4) {
		t.Fatalf("usage_today = %v, want 4", got)
	}
}

// TestListUsers_Defaults: absent page/limit → 1/20 reach the repo; status not
// in the allow-list → "".
func TestListUsers_Defaults(t *testing.T) {
	repo := &fakeAdminUserRepo{listRows: []UserListRow{}}
	h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/users?status=bogus", nil))

	if repo.lastList.Page != 1 || repo.lastList.Limit != 20 {
		t.Fatalf("page/limit = %d/%d, want 1/20", repo.lastList.Page, repo.lastList.Limit)
	}
	if repo.lastList.Status != "" {
		t.Fatalf("status = %q, want \"\" (bogus rejected)", repo.lastList.Status)
	}
	if repo.lastList.SortOrder != "DESC" {
		t.Fatalf("sort_order = %q, want DESC", repo.lastList.SortOrder)
	}
}

// TestListUsers_EmptyIsArray: no rows → "users":[] not null.
func TestListUsers_EmptyIsArray(t *testing.T) {
	repo := &fakeAdminUserRepo{listRows: nil, total: 0}
	h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
	if !strings.Contains(rec.Body.String(), `"users":[]`) {
		t.Fatalf("expected users:[] in %s", rec.Body.String())
	}
}

// TestGetUser_CompositeShape: top-level key order user/subscription/usage_today/
// recent_usage; subscription null when no sub; usage_today value; recent_usage []
// when empty.
func TestGetUser_CompositeShape(t *testing.T) {
	repo := &fakeAdminUserRepo{getFull: UserDetail{
		ID: "u1", Email: "a@b.c", Name: "Al", Role: "user", IsActive: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}
	h := newHandler(repo, &fakeSubReader{sub: nil}, &fakeUsage{count: 9}, &fakeRecent{rows: nil})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/users/u1", nil), "u1")
	h.GetUser(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "user", "subscription", "usage_today", "recent_usage")

	m := decodeBody(t, raw)
	if m["subscription"] != nil {
		t.Fatalf("subscription = %v, want null (no active sub)", m["subscription"])
	}
	if m["usage_today"] != float64(9) {
		t.Fatalf("usage_today = %v, want 9", m["usage_today"])
	}
	// recent_usage must be [] (non-nil slice), never null.
	if !strings.Contains(raw, `"recent_usage":[]`) {
		t.Fatalf("expected recent_usage:[] in %s", raw)
	}
	// user is the full entity (object, not null).
	if _, ok := m["user"].(map[string]any); !ok {
		t.Fatalf("user not an object: %v", m["user"])
	}
}

// TestGetUser_MissingUserIsNull: a missing user (repo ErrNotFound) → 200 with
// user:null (Node findById → null, NOT a 404). subscription/usage/recent still
// resolve.
func TestGetUser_MissingUserIsNull(t *testing.T) {
	repo := &fakeAdminUserRepo{getErr: ErrNotFound}
	h := newHandler(repo, &fakeSubReader{sub: nil}, &fakeUsage{count: 0}, &fakeRecent{})
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/users/missing", nil), "missing")
	h.GetUser(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec.Body.String())
	if m["user"] != nil {
		t.Fatalf("user = %v, want null", m["user"])
	}
}

// TestGetUser_SubViewKeyOrder: when an active sub is present, the nested
// subscription serialises in entity-declaration order (user omitted), with the
// nested plan self-marshalling.
func TestGetUser_SubViewKeyOrder(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	sub := &AdminSubView{
		ID: "s1", UserID: "u1", PlanID: "p1",
		Plan: plans.PlanEntity{
			ID: "p1", Name: "Pro", DailyLimit: 100, PriceCents: 900,
			TrialDays: 0, BillingPeriod: "monthly", IsActive: true,
			CreatedAt: now, UpdatedAt: now,
		},
		Status: "active", StoreType: "stripe", StoreTransactionID: nil,
		StartedAt: now, ExpiresAt: &exp, CreatedAt: now, UpdatedAt: now,
	}
	repo := &fakeAdminUserRepo{getFull: UserDetail{ID: "u1", CreatedAt: now, UpdatedAt: now}}
	h := newHandler(repo, &fakeSubReader{sub: sub}, &fakeUsage{count: 1}, &fakeRecent{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/users/u1", nil), "u1")
	h.GetUser(rec, req)

	raw := rec.Body.String()
	// Slice out just the subscription object so the plan's id/name don't shadow
	// the outer key checks.
	start := strings.Index(raw, `"subscription":`)
	if start < 0 {
		t.Fatalf("no subscription key in %s", raw)
	}
	subRaw := raw[start:]

	// The nested plan object reuses the same key names (id, created_at, …), so
	// strip it out before checking the OUTER subscription key order. Capture the
	// nested plan slice first for its own ordering check.
	planStart := strings.Index(subRaw, `"plan":`)
	planObjStart := strings.Index(subRaw[planStart:], "{") + planStart
	planObjEnd := strings.Index(subRaw[planObjStart:], "}") + planObjStart + 1
	planRaw := subRaw[planObjStart:planObjEnd]
	assertKeyOrder(t, planRaw,
		"id", "name", "daily_limit", "price_cents", "currency", "stripe_price_id",
		"trial_days", "billing_period", "is_active", "created_at", "updated_at")

	// Outer order: replace the nested plan object with a bare "plan" marker so the
	// repeated key names inside it don't shadow the outer scan.
	outer := subRaw[:planStart] + `"plan":null,` + subRaw[planObjEnd:]
	assertKeyOrder(t, outer,
		"id", "user_id", "plan_id", "plan", "status", "store_type",
		"store_transaction_id", "started_at", "expires_at", "created_at", "updated_at")

	// user relation must NOT appear inside the subscription object.
	if strings.Contains(outer[:strings.Index(outer, `"status"`)], `"user":`) {
		t.Fatalf("subscription must omit the user relation: %s", subRaw)
	}
}

// TestGetUser_RecentUsageRowShape: a recent_usage row serialises in the entity
// order with relations omitted.
func TestGetUser_RecentUsageRowShape(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeAdminUserRepo{getFull: UserDetail{ID: "u1", CreatedAt: now, UpdatedAt: now}}
	recent := &fakeRecent{rows: []RecentUsageRow{{
		ID: "l1", UserID: "u1", Tone: "formal", InputLength: 10, OutputLength: 12,
		AiProviderID: "ap1", ResponseTimeMs: 42, CreatedAt: now,
	}}}
	h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, recent)

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/users/u1", nil), "u1")
	h.GetUser(rec, req)

	raw := rec.Body.String()
	ru := raw[strings.Index(raw, `"recent_usage":`):]
	assertKeyOrder(t, ru,
		"id", "user_id", "tone", "input_length", "output_length",
		"ai_provider_id", "response_time_ms", "created_at")
}

// TestUpdateUser_ReturnsFullEntity: PATCH → 200, patch reaches repo, response is
// the re-read full entity.
func TestUpdateUser_ReturnsFullEntity(t *testing.T) {
	now := time.Now()
	active := false
	repo := &fakeAdminUserRepo{getFull: UserDetail{
		ID: "u1", Email: "a@b.c", Name: "Al", Role: "user", IsActive: false,
		CreatedAt: now, UpdatedAt: now,
	}}
	h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/users/u1",
		strings.NewReader(`{"is_active":false,"name":"Al"}`)), "u1")
	h.UpdateUser(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.updated.IsActive == nil || *repo.updated.IsActive != active {
		t.Fatalf("is_active patch not passed: %+v", repo.updated)
	}
	if repo.updated.Name == nil || *repo.updated.Name != "Al" {
		t.Fatalf("name patch not passed: %+v", repo.updated)
	}
	// full entity → has email_verified key (UserDetail-only field; the slim
	// list row lacks it). password_hash et al. are dropped by #31.
	m := decodeBody(t, rec.Body.String())
	if _, ok := m["email_verified"]; !ok {
		t.Fatalf("response is not the full UserDetail: %s", rec.Body.String())
	}
}

// TestUpdateUser_Validation mirrors Node's global ValidationPipe
// (whitelist + forbidNonWhitelisted) over UpdateUserDto. Each rejected
// body → 400, code invalid-input, exact constraint message; each accepted
// body → 200. Single-error cases are the realistic ones the shadow gate
// cares about byte-for-byte.
func TestUpdateUser_Validation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string // "" means expect 200
	}{
		{"role admin rejected", `{"role":"admin"}`, "role must be one of the following values: user"},
		{"role user ok", `{"role":"user"}`, ""},
		{"is_active wrong type", `{"is_active":"yes"}`, "is_active must be a boolean value"},
		{"is_active false ok", `{"is_active":false}`, ""},
		{"name wrong type", `{"name":123}`, "name must be a string"},
		{"unknown key", `{"foo":"bar"}`, "property foo should not exist"},
		{"all valid", `{"name":"Tan","is_active":true,"role":"user"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			repo := &fakeAdminUserRepo{getFull: UserDetail{
				ID: "u1", Email: "a@b.c", Name: "Al", Role: "user", IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}}
			h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})

			rec := httptest.NewRecorder()
			req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/users/u1",
				strings.NewReader(tc.body)), "u1")
			h.UpdateUser(rec, req)

			if tc.wantErr == "" {
				if rec.Code != 200 {
					t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			m := decodeBody(t, rec.Body.String())
			if m["code"] != "invalid-input" {
				t.Fatalf("code = %v, want invalid-input; body=%s", m["code"], rec.Body.String())
			}
			if m["error"] != tc.wantErr {
				t.Fatalf("error = %q, want %q", m["error"], tc.wantErr)
			}
		})
	}
}

// TestUpdateUser_MalformedBody: a body that is malformed JSON or non-object
// JSON (incl. null) → 400 invalid-input "Invalid request body" — NOT a silent
// 200 no-op write. Regression guard for c8c32f75. A valid empty object {} is
// NOT malformed: all DTO fields are optional, so it proceeds → 200 full entity.
func TestUpdateUser_MalformedBody(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode int
	}{
		{"malformed brace", `{bad`, 400},
		{"non-object array", `[]`, 400},
		{"non-object string", `"x"`, 400},
		{"non-object number", `123`, 400},
		{"non-object bool", `true`, 400},
		{"null body", `null`, 400},
		{"empty body", ``, 400},
		{"valid empty object", `{}`, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			repo := &fakeAdminUserRepo{getFull: UserDetail{
				ID: "u1", Email: "a@b.c", Name: "Al", Role: "user", IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}}
			h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})

			rec := httptest.NewRecorder()
			req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/users/u1",
				strings.NewReader(tc.body)), "u1")
			h.UpdateUser(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantCode == 400 {
				m := decodeBody(t, rec.Body.String())
				if m["code"] != "invalid-input" {
					t.Fatalf("code = %v, want invalid-input; body=%s", m["code"], rec.Body.String())
				}
				if m["error"] != "Invalid request body" {
					t.Fatalf("error = %q, want %q", m["error"], "Invalid request body")
				}
			}
		})
	}
}

// TestUpdateUser_FullEntityKeyOrder: a valid PATCH returns the re-read full
// entity (verified richer than UserListRow via email_verified) — guards the
// success path still proceeds after validation.
func TestUpdateUser_FullEntityKeyOrder(t *testing.T) {
	now := time.Now()
	repo := &fakeAdminUserRepo{getFull: UserDetail{
		ID: "u1", Email: "a@b.c", Name: "Tan", Role: "user", IsActive: true,
		CreatedAt: now, UpdatedAt: now,
	}}
	h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/users/u1",
		strings.NewReader(`{"name":"Tan","is_active":true,"role":"user"}`)), "u1")
	h.UpdateUser(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.updated.Name == nil || *repo.updated.Name != "Tan" {
		t.Fatalf("name patch not passed: %+v", repo.updated)
	}
	if repo.updated.Role == nil || *repo.updated.Role != "user" {
		t.Fatalf("role patch not passed: %+v", repo.updated)
	}
	if repo.updated.IsActive == nil || *repo.updated.IsActive != true {
		t.Fatalf("is_active patch not passed: %+v", repo.updated)
	}
	m := decodeBody(t, rec.Body.String())
	if _, ok := m["email_verified"]; !ok {
		t.Fatalf("response is not the full UserDetail: %s", rec.Body.String())
	}
}

// TestUpdateUser_MissingIs500: re-read ErrNotFound → 500 internal (Node
// findOneOrFail parity, NOT a 404).
func TestUpdateUser_MissingIs500(t *testing.T) {
	repo := &fakeAdminUserRepo{getErr: ErrNotFound}
	h := newHandler(repo, &fakeSubReader{}, &fakeUsage{}, &fakeRecent{})
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/users/x",
		strings.NewReader(`{"name":"x"}`)), "x")
	h.UpdateUser(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}
