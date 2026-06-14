package user

import (
	"context"
	"errors"
)

// ErrNotFound is returned when no user matches the lookup.
var ErrNotFound = errors.New("user: not found")

// Repo is the persistence port. Implemented by repo_pg in prod and by
// fakes in tests.
type Repo interface {
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id string) (User, error)
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	DeleteAccount(ctx context.Context, id string) error
	Create(ctx context.Context, in NewUser) (User, error)
	Update(ctx context.Context, id string, p UserPatch) error
	FindBySocialId(ctx context.Context, provider, socialID string) (User, error)
	AuthState(ctx context.Context, email string) (AuthState, error)
}

// Service is the thin domain entry point. It adds no logic over the
// repo in 1a but gives auth a stable interface and a home for future
// invariants (Phase 1b register/verify).
type Service struct{ repo Repo }

// NewService wires a Repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

func (s *Service) ByEmail(ctx context.Context, email string) (User, error) {
	return s.repo.ByEmail(ctx, email)
}
func (s *Service) ByID(ctx context.Context, id string) (User, error) {
	return s.repo.ByID(ctx, id)
}
func (s *Service) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	return s.repo.UpdatePasswordHash(ctx, id, hash)
}
func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	return s.repo.DeleteAccount(ctx, id)
}
