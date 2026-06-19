// HTTP edge for GET /admin/email-logs (bespoke def-50/max-200). Mounts inside
// the admin group (jwtMW → RequireAdmin) wired in the router task; the handler
// does no auth. Wiring into main.go is a LATER task.
//
//	GET /admin/email-logs?page=&limit=&status= → { rows, total } → 200
package email

import (
	"context"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// emailLogsLimit mirrors Math.min(parseInt(q.limit) || 50, 200). JS `||` treats
// only 0 and NaN as falsy, so negatives pass through unchanged ("-1" → -1).
func emailLogsLimit(raw string) int {
	n, ok := listquery.JSParseInt(raw)
	if !ok || n == 0 {
		n = 50
	}
	if n > 200 {
		n = 200
	}
	return n
}

// emailLogsPage mirrors Math.max(parseInt(q.page) || 1, 1) → always ≥ 1.
func emailLogsPage(raw string) int {
	n, ok := listquery.JSParseInt(raw)
	if !ok || n < 1 {
		n = 1
	}
	return n
}

// adminLogsList is the handler's consumer-side port; *AdminLogsService
// satisfies it. Kept on the consumer side so tests inject a fake without a DB.
type adminLogsList interface {
	List(ctx context.Context, p AdminLogsParams) ([]EmailLogRow, int, error)
}

// AdminLogsHandler serves the admin email-logs route.
type AdminLogsHandler struct {
	svc adminLogsList
}

// NewAdminLogsHandler wires the service.
func NewAdminLogsHandler(svc *AdminLogsService) *AdminLogsHandler {
	return &AdminLogsHandler{svc: svc}
}

// emailLogsResponse is the { rows, total } body. Field order = key order (rows,
// total), matching Node's emailLogs return. Rows must be [] not null when empty.
type emailLogsResponse struct {
	Rows  []EmailLogRow `json:"rows"`
	Total int           `json:"total"`
}

// List handles GET /admin/email-logs. Bespoke parse → svc.List → { rows, total }.
func (h *AdminLogsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	rows, total, err := h.svc.List(r.Context(), AdminLogsParams{
		Status: q.Get("status"),
		Page:   emailLogsPage(q.Get("page")),
		Limit:  emailLogsLimit(q.Get("limit")),
	})
	if err != nil {
		shared.WriteError(w, r, "internal", "email logs failed")
		return
	}
	if rows == nil {
		rows = []EmailLogRow{}
	}
	shared.WriteJSON(w, http.StatusOK, emailLogsResponse{Rows: rows, Total: total})
}
