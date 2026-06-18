package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// fakeAdminPaymentSvc satisfies adminPaymentService. It captures the args it
// receives and returns injected values/errors so the handler tests can assert
// on query parsing, status codes, and error mapping without a DB.
type fakeAdminPaymentSvc struct {
	stats    PaymentStats
	statsErr error

	findAllParam FindAllParams // captured
	findAllPage  PaymentsPage
	findAllErr   error

	confirmID    string // captured
	confirmNotes string // captured
	confirmRow   AdminPaymentDetailRow
	confirmErr   error

	refundID     string // captured
	refundReason string // captured
	refundRow    AdminPaymentDetailRow
	refundErr    error
}

func (f *fakeAdminPaymentSvc) GetStats(ctx context.Context) (PaymentStats, error) {
	return f.stats, f.statsErr
}
func (f *fakeAdminPaymentSvc) FindAll(ctx context.Context, p FindAllParams) (PaymentsPage, error) {
	f.findAllParam = p
	return f.findAllPage, f.findAllErr
}
func (f *fakeAdminPaymentSvc) AdminConfirm(ctx context.Context, id, notes string) (AdminPaymentDetailRow, error) {
	f.confirmID, f.confirmNotes = id, notes
	return f.confirmRow, f.confirmErr
}
func (f *fakeAdminPaymentSvc) Refund(ctx context.Context, id, reason string) (AdminPaymentDetailRow, error) {
	f.refundID, f.refundReason = id, reason
	return f.refundRow, f.refundErr
}

// handlerWith builds an AdminHandler over a fake (bypasses NewAdminHandler's
// *AdminService signature so the fake can be injected directly).
func handlerWith(f *fakeAdminPaymentSvc) *AdminHandler { return &AdminHandler{svc: f} }

// withChiID injects the chi :id URL param on a request.
func withChiID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// decodeErr parses the canonical error envelope.
type errEnvelope struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

func decodeErr(t *testing.T, body string) errEnvelope {
	t.Helper()
	var e errEnvelope
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("body not error envelope: %v (%s)", err, body)
	}
	return e
}

// --- Stats ---

