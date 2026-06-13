package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// UserRow is the minimal user projection GET /auth/me echoes back —
// the same id/email/role fields Node returns from the JWT-resolved
// user. Decoupled from the sqlc row so the handler stays DB-free in
// tests (Rule #1).
type UserRow struct {
	ID    string
	Email string
	Role  string
}

// UserReader is the seam over the user lookup. The Postgres adapter
// lives in this file; tests pass a stub.
type UserReader interface {
	ByID(ctx context.Context, id string) (UserRow, error)
}

// MeHandler serves GET /auth/me with the exact Node body shape:
// { id, email, role, flags: { use_go_backend } }. Identity comes from
// the verified JWT (claims.Sub); the user is reloaded so the response
// reflects current email/role, and use_go_backend is computed from the
// configured ramp.
type MeHandler struct {
	users       UserReader
	rampPercent int
}

// NewMeHandler wires the user source + the GO_BACKEND_RAMP_PERCENT
// value the flag is bucketed against.
func NewMeHandler(users UserReader, rampPercent int) *MeHandler {
	return &MeHandler{users: users, rampPercent: rampPercent}
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		// Route reached without RequireAuth wrapping it — a router
		// misconfiguration, not a client error.
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}

	row, err := h.users.ByID(r.Context(), claims.Sub)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			shared.WriteError(w, r, "user-not-found", "user not found")
			return
		}
		shared.WriteError(w, r, "internal", "failed to load user")
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"id":    row.ID,
		"email": row.Email,
		"role":  row.Role,
		"flags": map[string]any{
			"use_go_backend": UseGoBackend(row.ID, h.rampPercent),
		},
	})
}

// errUserNotFound is the package sentinel the handler maps to a 401
// user-not-found envelope. pgUserReader returns it when the lookup
// no-rows; the test stub returns it directly.
var errUserNotFound = errors.New("core: user not found")

// pgUserReader adapts the shared sqlc Queries to the UserReader seam.
type pgUserReader struct{ q *sqlc.Queries }

// NewPgUserReader builds the Postgres-backed UserReader from a
// sqlc.Queries (built on the shared pool).
func NewPgUserReader(q *sqlc.Queries) UserReader { return pgUserReader{q: q} }

func (p pgUserReader) ByID(ctx context.Context, id string) (UserRow, error) {
	row, err := p.q.GetUserByID(ctx, parseUUIDParam(id))
	if err != nil {
		return UserRow{}, mapUserErr(err)
	}
	// Echo the input id string (already a uuid from the JWT sub) rather
	// than re-stringifying row.ID's pgtype.UUID. Role is a string-backed
	// UsersRoleEnum — convert to a plain string for the JSON body.
	return UserRow{ID: id, Email: row.Email, Role: string(row.Role)}, nil
}

// parseUUIDParam converts a uuid string into a pgtype.UUID for the
// query parameter. A malformed id leaves the value unset; the lookup
// then no-rows and surfaces as user-not-found, so a bad sub never
// 500s.
func parseUUIDParam(id string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{} // unset -> query no-rows -> user-not-found
	}
	return u
}

// mapUserErr translates pgx's no-rows sentinel into the package's
// errUserNotFound; any other DB error passes through to a 500.
func mapUserErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errUserNotFound
	}
	return err
}
