package auth

import (
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

// authResponse is the login/register/social envelope in Node declaration
// order { access_token, refresh_token, user }. An ordered struct, NOT
// map[string]any: maps marshal keys sorted, which would re-order the
// nested `user` object (Node id,email,name → sorted email,id,name) and
// break byte-parity (#56). User is `any` so it can carry either authUser
// or authUserVerified.
type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         any    `json:"user"`
}

// authUser is the nested `user` object for login + social, in Node order
// { id, email, name }.
type authUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// authUserVerified is register's `user` object, adding the literal
// email_verified:false (Node: { id, email, name, email_verified }).
type authUserVerified struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"email_verified"`
}

// writeDomainErr renders a known domain error as the canonical envelope:
// *AuthError → invalid-token (bare message → "Unauthorized", matching
// NestJS's default), *BadRequestError → invalid-input, *ConflictError →
// conflict. Returns false (unhandled) for anything else so callers fall
// through to a 500. Superset of the old writeAuthErr (Rule #1).
func writeDomainErr(w http.ResponseWriter, r *http.Request, err error) bool {
	var ae *AuthError
	if errors.As(err, &ae) {
		msg := ae.Message
		if msg == "" {
			msg = "Unauthorized"
		}
		shared.WriteError(w, r, "invalid-token", msg)
		return true
	}
	var be *BadRequestError
	if errors.As(err, &be) {
		shared.WriteError(w, r, "invalid-input", be.Message)
		return true
	}
	var ce *ConflictError
	if errors.As(err, &ce) {
		shared.WriteError(w, r, "conflict", ce.Message)
		return true
	}
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	return shared.DecodeJSON(w, r, dst, shared.DecodeStrict)
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
		if writeDomainErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, authResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User:         authUser{ID: res.User.ID, Email: res.User.Email, Name: res.User.Name},
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
		if writeDomainErr(w, r, err) {
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
		if writeDomainErr(w, r, err) {
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

// Register: POST /auth/register → 201. Validation failures → 400
// invalid-input; duplicate email → 409 conflict. Response mirrors Login's
// token keys (access_token/refresh_token) + nested user with the literal
// email_verified:false.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := validateRegister(body.Email, body.Password, body.Name); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}
	res, err := h.svc.Register(r.Context(), body.Email, body.Password, body.Name)
	if err != nil {
		if writeDomainErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "register failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, authResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User: authUserVerified{
			ID: res.User.ID, Email: res.User.Email, Name: res.User.Name,
			EmailVerified: res.User.EmailVerified,
		},
	})
}

// Social: POST /auth/social → 201. Mirrors Login's body shape. Unsupported
// provider / missing email → 400 invalid-input; takeover guard / disabled →
// 401 invalid-token.
func (h *Handler) Social(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider  string `json:"provider"`
		IDToken   string `json:"id_token"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.SocialLogin(r.Context(), body.Provider, body.IDToken, InboundProfile{
		Name: body.Name, Email: body.Email, AvatarURL: body.AvatarURL,
	})
	if err != nil {
		if writeDomainErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "social login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, authResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User:         authUser{ID: res.User.ID, Email: res.User.Email, Name: res.User.Name},
	})
}

// VerifyEmail: POST /auth/verify-email → 200. Bad/expired code → 400
// invalid-input. Success body {"success":true}.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), body.Email, body.Code); err != nil {
		if writeDomainErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "verify-email failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ResendVerification: POST /auth/resend-verification → 200. Always
// {"success":true} (silent no-op for unknown/verified users — no domain
// errors). Only the internal fallback guards an unexpected failure.
func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ResendVerification(r.Context(), body.Email); err != nil {
		shared.WriteError(w, r, "internal", "resend-verification failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ForgotPassword: POST /auth/forgot-password → 200. Always
// {"success":true} (silent no-op for unknown/social accounts — no domain
// errors). Only the internal fallback guards an unexpected failure.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ForgotPassword(r.Context(), body.Email); err != nil {
		shared.WriteError(w, r, "internal", "forgot-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ResetPassword: POST /auth/reset-password → 200. Short password or
// bad/expired code → 400 invalid-input. Success body {"success":true}.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ResetPassword(r.Context(), body.Email, body.Code, body.NewPassword); err != nil {
		if writeDomainErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "reset-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
