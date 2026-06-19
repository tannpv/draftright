package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeTxService is a test double satisfying the AdminHandler's
// adminTransactionsService port. It captures the TxQuery it received and
// returns canned results.
type fakeTxService struct {
	page   TxPage
	err    error
	lastQ  TxQuery
	called bool
}

func (f *fakeTxService) FindAllPaginated(ctx context.Context, q TxQuery) (TxPage, error) {
	f.called = true
	f.lastQ = q
	return f.page, f.err
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

func ptrStr(s string) *string        { return &s }
func ptrTime(t time.Time) *time.Time { return &t }

// ---------------------------------------------------------------------------
// Happy path + mapping order
// ---------------------------------------------------------------------------

// TestAdminListTransactions_Shape: GET /admin/transactions → 200
// { transactions, total } in that order; the row's keys are in spec order.
func TestAdminListTransactions_Shape(t *testing.T) {
	started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := &fakeTxService{
		page: TxPage{
			Transactions: []TransactionRow{{
				ID:                 "tx-1",
				UserEmail:          "u@example.com",
				UserName:           "User One",
				UserID:             "user-1",
				PlanName:           "Pro",
				PriceCents:         999,
				StoreType:          "stripe",
				StoreTransactionID: ptrStr("sub_123"),
				Status:             "active",
				StartedAt:          ptrTime(started),
				ExpiresAt:          nil, // spot-check expires_at:null
				CreatedAt:          ptrTime(created),
			}},
			Total: 1,
		},
	}
	h := &AdminHandler{svc: svc}

	rec := httptest.NewRecorder()
	h.ListTransactions(rec, httptest.NewRequest(http.MethodGet, "/admin/transactions", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	// top-level order: transactions then total
	assertKeyOrder(t, raw, "transactions", "total")
	// row key order (spec)
	assertKeyOrder(t, raw,
		"id", "user_email", "user_name", "user_id", "plan_name",
		"price_cents", "store_type", "store_transaction_id", "status",
		"started_at", "expires_at", "created_at")

	if !strings.Contains(raw, `"expires_at":null`) {
		t.Fatalf("expected expires_at:null in %s", raw)
	}
	if !strings.Contains(raw, `"store_transaction_id":"sub_123"`) {
		t.Fatalf("expected store_transaction_id:sub_123 in %s", raw)
	}

	var body struct {
		Transactions []map[string]any `json:"transactions"`
		Total        int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	if body.Total != 1 {
		t.Fatalf("total = %d, want 1", body.Total)
	}
	if len(body.Transactions) != 1 {
		t.Fatalf("len(transactions) = %d, want 1", len(body.Transactions))
	}
}

// ---------------------------------------------------------------------------
// Empty → [] not null
// ---------------------------------------------------------------------------

// TestAdminListTransactions_EmptyIsArray: nil transactions → "transactions":[]
// (not null) and "total":0.
func TestAdminListTransactions_EmptyIsArray(t *testing.T) {
	svc := &fakeTxService{page: TxPage{Transactions: nil, Total: 0}}
	h := &AdminHandler{svc: svc}

	rec := httptest.NewRecorder()
	h.ListTransactions(rec, httptest.NewRequest(http.MethodGet, "/admin/transactions", nil))

	raw := rec.Body.String()
	if !strings.Contains(raw, `"transactions":[]`) {
		t.Fatalf("expected transactions:[] in %s", raw)
	}
	if !strings.Contains(raw, `"total":0`) {
		t.Fatalf("expected total:0 in %s", raw)
	}
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

// TestAdminListTransactions_Defaults: absent params → Page=1, Limit=20, Search="".
func TestAdminListTransactions_Defaults(t *testing.T) {
	svc := &fakeTxService{}
	h := &AdminHandler{svc: svc}

	rec := httptest.NewRecorder()
	h.ListTransactions(rec, httptest.NewRequest(http.MethodGet, "/admin/transactions", nil))

	want := TxQuery{Page: 1, Limit: 20, Search: ""}
	if svc.lastQ != want {
		t.Fatalf("query = %+v, want %+v", svc.lastQ, want)
	}
}

// ---------------------------------------------------------------------------
// Param parse
// ---------------------------------------------------------------------------

// TestAdminListTransactions_Params: ?page=3&limit=5&search=foo forwarded.
func TestAdminListTransactions_Params(t *testing.T) {
	svc := &fakeTxService{}
	h := &AdminHandler{svc: svc}

	rec := httptest.NewRecorder()
	h.ListTransactions(rec, httptest.NewRequest(http.MethodGet,
		"/admin/transactions?page=3&limit=5&search=foo", nil))

	want := TxQuery{Page: 3, Limit: 5, Search: "foo"}
	if svc.lastQ != want {
		t.Fatalf("query = %+v, want %+v", svc.lastQ, want)
	}
}

// ---------------------------------------------------------------------------
// Bad params fall back
// ---------------------------------------------------------------------------

// TestAdminListTransactions_BadParams: ?page=abc&limit=xyz → fall back to 1/20.
func TestAdminListTransactions_BadParams(t *testing.T) {
	svc := &fakeTxService{}
	h := &AdminHandler{svc: svc}

	rec := httptest.NewRecorder()
	h.ListTransactions(rec, httptest.NewRequest(http.MethodGet,
		"/admin/transactions?page=abc&limit=xyz", nil))

	want := TxQuery{Page: 1, Limit: 20, Search: ""}
	if svc.lastQ != want {
		t.Fatalf("query = %+v, want %+v", svc.lastQ, want)
	}
}

// ---------------------------------------------------------------------------
// Service error → 500 internal
// ---------------------------------------------------------------------------

// TestAdminListTransactions_ServiceError: service error → 500 internal.
func TestAdminListTransactions_ServiceError(t *testing.T) {
	svc := &fakeTxService{err: errors.New("db down")}
	h := &AdminHandler{svc: svc}

	rec := httptest.NewRecorder()
	h.ListTransactions(rec, httptest.NewRequest(http.MethodGet, "/admin/transactions", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if m["code"] != "internal" {
		t.Fatalf("code = %v, want internal; body=%s", m["code"], rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /admin/subscriptions/grant
// ---------------------------------------------------------------------------

// fakeGrantService satisfies the grant handler's consumer-side port. It
// captures the args it received and returns a canned GrantedSub.
type fakeGrantService struct {
	row        GrantedSub
	err        error
	called     bool
	gotUserID  string
	gotPlanID  string
	gotExpires *time.Time
}

func (f *fakeGrantService) Grant(ctx context.Context, userID, planID string, expiresAt *time.Time) (GrantedSub, error) {
	f.called = true
	f.gotUserID = userID
	f.gotPlanID = planID
	f.gotExpires = expiresAt
	return f.row, f.err
}

func newGrantHandler(svc grantService) *GrantHandler { return &GrantHandler{svc: svc} }

// TestGrant_Created: valid UUIDs, no expires_at → 201 GrantedSub, expires_at nil.
func TestGrant_Created(t *testing.T) {
	svc := &fakeGrantService{row: GrantedSub{
		ID:        "33333333-3333-3333-8333-333333333333",
		UserID:    "11111111-1111-1111-8111-111111111111",
		PlanID:    "22222222-2222-2222-8222-222222222222",
		Status:    "active",
		StoreType: "admin_granted",
	}}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111","plan_id":"22222222-2222-2222-8222-222222222222"}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !svc.called {
		t.Fatal("grant service not called")
	}
	if svc.gotUserID != "11111111-1111-1111-8111-111111111111" ||
		svc.gotPlanID != "22222222-2222-2222-8222-222222222222" {
		t.Fatalf("forwarded ids wrong: %q %q", svc.gotUserID, svc.gotPlanID)
	}
	if svc.gotExpires != nil {
		t.Fatalf("expires_at = %v, want nil", svc.gotExpires)
	}
	raw := rec.Body.String()
	// Node's subscriptionsService.grant() does create()+save() WITHOUT
	// store_transaction_id, so the returned entity has it === undefined and
	// JSON.stringify OMITS the key. expires_at IS explicitly set to null
	// (expiresAt || null), so it is present as null.
	assertKeyOrder(t, raw,
		"id", "user_id", "plan_id", "status", "store_type",
		"started_at", "expires_at", "created_at", "updated_at")
	if strings.Contains(raw, "store_transaction_id") {
		t.Fatalf("store_transaction_id must be omitted (Node grant() never sets it): %s", raw)
	}
	if !strings.Contains(raw, `"expires_at":null`) {
		t.Fatalf("expires_at must be present as null: %s", raw)
	}
}

// TestGrant_ExpiresAtParsed: an ISO 8601 expires_at is parsed and forwarded.
func TestGrant_ExpiresAtParsed(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111","plan_id":"22222222-2222-2222-8222-222222222222","expires_at":"2026-12-31T00:00:00.000Z"}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	want := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	if svc.gotExpires == nil || !svc.gotExpires.Equal(want) {
		t.Fatalf("expires_at = %v, want %v", svc.gotExpires, want)
	}
}

// TestGrant_BadUUID: invalid user_id → 400 invalid-input, verbatim message.
func TestGrant_BadUUID(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"not-a-uuid","plan_id":"22222222-2222-2222-8222-222222222222"}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if m["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input; body=%s", m["code"], rec.Body.String())
	}
	if m["error"] != "user_id must be a UUID" {
		t.Fatalf("error = %v, want %q", m["error"], "user_id must be a UUID")
	}
	if svc.called {
		t.Fatal("grant service should not be called on validation failure")
	}
}

// TestGrant_MissingPlanID: absent plan_id (required @IsUUID) → 400, UUID message.
func TestGrant_MissingPlanID(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111"}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["error"] != "plan_id must be a UUID" {
		t.Fatalf("error = %v, want %q", m["error"], "plan_id must be a UUID")
	}
}

// TestGrant_BadDateString: present but non-ISO expires_at → 400, IsDateString msg.
func TestGrant_BadDateString(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111","plan_id":"22222222-2222-2222-8222-222222222222","expires_at":"nope"}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["error"] != "expires_at must be a valid ISO 8601 date string" {
		t.Fatalf("error = %v, want %q", m["error"], "expires_at must be a valid ISO 8601 date string")
	}
}

// TestGrant_UnknownKey: forbidNonWhitelisted → 400, "property X should not exist".
func TestGrant_UnknownKey(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111","plan_id":"22222222-2222-2222-8222-222222222222","bogus":1}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["error"] != "property bogus should not exist" {
		t.Fatalf("error = %v, want %q", m["error"], "property bogus should not exist")
	}
}

// TestGrant_UnknownKeyWithBadFields: forbidNonWhitelisted errors precede the
// field-constraint errors. NestJS ValidationPipe (whitelist +
// forbidNonWhitelisted) emits the "property X should not exist" errors FIRST,
// then the field errors in declaration order (user_id, plan_id, expires_at).
// Verified empirically against the real DTO + class-validator 0.14.4.
func TestGrant_UnknownKeyWithBadFields(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"x","plan_id":"y","expires_at":"nope","bogus":1}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	want := "property bogus should not exist. user_id must be a UUID. plan_id must be a UUID. expires_at must be a valid ISO 8601 date string"
	if m["error"] != want {
		t.Fatalf("error = %v, want %q", m["error"], want)
	}
	if svc.called {
		t.Fatal("grant service should not be called on validation failure")
	}
}

// TestGrant_TwoUnknownKeys: multiple forbidNonWhitelisted errors preserve JSON
// source order. NestJS reports unknown keys in body insertion order (zeta
// before alpha here), not Go map-iteration order.
func TestGrant_TwoUnknownKeys(t *testing.T) {
	svc := &fakeGrantService{}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111","plan_id":"22222222-2222-2222-8222-222222222222","zeta":1,"alpha":2}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	want := "property zeta should not exist. property alpha should not exist"
	if m["error"] != want {
		t.Fatalf("error = %v, want %q", m["error"], want)
	}
}

// TestGrant_ServiceError: repo error → 500 internal.
func TestGrant_ServiceError(t *testing.T) {
	svc := &fakeGrantService{err: errors.New("db down")}
	h := newGrantHandler(svc)

	body := `{"user_id":"11111111-1111-1111-8111-111111111111","plan_id":"22222222-2222-2222-8222-222222222222"}`
	rec := httptest.NewRecorder()
	h.Grant(rec, httptest.NewRequest(http.MethodPost, "/admin/subscriptions/grant", strings.NewReader(body)))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["code"] != "internal" {
		t.Fatalf("code = %v, want internal; body=%s", m["code"], rec.Body.String())
	}
}

// TestValidateGrant_UUIDVariants: @IsUUID() (no version) accepts v1-v8/nil/max.
func TestValidateGrant_UUIDVariants(t *testing.T) {
	// v1 UUID — rejected by @IsUUID('4') but accepted by the version-less
	// @IsUUID() the grant DTO uses ('all' regex).
	body := `{"user_id":"550e8400-e29b-11d4-a716-446655440000","plan_id":"22222222-2222-2222-8222-222222222222"}`
	_, _, msg := validateGrant([]byte(body))
	if msg != "" {
		t.Fatalf("v1 UUID rejected: %q", msg)
	}
}
