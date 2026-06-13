// Package auth ports the NestJS auth module's password flows. Phase 1a:
// login, refresh, change-password, account, delete. register/social/
// email land in 1b/1c. Depends on user/subscription/usage/settings via
// interfaces wired in the composition root (no cross-module internals).
package auth

import (
	"context"
	"errors"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
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
}

// TTLReader resolves token lifetimes (platform/settings.Reader).
type TTLReader interface {
	TokenTTLs(ctx context.Context) (access, refresh time.Duration, err error)
}

// Service orchestrates auth flows. Holds two signers (access/refresh
// secrets) + a refresh verifier.
type Service struct {
	users      UserService
	ttl        TTLReader
	access     *platauth.Signer
	refresh    *platauth.Signer
	refreshVer *platauth.Verifier
}

// NewService wires dependencies. accessSecret signs/verifies access
// tokens; refreshSecret signs/verifies refresh tokens.
func NewService(users UserService, ttl TTLReader, accessSecret, refreshSecret string) *Service {
	return &Service{
		users:      users,
		ttl:        ttl,
		access:     platauth.NewSigner(accessSecret),
		refresh:    platauth.NewSigner(refreshSecret),
		refreshVer: platauth.NewVerifier(refreshSecret),
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
