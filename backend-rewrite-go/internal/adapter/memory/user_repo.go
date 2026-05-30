// Package memory holds in-memory implementations of every domain.Port.
// Two audiences:
//
//  1. Unit tests in usecase/, http/, and any future caller. Hand-rolled
//     "fake" objects are easier to drive than mocks generated from a
//     library — the behaviour is the test's documentation.
//  2. Local dev when you don't want to wire up Postgres + Redis just
//     to poke the binary. cmd/server/main.go can swap real adapters
//     for memory ones via env (future task).
//
// Rule #1 — reusable: every test in the repo pulls fakes from here, no
// per-test reinvention.
package memory

import (
	"context"
	"sync"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// UserRepo is the in-memory implementation of domain.UserRepo. Holds
// at most one user (tests typically need exactly one) so Find returns
// it regardless of the requested id — keeps test setup minimal.
//
// FindErr / LogErr let tests stub failure paths without separate mocks.
type UserRepo struct {
	mu         sync.Mutex
	user       *domain.User
	findErr    error
	logErr     error
	usageLogs  []domain.UsageLog
}

// NewUserRepo returns a fresh repo seeded with user. Pass nil to
// simulate "no user found" (Find will return ErrUserNotFound).
func NewUserRepo(user *domain.User) *UserRepo {
	return &UserRepo{user: user}
}

// WithFindErr returns a builder so tests can chain stubs in one line:
//
//	repo := memory.NewUserRepo(u).WithFindErr(domain.ErrUserNotFound)
func (r *UserRepo) WithFindErr(err error) *UserRepo {
	r.findErr = err
	return r
}

// WithLogErr forces LogUsage to fail; useful for "telemetry write
// failure is non-fatal" tests in the use case.
func (r *UserRepo) WithLogErr(err error) *UserRepo {
	r.logErr = err
	return r
}

// Find returns the seeded user. If FindErr is set, returns it first.
// If user is nil (and no error stub), returns ErrUserNotFound — the
// canonical "no row" response so use cases see the same shape they
// would from the real PG adapter.
func (r *UserRepo) Find(_ context.Context, _ domain.UserID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.user == nil {
		return nil, domain.ErrUserNotFound
	}
	// Return a defensive copy so the test mutating the User struct
	// after this call (rare but possible) doesn't poison subsequent
	// Find calls.
	u := *r.user
	return &u, nil
}

// LogUsage records the call so tests can assert on it. Returns
// LogErr when stubbed.
func (r *UserRepo) LogUsage(_ context.Context, log domain.UsageLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.logErr != nil {
		return r.logErr
	}
	r.usageLogs = append(r.usageLogs, log)
	return nil
}

// Logs returns a defensive copy of every UsageLog recorded so far.
func (r *UserRepo) Logs() []domain.UsageLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.UsageLog, len(r.usageLogs))
	copy(out, r.usageLogs)
	return out
}

// LogsLen is a convenience helper — many tests just want the count.
func (r *UserRepo) LogsLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.usageLogs)
}
