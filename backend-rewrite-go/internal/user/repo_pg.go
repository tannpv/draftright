package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pgQuerier is the sqlc subset the read/update methods need.
type pgQuerier interface {
	GetAuthUserByEmail(ctx context.Context, email string) (sqlc.GetAuthUserByEmailRow, error)
	GetAuthUserByID(ctx context.Context, id pgtype.UUID) (sqlc.GetAuthUserByIDRow, error)
	UpdateUserPasswordHash(ctx context.Context, arg sqlc.UpdateUserPasswordHashParams) error
}

// PgRepo implements Repo over Postgres. The delete-cascade txn needs the
// pool directly (multi-statement), so PgRepo holds both.
type PgRepo struct {
	q    pgQuerier
	pool DeleteExecer
}

// NewPgRepo wires the sqlc querier + a transaction-capable executor.
func NewPgRepo(q pgQuerier, pool DeleteExecer) *PgRepo { return &PgRepo{q: q, pool: pool} }

func (r *PgRepo) ByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetAuthUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return User{
		ID: uuidStr(row.ID), Email: row.Email, PasswordHash: strOrEmpty(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: strOrEmpty(row.LemonsqueezyCustomerID),
	}, nil
}

func (r *PgRepo) ByID(ctx context.Context, id string) (User, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return User{}, ErrNotFound
	}
	row, err := r.q.GetAuthUserByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return User{
		ID: uuidStr(row.ID), Email: row.Email, PasswordHash: strOrEmpty(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: strOrEmpty(row.LemonsqueezyCustomerID),
	}, nil
}

func (r *PgRepo) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	return r.q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
		ID: uid, PasswordHash: &hash,
	})
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

// uuidStr renders a pgtype.UUID as the canonical lowercase hyphenated
// form (8-4-4-4-12). pgtype.UUID.String() already does exactly this for
// valid values, so we delegate to it.
func uuidStr(u pgtype.UUID) string { return u.String() }
