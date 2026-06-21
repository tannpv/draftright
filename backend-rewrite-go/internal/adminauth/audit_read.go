package adminauth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

const (
	auditDefaultLimit = 50
	auditMaxLimit     = 100
)

// AdminAuditRepo reads admin_user_audit_log rows. Both queries are static → sqlc.
// The write side lives in AdminUsersRepo (it owns the soft-delete tx).
type AdminAuditRepo struct {
	q *sqlc.Queries
}

// NewAdminAuditRepo wires the sqlc querier. The pool is unused here (static
// queries only) but accepted for wiring symmetry with the other admin repos.
func NewAdminAuditRepo(q *sqlc.Queries, _ *pgxpool.Pool) *AdminAuditRepo {
	return &AdminAuditRepo{q: q}
}

// ListAudit returns rows newest-first, paginated. Non-nil empty slice → JSON [].
func (r *AdminAuditRepo) ListAudit(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, error) {
	rows, err := r.q.ListAdminUserAudit(ctx, sqlc.ListAdminUserAuditParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]AdminUserAuditOut, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminUserAuditOut{
			ID:            uuidStr(row.ID),
			ActorAdminID:  uuidStr(row.ActorAdminID),
			ActorEmail:    row.ActorEmail,
			TargetAdminID: uuidStr(row.TargetAdminID),
			TargetEmail:   row.TargetEmail,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return out, nil
}

// CountAudit returns the total audit-row count (for pagination).
func (r *AdminAuditRepo) CountAudit(ctx context.Context) (int, error) {
	n, err := r.q.CountAdminUserAudit(ctx)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// adminAuditRepo is the service's consumer-side port; *AdminAuditRepo satisfies it.
type adminAuditRepo interface {
	ListAudit(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, error)
	CountAudit(ctx context.Context) (int, error)
}

// AdminAuditService backs the read endpoint.
type AdminAuditService struct {
	repo adminAuditRepo
}

// NewAdminAuditService wires the repo.
func NewAdminAuditService(repo adminAuditRepo) *AdminAuditService {
	return &AdminAuditService{repo: repo}
}

// List returns one page of audit rows plus the total count.
func (s *AdminAuditService) List(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, int, error) {
	rows, err := s.repo.ListAudit(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountAudit(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// adminAuditLister is the handler's consumer-side port; *AdminAuditService satisfies it.
type adminAuditLister interface {
	List(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, int, error)
}

// AdminAuditHandler serves GET /admin/admin-user-audit (admin group). Go-only:
// Node has no equivalent route, so this endpoint is intentionally absent from
// the shadow-gate route inventory (deploy/shadow/routes.txt).
type AdminAuditHandler struct {
	svc adminAuditLister
}

// NewAdminAuditHandler wires the service.
func NewAdminAuditHandler(svc *AdminAuditService) *AdminAuditHandler {
	return &AdminAuditHandler{svc: svc}
}

// adminAuditPaginatedResponse is the { rows, total } body (field order matches
// the admin-users list endpoint).
type adminAuditPaginatedResponse struct {
	Rows  []AdminUserAuditOut `json:"rows"`
	Total int                 `json:"total"`
}

// List parses limit/offset (limit default 50, max 100; offset default 0) and
// returns the page newest-first.
func (h *AdminAuditHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := auditDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > auditMaxLimit {
		limit = auditMaxLimit
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}

	rows, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		shared.WriteError(w, r, "internal", "admin-user-audit failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, adminAuditPaginatedResponse{Rows: rows, Total: total})
}
