// Admin user HTTP edge (Phase 4c-2). Mounts inside the admin group
// (jwtMW → RequireAdmin) wired in the router task; the handler does no auth.
//
//	GET   /admin/users        bespoke query parse → { users, total }   → 200
//	GET   /admin/users/:id     composite                                → 200
//	PATCH /admin/users/:id     partial update → full entity             → 200
//
// The list query parse is BESPOKE (Node usersService.findAll, NOT the shared
// listquery): page=parseInt|1, limit=parseInt|20, status only if in
// {active,inactive,all}, sort_by passthrough, sort_order = "ASC" iff
// upper=="ASC" else "DESC". Mirrors admin.controller.ts listUsers exactly.
package user

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminUsersService is the handler's consumer-side port; *AdminService
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject
// a fake without a DB.
type adminUsersService interface {
	List(ctx context.Context, p ListUsersParams) ([]AdminUserRow, int, error)
	Get(ctx context.Context, id string) (GetUserResponse, error)
	Update(ctx context.Context, id string, p UserPatchAdmin) (UserDetail, error)
}

// AdminHandler serves the admin user routes.
type AdminHandler struct {
	svc adminUsersService
}

// NewAdminHandler wires the service.
func NewAdminHandler(svc *AdminService) *AdminHandler { return &AdminHandler{svc: svc} }

// adminUsersListResponse is the { users, total } body. Field order = key order
// (users, total), matching Node's listUsers return.
type adminUsersListResponse struct {
	Users []AdminUserRow `json:"users"`
	Total int            `json:"total"`
}

// updateUserBody decodes the PATCH body. Snake-case tags bind the admin UI's
// keys; pointers distinguish absent (nil = unchanged, TypeORM partial update)
// from present. Mirrors UpdateUserDto (is_active, role, name — all optional).
type updateUserBody struct {
	IsActive *bool   `json:"is_active"`
	Role     *string `json:"role"`
	Name     *string `json:"name"`
}

// List handles GET /admin/users. Bespoke parse → svc.List → { users, total }.
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := 1
	if v, err := strconv.Atoi(q.Get("page")); err == nil {
		page = v
	}
	limit := 20
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = v
	}

	// status only honoured when one of the three valid literals (Node ternary:
	// active|inactive|all else undefined → "" = no filter).
	status := ""
	switch q.Get("status") {
	case "active", "inactive", "all":
		status = q.Get("status")
	}

	// sort_order = "ASC" iff upper == "ASC", else "DESC".
	sortOrder := "DESC"
	if strings.ToUpper(q.Get("sort_order")) == "ASC" {
		sortOrder = "ASC"
	}

	rows, total, err := h.svc.List(r.Context(), ListUsersParams{
		Search:    q.Get("search"),
		Status:    status,
		SortBy:    q.Get("sort_by"),
		SortOrder: sortOrder,
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		shared.WriteError(w, r, "internal", "users failed")
		return
	}
	if rows == nil {
		rows = []AdminUserRow{}
	}
	shared.WriteJSON(w, http.StatusOK, adminUsersListResponse{Users: rows, Total: total})
}

// GetUser handles GET /admin/users/:id → 200 composite. A missing/malformed id
// is NOT a 404: Node's findById returns null and the controller still returns
// 200 with `user: null` (the usecase reproduces this). Only an unexpected repo
// error 500s.
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp, err := h.svc.Get(r.Context(), id)
	if err != nil {
		shared.WriteError(w, r, "internal", "users failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, resp)
}

// UpdateUser handles PATCH /admin/users/:id → 200 full entity. Missing keys =
// nil = column untouched. ANY repo error (incl. the re-read's ErrNotFound for a
// missing id) maps to 500 internal — mirroring Node's findOneOrFail →
// AllExceptionsFilter, NOT a 404.
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body updateUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	patch := UserPatchAdmin{
		IsActive: body.IsActive,
		Role:     body.Role,
		Name:     body.Name,
	}
	user, err := h.svc.Update(r.Context(), id, patch)
	if err != nil {
		shared.WriteError(w, r, "internal", "users failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, user)
}
