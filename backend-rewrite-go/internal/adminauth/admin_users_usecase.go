package adminauth

import (
	"context"
	"errors"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// ErrDuplicateEmail is returned by Create when an admin row already has the
// requested email. The handler maps it to a 400 invalid-input with the Node
// message "Email already exists" (admin.controller.ts createAdminUser).
var ErrDuplicateEmail = errors.New("email already exists")

// adminUsersRepo is the admin-users usecase's consumer-side persistence port,
// satisfied by Task 14's concrete *AdminUsersRepo. Named distinctly from that
// concrete struct per the clean-arch guardrail: the consumer owns the
// interface, the adapter owns the implementation. Methods are 1:1 the Task-14
// methods this service uses.
type adminUsersRepo interface {
	ListAll(ctx context.Context) ([]AdminUserOut, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]AdminUserOut, int, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	Insert(ctx context.Context, in NewAdminUser) (AdminUserOut, error)
	Update(ctx context.Context, id string, p AdminUserPatch) (AdminUserOut, error)
	SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error
	IsActiveAdmin(ctx context.Context, id string) (bool, error)
	CountActiveAdmins(ctx context.Context) (int, error)
}

// CreateAdminUserInput is the resolved create payload — Role is already
// defaulted to "" by the handler when absent; this usecase applies the "admin"
// fallback before hashing/inserting (Node `role: body.role || 'admin'`).
type CreateAdminUserInput struct {
	Email    string
	Password string
	Name     string
	Role     string
}

// UpdateAdminUserInput is the partial update payload — a nil pointer = field
// absent (Node `!== undefined`). Password is a string: empty = not provided
// (Node `if (body.password)` truthy-guard); non-empty → re-hash.
type UpdateAdminUserInput struct {
	Name     *string
	Email    *string
	Role     *string
	IsActive *bool
	Password string
}

// AdminUsersService is the admin-users CRUD usecase — parity with the Node
// admin.controller.ts admin-users routes. The dual-mode list decision (bare
// array vs { rows, total }) lives in the handler, matching Node's
// controller-level branch.
type AdminUsersService struct {
	repo adminUsersRepo
}

// NewAdminUsersService wires the repo.
func NewAdminUsersService(repo adminUsersRepo) *AdminUsersService {
	return &AdminUsersService{repo: repo}
}

// ListAll returns every admin user ordered created_at ASC (Node find ASC).
func (s *AdminUsersService) ListAll(ctx context.Context) ([]AdminUserOut, error) {
	return s.repo.ListAll(ctx)
}

// ListPaginated returns the paginated/filtered set (Node applyListQuery).
func (s *AdminUsersService) ListPaginated(ctx context.Context, b listquery.Built) ([]AdminUserOut, int, error) {
	return s.repo.ListPaginated(ctx, b)
}

// Create checks for a duplicate email, hashes the password, defaults the role
// to "admin" when empty, then inserts (Node createAdminUser). A duplicate email
// short-circuits with ErrDuplicateEmail before any hashing.
func (s *AdminUsersService) Create(ctx context.Context, in CreateAdminUserInput) (AdminUserOut, error) {
	exists, err := s.repo.EmailExists(ctx, in.Email)
	if err != nil {
		return AdminUserOut{}, err
	}
	if exists {
		return AdminUserOut{}, ErrDuplicateEmail
	}

	hash, err := shared.HashPassword(in.Password)
	if err != nil {
		return AdminUserOut{}, err
	}

	role := in.Role
	if role == "" {
		role = "admin"
	}

	return s.repo.Insert(ctx, NewAdminUser{
		Email:        in.Email,
		PasswordHash: hash,
		Name:         in.Name,
		Role:         role,
	})
}

// Update builds the partial patch from the provided fields and re-reads the row
// (Node update → findOneOrFail). A truthy password is re-hashed into
// PasswordHash; an empty password leaves it untouched. A missing id propagates
// ErrAdminNotFound from the repo, which the handler maps to 500.
func (s *AdminUsersService) Update(ctx context.Context, id string, in UpdateAdminUserInput) (AdminUserOut, error) {
	patch := AdminUserPatch{
		Name:     in.Name,
		Email:    in.Email,
		Role:     in.Role,
		IsActive: in.IsActive,
	}
	if in.Password != "" {
		hash, err := shared.HashPassword(in.Password)
		if err != nil {
			return AdminUserOut{}, err
		}
		patch.PasswordHash = &hash
	}
	return s.repo.Update(ctx, id, patch)
}

// SoftDeleteWithAudit clears is_active AND records an append-only audit row in
// the same transaction (#51). actorID is the deactivating admin (JWT sub);
// targetID is the deactivated admin. The #32 guards run upstream in the handler,
// so a rejected deactivation never reaches here and never writes a row.
func (s *AdminUsersService) SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error {
	return s.repo.SoftDeleteWithAudit(ctx, actorID, targetID)
}

// IsActiveAdmin reports whether id is an existing, active admin (#32 guard).
func (s *AdminUsersService) IsActiveAdmin(ctx context.Context, id string) (bool, error) {
	return s.repo.IsActiveAdmin(ctx, id)
}

// CountActiveAdmins returns the active-admin count (#32 last-admin guard).
func (s *AdminUsersService) CountActiveAdmins(ctx context.Context) (int, error) {
	return s.repo.CountActiveAdmins(ctx)
}
