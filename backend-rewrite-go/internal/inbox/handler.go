// handler.go — HTTP edge for the admin inbox routes (D2). Mounts inside the
// admin group (jwtMW → RequireAdmin) wired in the router task.
//
//	GET /admin/inbox/counts → 200 { new_bugs, new_features, new_errors, total }
//	GET /admin/inbox        → 200 { items, counts }
//
// Node parity notes (src/admin/admin.controller.ts):
//   - inbox limit = Math.min(parseInt(limit || '50', 10) || 50, 100). An
//     empty / missing / unparseable / zero value falls to 50, then caps at 100.
//   - kind ∈ {error, bug} drives which sources are fetched; status=='open'
//     filters to the open subset (error status 0 / bug status 'new').
package inbox

import (
	"context"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// handlerService is the handler's consumer-side port; *Service satisfies it.
type handlerService interface {
	Counts(ctx context.Context) (Counts, error)
	FeedFor(ctx context.Context, q FeedQuery) (Feed, error)
}

// Handler serves the admin inbox routes (D2).
type Handler struct {
	svc handlerService
}

// NewHandler wires the inbox service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Counts handles GET /admin/inbox/counts → 200 { new_bugs, ... , total }.
func (h *Handler) Counts(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Counts(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, c)
}

// Feed handles GET /admin/inbox → 200 { items, counts }. The limit mirrors
// Node's Math.min(parseInt(limit||'50')||50, 100).
func (h *Handler) Feed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	feed, err := h.svc.FeedFor(r.Context(), FeedQuery{
		Kind:     q.Get("kind"),
		OpenOnly: q.Get("status") == "open",
		Limit:    inboxLimit(q.Get("limit")),
	})
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, feed)
}

// inboxLimit mirrors Math.min(parseInt(limit || '50', 10) || 50, 100): missing,
// empty, unparseable, or zero → 50; then cap at 100.
func inboxLimit(s string) int {
	v, ok := listquery.JSParseInt(s)
	if !ok || v == 0 {
		v = 50
	}
	if v > 100 {
		v = 100
	}
	return v
}
