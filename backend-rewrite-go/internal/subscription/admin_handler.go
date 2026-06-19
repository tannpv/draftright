// admin_handler.go — HTTP edge for the admin transactions list route.
//
//	GET /admin/transactions?page=&limit=&search= → 200 { transactions, total }
//
// Mounts inside the admin group (jwtMW → RequireAdmin) wired in the router
// task; the handler itself does no auth. This is SEPARATE from the
// customer-facing handler.go in this package — leave that one alone.
//
// Node parity (admin.controller.ts listTransactions → subscriptionsService
// .findAllPaginated):
//   - page:  parseInt || 1   (Node: page ? parseInt(page) : 1)
//   - limit: parseInt || 20  (bespoke default — NOT listquery)
//   - search: raw passthrough; "" → Reader maps to NULL → match-all
//   - 500 message: hardcoded generic "transactions failed" (the dominant,
//     shadow-verified admin-handler convention — plans→"plans failed",
//     user→"users failed"). Node forwards err.message, but that text is
//     driver-specific and un-byte-matchable; the generic string is what
//     passed the shadow gates in Phase 0-4b.
package subscription

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminTransactionsService is the handler's consumer-side port; *Reader
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject
// a fake without a DB.
type adminTransactionsService interface {
	FindAllPaginated(ctx context.Context, q TxQuery) (TxPage, error)
}

// AdminHandler serves GET /admin/transactions.
type AdminHandler struct {
	svc adminTransactionsService
}

// NewAdminHandler wires the reader.
func NewAdminHandler(svc *Reader) *AdminHandler { return &AdminHandler{svc: svc} }

// transactionsResponse is the { transactions, total } body. Field order
// (transactions first, total second) matches Node's listTransactions return.
// TransactionRow carries its own MarshalJSON pinning the row key order.
type transactionsResponse struct {
	Transactions []TransactionRow `json:"transactions"`
	Total        int              `json:"total"`
}

// ListTransactions handles GET /admin/transactions → 200 { transactions, total }.
//
// Query params (bespoke parse, NOT listquery):
//   - page:  parseInt || 1
//   - limit: parseInt || 20
//   - search: raw passthrough ("" → Reader maps to NULL → match-all)
//
// A nil slice from the reader is normalized to [] so JSON never emits null.
func (h *AdminHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := 1
	if v, err := strconv.Atoi(q.Get("page")); err == nil {
		page = v
	}
	limit := 20
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = v
	}

	res, err := h.svc.FindAllPaginated(r.Context(), TxQuery{
		Search: q.Get("search"),
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		shared.WriteError(w, r, "internal", "transactions failed")
		return
	}
	txs := res.Transactions
	if txs == nil {
		txs = []TransactionRow{}
	}
	shared.WriteJSON(w, http.StatusOK, transactionsResponse{Transactions: txs, Total: res.Total})
}

// grantService is the grant handler's consumer-side port; *Writer satisfies
// it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject a fake
// without a DB.
type grantService interface {
	Grant(ctx context.Context, userID, planID string, expiresAt *time.Time) (GrantedSub, error)
}

// GrantHandler serves POST /admin/subscriptions/grant.
type GrantHandler struct {
	svc grantService
}

// NewGrantHandler wires the writer.
func NewGrantHandler(svc *Writer) *GrantHandler { return &GrantHandler{svc: svc} }

// Grant handles POST /admin/subscriptions/grant → 201 GrantedSub (the full
// inserted Subscription row). Mirrors admin.controller.ts grantSubscription:
// validate GrantSubscriptionDto, parse expires_at, then
// subscriptionsService.grant(user_id, plan_id, expiresAt).
//
// Validation mirrors the global ValidationPipe (whitelist +
// forbidNonWhitelisted) over GrantSubscriptionDto: a malformed/non-object body
// → 400 invalid-input "Invalid request body"; a bad UUID / non-ISO expires_at
// / unknown key → 400 invalid-input with the class-validator message(s) joined
// by ". ". A repo error → 500 internal.
func (h *GrantHandler) Grant(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	in, malformed, msg := validateGrant(raw)
	if malformed {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}
	sub, err := h.svc.Grant(r.Context(), in.UserID, in.PlanID, in.ExpiresAt)
	if err != nil {
		shared.WriteError(w, r, "internal", "transactions failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, sub)
}
