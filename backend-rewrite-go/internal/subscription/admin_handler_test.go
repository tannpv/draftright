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