func TestHandlerStats_OK(t *testing.T) {
	f := &fakeAdminPaymentSvc{stats: PaymentStats{Total: 5, Completed: 3, Pending: 2, Revenue: 1200}}
	rec := httptest.NewRecorder()
	handlerWith(f).Stats(rec, httptest.NewRequest(http.MethodGet, "/admin/payments/stats", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
		Pending   int `json:"pending"`
		Revenue   int `json:"revenue"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if body != (struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
		Pending   int `json:"pending"`
		Revenue   int `json:"revenue"`
	}{5, 3, 2, 1200}) {
		t.Fatalf("body = %+v", body)
	}
}

func TestHandlerStats_Error(t *testing.T) {
	f := &fakeAdminPaymentSvc{statsErr: errors.New("boom")}
	rec := httptest.NewRecorder()
	handlerWith(f).Stats(rec, httptest.NewRequest(http.MethodGet, "/admin/payments/stats", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "stats failed" || e.Code != "internal" {
		t.Fatalf("envelope = %+v", e)
	}
}

// --- ListPayments ---

func TestHandlerListPayments_NoQueryDefaults(t *testing.T) {
	f := &fakeAdminPaymentSvc{findAllPage: PaymentsPage{Payments: []AdminPaymentRow{}, Total: 0}}
	rec := httptest.NewRecorder()
	handlerWith(f).ListPayments(rec, httptest.NewRequest(http.MethodGet, "/admin/payments", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	p := f.findAllParam
	if p.Page != 1 || p.Limit != 20 || p.Status != "" || p.Search != "" || p.SortBy != "" || p.SortOrder != "DESC" {
		t.Fatalf("params = %+v, want {Page:1 Limit:20 Status:\"\" Search:\"\" SortBy:\"\" SortOrder:DESC}", p)
	}
}

func TestHandlerListPayments_FullQuery(t *testing.T) {
	f := &fakeAdminPaymentSvc{findAllPage: PaymentsPage{Payments: []AdminPaymentRow{}, Total: 3}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/admin/payments?page=2&limit=50&status=completed&search=foo&sort_by=amount&sort_order=asc", nil)
	handlerWith(f).ListPayments(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	p := f.findAllParam
	if p.Page != 2 || p.Limit != 50 || p.Status != "completed" || p.Search != "foo" || p.SortBy != "amount" || p.SortOrder != "ASC" {
		t.Fatalf("params = %+v", p)
	}
}

func TestHandlerListPayments_SortOrderNormalisation(t *testing.T) {
	cases := map[string]string{
		"sort_order=DESC": "DESC",
		"sort_order=asc":  "ASC",
		"sort_order=ASC":  "ASC",
		"sort_order=xyz":  "DESC",
		"":                "DESC", // absent
	}
	for query, want := range cases {
		f := &fakeAdminPaymentSvc{findAllPage: PaymentsPage{Payments: []AdminPaymentRow{}}}
		rec := httptest.NewRecorder()
		url := "/admin/payments"
		if query != "" {
			url += "?" + query
		}
		handlerWith(f).ListPayments(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if got := f.findAllParam.SortOrder; got != want {
			t.Fatalf("query %q: SortOrder = %q, want %q", query, got, want)
		}
	}
}

func TestHandlerListPayments_EmptyPaymentsIsArray(t *testing.T) {
	// FindAll already normalises nil→[]; the fake returns an empty slice so the
	// handler must emit "payments":[] (never null) with key order payments,total.
	f := &fakeAdminPaymentSvc{findAllPage: PaymentsPage{Payments: []AdminPaymentRow{}, Total: 4}}
	rec := httptest.NewRecorder()
	handlerWith(f).ListPayments(rec, httptest.NewRequest(http.MethodGet, "/admin/payments", nil))

	raw := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(raw, `"payments":[]`) {
		t.Fatalf("body = %s, want payments:[]", raw)
	}
	pIdx := strings.Index(raw, `"payments"`)
	tIdx := strings.Index(raw, `"total"`)
	if pIdx < 0 || tIdx < 0 || pIdx >= tIdx {
		t.Fatalf("key order wrong: %s", raw)
	}
}

func TestHandlerListPayments_Error(t *testing.T) {
	f := &fakeAdminPaymentSvc{findAllErr: errors.New("boom")}
	rec := httptest.NewRecorder()
	handlerWith(f).ListPayments(rec, httptest.NewRequest(http.MethodGet, "/admin/payments", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "payments failed" || e.Code != "internal" {
		t.Fatalf("envelope = %+v", e)
	}
}

// --- Confirm ---

func TestHandlerConfirm_Created(t *testing.T) {
	f := &fakeAdminPaymentSvc{confirmRow: AdminPaymentDetailRow{ID: "p1", UserID: "u1", PlanID: "pl1"}}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p1/confirm",
		strings.NewReader(`{"notes":"x"}`)), "p1")
	handlerWith(f).Confirm(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if f.confirmID != "p1" || f.confirmNotes != "x" {
		t.Fatalf("captured id=%q notes=%q", f.confirmID, f.confirmNotes)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, `"user"`) {
		t.Fatalf("body must NOT contain user key: %s", raw)
	}
	if !strings.Contains(raw, `"id":"p1"`) {
		t.Fatalf("body missing row id: %s", raw)
	}
}

func TestHandlerConfirm_EmptyBodyNoNotes(t *testing.T) {
	f := &fakeAdminPaymentSvc{confirmRow: AdminPaymentDetailRow{ID: "p1"}}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p1/confirm", nil), "p1")
	handlerWith(f).Confirm(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if f.confirmNotes != "" {
		t.Fatalf("notes = %q, want empty", f.confirmNotes)
	}
}

func TestHandlerConfirm_NotFound(t *testing.T) {
	f := &fakeAdminPaymentSvc{confirmErr: ErrPaymentNotFound}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p1/confirm", nil), "p1")
	handlerWith(f).Confirm(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "Payment not found" || e.Code != "not-found" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestHandlerConfirm_NotPending(t *testing.T) {
	f := &fakeAdminPaymentSvc{confirmErr: ErrPaymentNotPending}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p1/confirm", nil), "p1")
	handlerWith(f).Confirm(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "Payment is not pending" || e.Code != "invalid-input" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestHandlerConfirm_GenericError(t *testing.T) {
	f := &fakeAdminPaymentSvc{confirmErr: errors.New("boom")}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p1/confirm", nil), "p1")
	handlerWith(f).Confirm(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "confirm failed" || e.Code != "internal" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestHandlerConfirm_MalformedBody(t *testing.T) {
	f := &fakeAdminPaymentSvc{}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p1/confirm",
		strings.NewReader(`{bad`)), "p1")
	handlerWith(f).Confirm(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "Invalid request body" || e.Code != "invalid-input" {
		t.Fatalf("envelope = %+v", e)
	}
}

// --- Refund ---

func TestHandlerRefund_Created(t *testing.T) {
	f := &fakeAdminPaymentSvc{refundRow: AdminPaymentDetailRow{ID: "p2"}}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p2/refund",
		strings.NewReader(`{"reason":"y"}`)), "p2")
	handlerWith(f).Refund(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if f.refundID != "p2" || f.refundReason != "y" {
		t.Fatalf("captured id=%q reason=%q", f.refundID, f.refundReason)
	}
	if !strings.Contains(rec.Body.String(), `"id":"p2"`) {
		t.Fatalf("body missing row id: %s", rec.Body.String())
	}
}

func TestHandlerRefund_EmptyBodyNoReason(t *testing.T) {
	f := &fakeAdminPaymentSvc{refundRow: AdminPaymentDetailRow{ID: "p2"}}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p2/refund", nil), "p2")
	handlerWith(f).Refund(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if f.refundReason != "" {
		t.Fatalf("reason = %q, want empty", f.refundReason)
	}
}

func TestHandlerRefund_NotFound(t *testing.T) {
	f := &fakeAdminPaymentSvc{refundErr: ErrPaymentNotFound}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p2/refund", nil), "p2")
	handlerWith(f).Refund(rec, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "Payment not found" || e.Code != "not-found" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestHandlerRefund_NotCompleted(t *testing.T) {
	f := &fakeAdminPaymentSvc{refundErr: ErrRefundNotCompleted}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p2/refund", nil), "p2")
	handlerWith(f).Refund(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "Only completed payments can be refunded" || e.Code != "invalid-input" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestHandlerRefund_UnsupportedMethod(t *testing.T) {
	f := &fakeAdminPaymentSvc{refundErr: badRequest("Refund not supported for method 'paypal'")}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p2/refund", nil), "p2")
	handlerWith(f).Refund(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "Refund not supported for method 'paypal'" || e.Code != "invalid-input" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestHandlerRefund_GenericError(t *testing.T) {
	f := &fakeAdminPaymentSvc{refundErr: errors.New("boom")}
	rec := httptest.NewRecorder()
	req := withChiID(httptest.NewRequest(http.MethodPost, "/admin/payments/p2/refund", nil), "p2")
	handlerWith(f).Refund(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	e := decodeErr(t, rec.Body.String())
	if e.Error != "refund failed" || e.Code != "internal" {
		t.Fatalf("envelope = %+v", e)
	}
}
