package adminauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// adminUserSearchCols / adminUserSortAllow are this consumer's listquery config
// for the paginated branch of GET /admin/admin-users (Node applyListQuery).
// searchCols are ILIKE-matched against `search`; sortAllow is the
// injection-safe allow-list mapping public sort keys → SQL columns (all
// identity-mapped here, no aliases). Default sort created_at, status column
// is_active — matching admin.controller.ts listAdminUsers exactly.
var adminUserSearchCols = []string{"name", "email", "role"}
var adminUserSortAllow = map[string]string{
	"name":       "name",
	"email":      "email",
	"role":       "role",
	"is_active":  "is_active",
	"created_at": "created_at",
}

// adminUsersService is the handler's consumer-side port; *AdminUsersService
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject
// a fake.
type adminUsersService interface {
	ListAll(ctx context.Context) ([]AdminUserOut, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]AdminUserOut, int, error)
	Create(ctx context.Context, in CreateAdminUserInput) (AdminUserOut, error)
	Update(ctx context.Context, id string, in UpdateAdminUserInput) (AdminUserOut, error)
	SoftDelete(ctx context.Context, id string) error
}

// AdminUsersHandler serves the admin admin-users routes. All routes mount inside
// the admin group (jwtMW → RequireAdmin); the handler itself does no auth.
//
//	GET    /admin/admin-users       dual-mode: bare array (no params) | { rows, total }
//	POST   /admin/admin-users       create → 201
//	PATCH  /admin/admin-users/:id    partial update → 200
//	DELETE /admin/admin-users/:id    soft delete → 200 { success:true }
type AdminUsersHandler struct {
	svc adminUsersService
}

// NewAdminUsersHandler wires the service.
func NewAdminUsersHandler(svc *AdminUsersService) *AdminUsersHandler {
	return &AdminUsersHandler{svc: svc}
}

// adminUsersPaginatedResponse is the { rows, total } body of the paginated
// branch. Field order (rows, total) matches Node's applyListQuery return.
type adminUsersPaginatedResponse struct {
	Rows  []AdminUserOut `json:"rows"`
	Total int            `json:"total"`
}

// createAdminUserBody decodes the POST body. role is optional (pointer = absent
// → default "admin" applied by the usecase). email/password/name are the
// required inputs; an inline Node `@Body() {…}` (not a DTO) so unknown fields
// are ignored and only a malformed body 400s.
type createAdminUserBody struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Name     string  `json:"name"`
	Role     *string `json:"role"`
}

// patchAdminUserBody decodes the PATCH body — every field optional. Pointers
// distinguish an absent key (nil → column untouched) from an explicit value
// (TypeORM partial update parity, Node `!== undefined`). Password is a plain
// string: empty/absent = leave hash untouched (Node `if (body.password)`).
type patchAdminUserBody struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
	Password string  `json:"password"`
}

// List handles GET /admin/admin-users. Dual-mode (admin.controller.ts
// listAdminUsers): when page, search, status and sort_by are ALL absent → legacy
// unpaginated find as a bare array; otherwise the paginated { rows, total }
// branch.
func (h *AdminUsersHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !q.Has("page") && !q.Has("search") && !q.Has("status") && !q.Has("sort_by") {
		users, err := h.svc.ListAll(r.Context())
		if err != nil {
			shared.WriteError(w, r, "internal", "admin-users failed")
			return
		}
		if users == nil {
			users = []AdminUserOut{}
		}
		shared.WriteJSON(w, http.StatusOK, users)
		return
	}

	b := listquery.Build(listquery.Parse(q), adminUserSearchCols, adminUserSortAllow, "created_at", "is_active")
	rows, total, err := h.svc.ListPaginated(r.Context(), b)
	if err != nil {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	if rows == nil {
		rows = []AdminUserOut{}
	}
	shared.WriteJSON(w, http.StatusOK, adminUsersPaginatedResponse{Rows: rows, Total: total})
}

// Create handles POST /admin/admin-users → 201. A duplicate email → 400
// invalid-input "Email already exists" (Node BadRequestException). password_hash
// never appears in the response (AdminUserOut omits it).
func (h *AdminUsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createAdminUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	role := ""
	if body.Role != nil {
		role = *body.Role
	}

	out, err := h.svc.Create(r.Context(), CreateAdminUserInput{
		Email:    body.Email,
		Password: body.Password,
		Name:     body.Name,
		Role:     role,
	})
	if errors.Is(err, ErrDuplicateEmail) {
		shared.WriteError(w, r, "invalid-input", "Email already exists")
		return
	}
	if err != nil {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, out)
}

// Update handles PATCH /admin/admin-users/:id → 200 full user (password_hash
// stripped). Missing keys = nil = column untouched. A missing id surfaces as
// ErrAdminNotFound → 500 internal (Node findOneOrFail → AllExceptionsFilter),
// NOT a 404.
func (h *AdminUsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body patchAdminUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	out, err := h.svc.Update(r.Context(), id, UpdateAdminUserInput{
		Name:     body.Name,
		Email:    body.Email,
		Role:     body.Role,
		IsActive: body.IsActive,
		Password: body.Password,
	})
	if err != nil {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, out)
}

// Delete handles DELETE /admin/admin-users/:id → 200 { success:true }.
func (h *AdminUsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.SoftDelete(r.Context(), id); err != nil {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, struct {
		Success bool `json:"success"`
	}{true})
}
