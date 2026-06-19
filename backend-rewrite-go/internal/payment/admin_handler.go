package payment

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminPaymentService is the AdminHandler's consumer-side port; *AdminService
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject
// a fake.
type adminPaymentService interface {
	GetStats(ctx context.Context) (PaymentStats, error)
	FindAll(ctx context.Context, p FindAllParams) (PaymentsPage, error)
	AdminConfirm(ctx context.Context, id, notes string) (AdminPaymentDetailRow, error)
	Refund(ctx context.Context, id, reason string) (AdminPaymentDetailRow, error)
}

// AdminHandler serves the admin payment routes. All routes mount inside the
// admin group (jwtMW → RequireAdmin) wired in the router task; the handler
// itself does no auth.
//
//	GET  /admin/payments/stats        → 200 PaymentStats
//	GET  /admin/payments              → 200 PaymentsPage { payments, total }
//	POST /admin/payments/:id/confirm  → 201 AdminPaymentDetailRow
//	POST /admin/payments/:id/refund   → 201 AdminPaymentDetailRow
type AdminHandler struct {
	svc adminPaymentService
}

// NewAdminHandler wires the service.
func NewAdminHandler(svc *AdminService) *AdminHandler { return &AdminHandler{svc: svc} }

// confirmBody / refundBody decode the OPTIONAL POST bodies. Node's
// `@Body() { notes?: string }` / `{ reason?: string }` tolerate an empty or
// missing body — an absent key leaves the field "" (matching body.notes ===
// undefined → adminConfirm's default). Only malformed JSON 400s.
type confirmBody struct {
	Notes string `json:"notes"`
}
type refundBody struct {
	Reason string `json:"reason"`
}

// Stats handles GET /admin/payments/stats → 200 PaymentStats.
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "stats failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, stats)
}

// ListPayments handles GET /admin/payments → 200 PaymentsPage. Mirrors Node's
// listPayments: page/limit default to 1/20, status/search/sort_by are raw
// strings ("" when absent), sort_order is "ASC" iff it case-insensitively
// equals "ASC" else "DESC". The page/limit parse uses Atoi-or-default (the
// sibling subscription ListTransactions convention) — it diverges from JS
// parseInt only on non-numeric-non-empty input (e.g. ?page=abc), an accepted
// minor edge.
func (h *AdminHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, err := strconv.Atoi(q.Get("page"))
	if err != nil {
		page = 1
	}
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		limit = 20
	}

	order := "DESC"
	if strings.EqualFold(q.Get("sort_order"), "ASC") {
		order = "ASC"
	}

	pageOut, err := h.svc.FindAll(r.Context(), FindAllParams{
		Page:      page,
		Limit:     limit,
		Status:    q.Get("status"),
		Search:    q.Get("search"),
		SortBy:    q.Get("sort_by"),
		SortOrder: order,
	})
	if err != nil {
		shared.WriteError(w, r, "internal", "payments failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, pageOut)
}

// Confirm handles POST /admin/payments/:id/confirm → 201. Optional body
// { notes? }. Known *DomainError (404 not-found / 400 invalid-input) renders
// via writePaymentErr; anything else falls through to 500.
func (h *AdminHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body confirmBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	row, err := h.svc.AdminConfirm(r.Context(), id, body.Notes)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "confirm failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, row)
}

// Refund handles POST /admin/payments/:id/refund → 201. Optional body
// { reason? }. Same error mapping as Confirm.
func (h *AdminHandler) Refund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body refundBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	row, err := h.svc.Refund(r.Context(), id, body.Reason)
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "refund failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, row)
}
