package adminauth

import (
	"context"
	"strings"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Repo is the persistence port (consumer-side). Satisfied by *PgRepo.
type Repo interface {
	FindByEmailLower(ctx context.Context, email string) (*AdminUser, error) // nil,nil if none
	FindByID(ctx context.Context, id string) (*AdminUser, error)            // nil,nil if none
	UpdatePasswordHash(ctx context.Context, id, hash string) error
}

const (
	accessTTLProd = 15 * time.Minute
	accessTTLDev  = 24 * time.Hour
	refreshTTL    = 7 * 24 * time.Hour
)

// Service orchestrates admin-auth flows. Two signers (access/refresh secrets)
// + isProd select the access TTL. Mirrors internal/auth.Service, minus the
// settings-backed TTLReader (admin TTLs are hardcoded by environment).
type Service struct {
	repo    Repo
	access  *platauth.Signer
	refresh *platauth.Signer
	isProd  bool
}

// NewService wires the repo + the two signing secrets. accessSecret signs admin
// access tokens (same JWT_SECRET as customers); refreshSecret signs refresh.
func NewService(repo Repo, accessSecret, refreshSecret string, isProd bool) *Service {
	return &Service{
		repo:    repo,
		access:  platauth.NewSigner(accessSecret),
		refresh: platauth.NewSigner(refreshSecret),
		isProd:  isProd,
	}
}

// LoginResult is the {access_token, refresh_token, user} the handler shapes.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         AdminUser
}

// Login authenticates an admin and mints the token pair.
// Check order mirrors Node admin-auth.service.ts login() exactly:
// not-found → ErrInvalidCredentials, bad password → ErrInvalidCredentials,
// !is_active → ErrAccountDisabled.
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	admin, err := s.repo.FindByEmailLower(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return LoginResult{}, err
	}
	if admin == nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	ok, err := shared.VerifyPassword(password, admin.PasswordHash)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	if !admin.IsActive {
		return LoginResult{}, ErrAccountDisabled
	}
	access, refresh, err := s.generateTokens(admin)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{AccessToken: access, RefreshToken: refresh, User: *admin}, nil
}

// generateTokens mirrors Node generateTokens: payload {sub,email,role,isAdmin:true};
// access signed with JWT_SECRET (15m prod / 24h dev), refresh with
// JWT_REFRESH_SECRET (7d).
func (s *Service) generateTokens(a *AdminUser) (access, refresh string, err error) {
	claims := platauth.Claims{Sub: a.ID, Email: a.Email, Role: a.Role, IsAdminFlag: true}
	accTTL := accessTTLDev
	if s.isProd {
		accTTL = accessTTLProd
	}
	access, err = s.access.Sign(claims, accTTL)
	if err != nil {
		return "", "", err
	}
	refresh, err = s.refresh.Sign(claims, refreshTTL)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}
