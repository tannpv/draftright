package adminauth

import (
	"context"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminService is the handler's consumer-side port (Service satisfies it).
type adminService interface {
	Login(ctx context.Context, email, password string) (LoginResult, error)
	ChangePassword(ctx context.Context, adminID, current, next string) error
	GetProfile(ctx context.Context, adminID string) (AdminUser, error)
}

// Handler serves /admin/auth/{login,change-password,me}. Login is public;
// change-password + me sit behind shared.RequireAuth + shared.RequireAdmin, so
// they read the admin id from the verified claims.
type Handler struct{ svc adminService }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePwBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type loginUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type loginResp struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	User         loginUser `json:"user"`
}

type successResp struct {
	Success bool `json:"success"`
}

type meResp struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Login → POST /admin/auth/login (public). 201 on success.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if !shared.DecodeJSON(w, r, &body, shared.DecodeStrict) {
		return
	}
	res, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeAdminErr(w, r, err, "login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, loginResp{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User:         loginUser{ID: res.User.ID, Email: res.User.Email, Name: res.User.Name, Role: res.User.Role},
	})
}

// ChangePassword → POST /admin/auth/change-password (admin). 201 {success:true}.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminIDFromCtx(r)
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body changePwBody
	if !shared.DecodeJSON(w, r, &body, shared.DecodeStrict) {
		return
	}
	if err := h.svc.ChangePassword(r.Context(), adminID, body.CurrentPassword, body.NewPassword); err != nil {
		writeAdminErr(w, r, err, "change-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, successResp{Success: true})
}

// Me → GET /admin/auth/me (admin). 200, password_hash stripped.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminIDFromCtx(r)
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	a, err := h.svc.GetProfile(r.Context(), adminID)
	if err != nil {
		writeAdminErr(w, r, err, "profile failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, meResp{
		ID:        a.ID,
		Email:     a.Email,
		Name:      a.Name,
		IsActive:  a.IsActive,
		Role:      a.Role,
		CreatedAt: shared.ISOMillis(a.CreatedAt),
		UpdatedAt: shared.ISOMillis(a.UpdatedAt),
	})
}

// adminIDFromCtx pulls the verified admin id (sub claim) stamped by RequireAuth.
func adminIDFromCtx(r *http.Request) (string, bool) {
	c, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		return "", false
	}
	return c.Sub, true
}

// writeAdminErr maps a service error to its envelope. *Error → invalid-token
// 401 (the four admin-auth messages); anything else → internal 500.
func writeAdminErr(w http.ResponseWriter, r *http.Request, err error, internalMsg string) {
	var ae *Error
	if errors.As(err, &ae) {
		shared.WriteError(w, r, "invalid-token", ae.Message)
		return
	}
	shared.WriteError(w, r, "internal", internalMsg)
}
