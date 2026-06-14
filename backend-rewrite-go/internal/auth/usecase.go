// Package auth ports the NestJS auth module's password flows. Phase 1a:
// login, refresh, change-password, account, delete. register/social/
// email land in 1b/1c. Depends on user/subscription/usage/settings via
// interfaces wired in the composition root (no cross-module internals).
package auth

import (
	"context"
	"errors"
	"strings"
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

// RegisterResult is register's return — tokens + the trimmed user with
// the literal email_verified:false (Node returns the literal, not the col).
type RegisterResult struct {
	Tokens
	User RegisterUser
}

// RegisterUser is the trimmed user echoed by register.
type RegisterUser struct {
	ID            string
	Email         string
	Name          string
	EmailVerified bool
}

// Register ports auth.service.register exactly (order matters): reject
// duplicate email (409), hash, create with a 6-digit verification code,
// grant the free plan + free sub, fire-and-forget the verification email
// (after the sub, before tokens), then mint tokens.
func (s *Service) Register(ctx context.Context, email, password, name string) (RegisterResult, error) {
	norm := normalizeEmail(email)
	if _, err := s.users.ByEmail(ctx, norm); err == nil {
		return RegisterResult{}, conflict(msgEmailRegistered)
	} else if !errors.Is(err, user.ErrNotFound) {
		return RegisterResult{}, err
	}
	hash, err := shared.HashPassword(password)
	if err != nil {
		return RegisterResult{}, err
	}
	code := generateCode()
	expires := nowUTC().Add(emailCodeTTL)
	u, err := s.users.Create(ctx, user.NewUser{
		Email: norm, PasswordHash: hash, Name: name,
		EmailVerificationCode: code, EmailVerificationExpires: &expires,
	})
	if err != nil {
		return RegisterResult{}, err
	}
	free, err := s.plans.FindFreePlan(ctx)
	if err != nil {
		return RegisterResult{}, err
	}
	if err := s.subWriter.CreateFree(ctx, u.ID, free.ID); err != nil {
		return RegisterResult{}, err
	}
	s.email.SendVerification(ctx, norm, name, code) // fire-and-forget
	toks, err := s.generateTokens(ctx, u)
	if err != nil {
		return RegisterResult{}, err
	}
	return RegisterResult{Tokens: toks, User: RegisterUser{
		ID: u.ID, Email: u.Email, Name: u.Name, EmailVerified: false,
	}}, nil
}

// VerifyEmail ports auth.service.verifyEmail: look up the auth-state by
// normalized email, require a present, matching, unexpired 6-digit code,
// then mark verified + NULL the code/expiry. Every failure path returns
// the same byte-identical "Invalid or expired verification code".
func (s *Service) VerifyEmail(ctx context.Context, email, code string) error {
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		return badRequest(msgInvalidVerifyCode)
	}
	if err != nil {
		return err
	}
	if st.EmailVerificationCode == "" || st.EmailVerificationExpires == nil {
		return badRequest(msgInvalidVerifyCode)
	}
	if st.EmailVerificationCode != code {
		return badRequest(msgInvalidVerifyCode)
	}
	if st.EmailVerificationExpires.Before(nowUTC()) {
		return badRequest(msgInvalidVerifyCode)
	}
	on := true
	return s.users.Update(ctx, st.ID, user.UserPatch{
		EmailVerified:            &on,
		EmailVerificationCode:    user.NullableString{Set: true, Value: nil},
		EmailVerificationExpires: user.NullableTime{Set: true, Value: nil},
	})
}

// ResendVerification ports auth.service.resendVerification: silent no-op
// for unknown or already-verified users; otherwise set a fresh code +
// expiry and fire-and-forget the verification email.
func (s *Service) ResendVerification(ctx context.Context, email string) error {
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		return nil // silent
	}
	if err != nil {
		return err
	}
	if st.EmailVerified {
		return nil // silent
	}
	code := generateCode()
	expires := nowUTC().Add(emailCodeTTL)
	if err := s.users.Update(ctx, st.ID, user.UserPatch{
		EmailVerificationCode:    user.NullableString{Set: true, Value: &code},
		EmailVerificationExpires: user.NullableTime{Set: true, Value: &expires},
	}); err != nil {
		return err
	}
	s.email.SendVerification(ctx, st.Email, st.Name, code)
	return nil
}

// ForgotPassword ports auth.service.forgotPassword: silent no-op for
// unknown OR social-only (non-local / no password) accounts; otherwise
// set a fresh 6-digit reset code + expiry + attempts=0 and fire-and-forget
// the reset email.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		return nil // silent
	}
	if err != nil {
		return err
	}
	if st.AuthProvider != "local" || st.PasswordHash == "" {
		return nil // silent for social-only accounts
	}
	code := generateCode()
	expires := nowUTC().Add(emailCodeTTL)
	zero := 0
	if err := s.users.Update(ctx, st.ID, user.UserPatch{
		PasswordResetCode:     user.NullableString{Set: true, Value: &code},
		PasswordResetExpires:  user.NullableTime{Set: true, Value: &expires},
		PasswordResetAttempts: &zero,
	}); err != nil {
		return err
	}
	s.email.SendPasswordReset(ctx, st.Email, st.Name, code) // fire-and-forget
	return nil
}

// ResetPassword ports auth.service.resetPassword: reject short passwords
// (400); validate the reset code (present + matches + not expired); on a
// bad code, increment attempts and BURN the code (null it + reset attempts)
// once attempts reach maxResetAttempts; on success, patch only PasswordHash
// (the repo's ResetPasswordHash query clears code/expiry/attempts atomically).
func (s *Service) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	if len(newPassword) < 8 {
		return badRequest(msgPasswordTooShort)
	}
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		return badRequest(msgInvalidResetCode)
	}
	if err != nil {
		return err
	}
	if st.PasswordResetCode == "" || st.PasswordResetExpires == nil {
		return badRequest(msgInvalidResetCode)
	}
	badCode := st.PasswordResetCode != code || st.PasswordResetExpires.Before(nowUTC())
	if badCode {
		attempts := st.PasswordResetAttempts + 1
		if attempts >= maxResetAttempts {
			zero := 0
			_ = s.users.Update(ctx, st.ID, user.UserPatch{
				PasswordResetCode:     user.NullableString{Set: true, Value: nil},
				PasswordResetExpires:  user.NullableTime{Set: true, Value: nil},
				PasswordResetAttempts: &zero,
			})
		} else {
			_ = s.users.Update(ctx, st.ID, user.UserPatch{PasswordResetAttempts: &attempts})
		}
		return badRequest(msgInvalidResetCode)
	}
	hash, err := shared.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.users.Update(ctx, st.ID, user.UserPatch{PasswordHash: &hash})
}

// DeleteAccount ports the cascade delete.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	return s.users.DeleteAccount(ctx, userID)
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func nowUTC() time.Time { return time.Now().UTC() }
