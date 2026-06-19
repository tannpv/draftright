package adminauth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the admin-auth repo needs (consumer-side port).
type Querier interface {
	FindAdminByEmailLower(ctx context.Context, email string) (sqlc.AdminUser, error)
	FindAdminByID(ctx context.Context, id pgtype.UUID) (sqlc.AdminUser, error)
	UpdateAdminPasswordHash(ctx context.Context, arg sqlc.UpdateAdminPasswordHashParams) error
}

// PgRepo adapts the admin_users sqlc queries to the Repo port.
type PgRepo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *PgRepo { return &PgRepo{q: q} }

// FindByEmailLower returns the admin matching LOWER(email), or (nil, nil) when
// none exists.
func (r *PgRepo) FindByEmailLower(ctx context.Context, email string) (*AdminUser, error) {
	row, err := r.q.FindAdminByEmailLower(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a := mapRow(row)
	return &a, nil
}

// FindByID returns the admin by id, or (nil, nil) when none exists.
func (r *PgRepo) FindByID(ctx context.Context, id string) (*AdminUser, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return nil, nil
	}
	row, err := r.q.FindAdminByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a := mapRow(row)
	return &a, nil
}

// UpdatePasswordHash sets a new bcrypt hash (and bumps updated_at via the query).
func (r *PgRepo) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.q.UpdateAdminPasswordHash(ctx, sqlc.UpdateAdminPasswordHashParams{
		ID:           uid,
		PasswordHash: hash,
	})
}

func mapRow(row sqlc.AdminUser) AdminUser {
	return AdminUser{
		ID:           uuidStr(row.ID),
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		IsActive:     row.IsActive,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

// parseUUID parses a canonical UUID string into pgtype.UUID using pgtype's own
// scanner — no external uuid library needed (mirrors user/repo_pg.go idiom).
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

// uuidStr renders a pgtype.UUID as the canonical lowercase hyphenated form
// (8-4-4-4-12). Delegates to pgtype.UUID.String() which does exactly this for
// valid values (mirrors user/repo_pg.go idiom).
func uuidStr(u pgtype.UUID) string { return u.String() }
