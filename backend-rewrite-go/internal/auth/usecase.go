// Package auth ports the NestJS auth module's password flows. Phase 1a:
// login, refresh, change-password, account, delete. register/social/
// email land in 1b/1c. Depends on user/subscription/usage/settings via
// interfaces wired in the composition root (no cross-module internals).
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

// AuthError is a 401-class failure carrying the exact Node message. The
// handler maps it to {error: Message, code: "invalid-token"} → 401.
type AuthError struct{ Message string }

func (e *AuthError) Error() string { return e.Message }

func unauthorized(msg string) *AuthError { return &AuthError{Message: msg} }

// UserService is the user-module subset auth needs.
type UserService interface {
	ByEmail(ctx context.Context, email string) (user.User, error)
	ByID(ctx context.Context, id string) (user.User, error)
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	DeleteAccount(ctx context.Context, id string) error
	Create(ctx context.Context, in user.NewUser) (user.User, error)
	Update(ctx context.Context, id string, p user.UserPatch) error
	FindBySocialId(ctx context.Context, provider, socialID string) (user.User, error)
	AuthState(ctx context.Context, email string) (user.AuthState, error)
}

// FreePlanReader is the plans-module subset (free-plan lookup on signup).
type FreePlanReader interface {
	FindFreePlan(ctx context.Context) (plans.Plan, error)
}

// FreeSubWriter is the subscription-module subset (free grant on signup).
type FreeSubWriter interface {
	CreateFree(ctx context.Context, userID, planID string) error
}

// EmailSender is the email-module subset. Both methods fire-and-forget.
type EmailSender interface {
	SendVerification(ctx context.Context, to, name, code string)
	SendPasswordReset(ctx context.Context, to, name, code string)
}

// SocialVerifier verifies a provider id_token → SocialProfile.
// (SocialProfile + InboundProfile are declared in Part B's social.go.)
type SocialVerifier interface {
	Verify(ctx context.Context, provider, idToken string, profile InboundProfile) (SocialProfile, error)
}

// TTLReader resolves token lifetimes (platform/settings.Reader).
type TTLReader interface {
	TokenTTLs(ctx context.Context) (access, refresh time.Duration, err error)
}

// SubReader is the subscription-module subset (read-only).
type SubReader interface {
	ActiveByUser(ctx context.Context, userID string) (*subscription.AccountSub, error)
}

// UsageCounter is the usage-module subset.
type UsageCounter interface {
	CountToday(ctx context.Context, userID string) (int, error)
}

// Service orchestrates auth flows. Holds two signers (access/refresh
// secrets) + a refresh verifier.
type Service struct {
	users      UserService
	subs       SubReader
	usage      UsageCounter
	ttl        TTLReader
	access     *platauth.Signer
	refresh    *platauth.Signer
	refreshVer *platauth.Verifier
	plans      FreePlanReader
	subWriter  FreeSubWriter
	email      EmailSender
	social     SocialVerifier
}

// NewService wires dependencies. accessSecret signs/verifies access
// tokens; refreshSecret signs/verifies refresh tokens. The Phase 1b
// collaborators (plansReader/subWriter/emailSvc/socialVer) are appended
// after the Phase 1a deps so existing call sites only grow.
func NewService(users UserService, subs SubReader, usage UsageCounter, ttl TTLReader,
	accessSecret, refreshSecret string,
	plansReader FreePlanReader, subWriter FreeSubWriter, emailSvc EmailSender, socialVer SocialVerifier) *Service {
	return &Service{
		users:      users,
		subs:       subs,
		usage:      usage,
		ttl:        ttl,
		access:     platauth.NewSigner(accessSecret),
		refresh:    platauth.NewSigner(refreshSecret),
		refreshVer: platauth.NewVerifier(refreshSecret),
		plans:      plansReader,
		subWriter:  subWriter,
		email:      emailSvc,
		social:     socialVer,
	}
}

// Tokens is the {access_token, refresh_token} pair.
type Tokens struct {
	AccessToken  string
	RefreshToken string
}

// UserPayload is the trimmed user echoed by login.
type UserPayload struct {
	ID    string
	Email string
	Name  string
}

// LoginResult is login's full return.
type LoginResult struct {
	Tokens
	User UserPayload
}

