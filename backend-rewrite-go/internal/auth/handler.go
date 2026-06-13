package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler adapts the auth Service to chi http.HandlerFuncs. Each method
// matches one route. JSON shapes + status codes mirror NestJS exactly
// (login + change-password = 201; refresh + delete = 200).
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// writeAuthErr renders an *AuthError as the canonical envelope. Bare
// (empty) message → "Unauthorized" to match NestJS's default.
func writeAuthErr(w http.ResponseWriter, r *http.Request, err error) bool {
	var ae *AuthError
	if errors.As(err, &ae) {
		msg := ae.Message
		if msg == "" {
			msg = "Unauthorized"
		}
		shared.WriteError(w, r, "invalid-token", msg)
		return true
	}
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return false
	}
	return true
}

// Login: POST /auth/login → 201.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		if writeAuthErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, map[string]any{
		"access_token":  res.AccessToken,
		"refresh_token": res.RefreshToken,
		"user": map[string]any{
			"id": res.User.ID, "email": res.User.Email, "name": res.User.Name,
		},
	})
}

// Refresh: POST /auth/refresh → 200.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	toks, err := h.svc.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		if writeAuthErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "refresh failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": toks.AccessToken, "refresh_token": toks.RefreshToken,
	})
}

// ChangePassword: POST /auth/change-password (JWT) → 201.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ChangePassword(r.Context(), claims.Sub, body.CurrentPassword, body.NewPassword); err != nil {
		if writeAuthErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "change-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, map[string]any{"success": true})
}

// Account: GET /auth/account (JWT) → 200 (object or null).
func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	view, err := h.svc.Account(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "account failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, view)
}

// DeleteAccount: DELETE /auth/account (JWT) → 200.
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), claims.Sub); err != nil {
		shared.WriteError(w, r, "internal", "delete failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
