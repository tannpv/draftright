package exttoken

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the exttoken repo needs. Declared here
// (consumer side) so the package depends on behavior, not the concrete
// *sqlc.Queries — keeps tests able to fake it without a DB.
type Querier interface {
	RevokeActiveTokensForDevice(ctx context.Context, arg sqlc.RevokeActiveTokensForDeviceParams) error
	InsertExtensionToken(ctx context.Context, arg sqlc.InsertExtensionTokenParams) (sqlc.InsertExtensionTokenRow, error)
	ListActiveTokens(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListActiveTokensRow, error)
	RevokeTokenByID(ctx context.Context, arg sqlc.RevokeTokenByIDParams) error
	FindActiveTokenByHash(ctx context.Context, tokenHash string) (sqlc.FindActiveTokenByHashRow, error)
	TouchTokenLastUsed(ctx context.Context, id pgtype.UUID) error
}

// ActiveToken is the verify-path projection: just what the T13 Service needs
// to authorize a presented token (id for the last-used touch, userID for the
// request principal, scopes for the rewrite-scope check).
type ActiveToken struct {
	ID     string
	UserID string
	Scopes []string
}

// Repo is the concrete persistence adapter over the sqlc Querier. It maps
// sqlc rows ⇄ exttoken domain types so callers never see pgtype.
type Repo struct{ q Querier }

// NewRepo wires the querier (accept interface, return struct).
func NewRepo(q Querier) *Repo { return &Repo{q: q} }

// RevokeActiveForDevice revokes any still-active token for (user, device) —
// mint's rotate step (avoids colliding with the partial unique index).
func (r *Repo) RevokeActiveForDevice(ctx context.Context, userID, deviceID string) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	did, err := parseUUID(deviceID)
	if err != nil {
		return err
	}
	return r.q.RevokeActiveTokensForDevice(ctx, sqlc.RevokeActiveTokensForDeviceParams{
		UserID:   uid,
		DeviceID: did,
	})
}

// Insert creates a new token row and returns its list-shaped projection.
func (r *Repo) Insert(ctx context.Context, userID, tokenHash, deviceID, deviceName string, scopes []string) (TokenRow, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return TokenRow{}, err
	}
	did, err := parseUUID(deviceID)
	if err != nil {
		return TokenRow{}, err
	}
	row, err := r.q.InsertExtensionToken(ctx, sqlc.InsertExtensionTokenParams{
		UserID:     uid,
		TokenHash:  tokenHash,
		Scopes:     scopes,
		DeviceID:   did,
		DeviceName: deviceName,
	})
	if err != nil {
		return TokenRow{}, err
	}
	return TokenRow{
		ID:         uuid.UUID(row.ID.Bytes).String(),
		Scopes:     row.Scopes,
		DeviceID:   uuid.UUID(row.DeviceID.Bytes).String(),
		DeviceName: row.DeviceName,
		LastUsedAt: tsPtr(row.LastUsedAt),
		CreatedAt:  row.CreatedAt.Time,
		RevokedAt:  tsPtr(row.RevokedAt),
	}, nil
}

// ListActive returns the user's active tokens, newest first. Always non-nil.
func (r *Repo) ListActive(ctx context.Context, userID string) ([]TokenRow, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListActiveTokens(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]TokenRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, TokenRow{
			ID:         uuid.UUID(row.ID.Bytes).String(),
			Scopes:     row.Scopes,
			DeviceID:   uuid.UUID(row.DeviceID.Bytes).String(),
			DeviceName: row.DeviceName,
			LastUsedAt: tsPtr(row.LastUsedAt),
			CreatedAt:  row.CreatedAt.Time,
			RevokedAt:  tsPtr(row.RevokedAt),
		})
	}
	return out, nil
}

// RevokeByID revokes a token, owner-scoped. Idempotent (matches Node: no
// revoked_at filter, controller returns 204 regardless).
func (r *Repo) RevokeByID(ctx context.Context, id, userID string) error {
	tid, err := parseUUID(id)
	if err != nil {
		return err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	return r.q.RevokeTokenByID(ctx, sqlc.RevokeTokenByIDParams{ID: tid, UserID: uid})
}

// FindActiveByHash resolves a presented token's hash to its owner + scopes,
// or (nil, nil) when no active row matches.
func (r *Repo) FindActiveByHash(ctx context.Context, hash string) (*ActiveToken, error) {
	row, err := r.q.FindActiveTokenByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ActiveToken{
		ID:     uuid.UUID(row.ID.Bytes).String(),
		UserID: uuid.UUID(row.UserID.Bytes).String(),
		Scopes: row.Scopes,
	}, nil
}

// TouchLastUsed bumps last_used_at (verify write-behind). Caller treats
// errors as non-fatal.
func (r *Repo) TouchLastUsed(ctx context.Context, id string) error {
	tid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.q.TouchTokenLastUsed(ctx, tid)
}

// --- pgtype helpers ------------------------------------------------

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

func tsPtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