// generateTokens mirrors Node generateTokens: same claims, TTLs from
// app_settings.
func (s *Service) generateTokens(ctx context.Context, u user.User) (Tokens, error) {
	accTTL, refTTL, err := s.ttl.TokenTTLs(ctx)
	if err != nil {
		return Tokens{}, err
	}
	claims := platauth.Claims{Sub: u.ID, Email: u.Email, Role: u.Role}
	at, err := s.access.Sign(claims, accTTL)
	if err != nil {
		return Tokens{}, err
	}
	rt, err := s.refresh.Sign(claims, refTTL)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: at, RefreshToken: rt}, nil
}

// Login ports auth.service.login exactly (branch order matters).
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	u, err := s.users.ByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return LoginResult{}, unauthorized(msgInvalidCredentials)
	}
	if err != nil {
		return LoginResult{}, err
	}
	if u.PasswordHash == "" {
		return LoginResult{}, unauthorized(socialLoginMsg(u.AuthProvider))
	}
	ok, err := shared.VerifyPassword(password, u.PasswordHash)
	if err != nil || !ok {
		return LoginResult{}, unauthorized(msgInvalidCredentials)
	}
	if !u.IsActive {
		return LoginResult{}, unauthorized(msgAccountDisabled)
	}
	toks, err := s.generateTokens(ctx, u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Tokens: toks, User: UserPayload{ID: u.ID, Email: u.Email, Name: u.Name}}, nil
}

// Refresh ports auth.service.refresh: verify the refresh token with the
// refresh secret, reload the user, require is_active, re-mint. Any
// failure → "Invalid refresh token".
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	claims, err := s.refreshVer.Verify(refreshToken)
	if err != nil {
		return Tokens{}, unauthorized(msgInvalidRefresh)
	}
	u, err := s.users.ByID(ctx, claims.Sub)
	if err != nil || !u.IsActive {
		return Tokens{}, unauthorized(msgInvalidRefresh)
	}
	return s.generateTokens(ctx, u)
}

// ChangePassword ports auth.service.changePassword: verify current,
// hash new, persist.
func (s *Service) ChangePassword(ctx context.Context, userID, current, next string) error {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return unauthorized("") // Node bare UnauthorizedException → "Unauthorized" (handler maps "")
	}
	ok, err := shared.VerifyPassword(current, u.PasswordHash)
	if err != nil || !ok {
		return unauthorized(msgCurrentPwWrong)
	}
	hash, err := shared.HashPassword(next)
	if err != nil {
		return err
	}
	return s.users.UpdatePasswordHash(ctx, userID, hash)
}

// AccountView is the /auth/account response (nil → JSON null).
type AccountView struct {
	ID                      string          `json:"id"`
	Email                   string          `json:"email"`
	Name                    string          `json:"name"`
	EmailVerified           bool            `json:"email_verified"`
	HasLemonsqueezyCustomer bool            `json:"has_lemonsqueezy_customer"`
	Subscription            *AccountSubView `json:"subscription"`
}

// AccountSubView is the nested subscription object.
type AccountSubView struct {
	PlanName   string  `json:"plan_name"`
	Status     string  `json:"status"`
	StoreType  string  `json:"store_type"`
	StartedAt  string  `json:"started_at"`
	ExpiresAt  *string `json:"expires_at"`
	DailyLimit int     `json:"daily_limit"`
	UsageToday int     `json:"usage_today"`
}

// Account ports auth.controller.account: user + active sub + usage. nil
// (→ JSON null) when the user row is gone. Timestamp format (RFC3339) is
// a known parity risk — the shadow gate pins it against the real Node
// response.
func (s *Service) Account(ctx context.Context, userID string) (*AccountView, error) {
	u, err := s.users.ByID(ctx, userID)
	if errors.Is(err, user.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sub, err := s.subs.ActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	usageToday, err := s.usage.CountToday(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := &AccountView{
		ID: u.ID, Email: u.Email, Name: u.Name,
		EmailVerified:           u.EmailVerified,
		HasLemonsqueezyCustomer: u.LemonsqueezyCustomer != "",
	}
	if sub != nil {
		view.Subscription = &AccountSubView{
			PlanName:   sub.PlanName,
			Status:     sub.Status,
			StoreType:  sub.StoreType,
			StartedAt:  sub.StartedAt.Format(time.RFC3339),
			DailyLimit: sub.DailyLimit,
			UsageToday: usageToday,
		}
		if sub.ExpiresAt != nil {
			e := sub.ExpiresAt.Format(time.RFC3339)
			view.Subscription.ExpiresAt = &e
		}
	}
	return view, nil
}

// DeleteAccount ports the cascade delete.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	return s.users.DeleteAccount(ctx, userID)
}
