package domain

import "github.com/google/uuid"

// UserID is a typed wrapper around uuid.UUID — prevents accidentally
// passing a plain UUID (a plan id, an ai provider id, …) where a user
// id was expected. The compiler catches the mix-up.
type UserID uuid.UUID

// String renders as the canonical hyphenated UUID. Used in logs +
// HTTP responses + JWT sub-claim parsing.
func (u UserID) String() string { return uuid.UUID(u).String() }

// ParseUserID is the inverse — used when reading the JWT `sub` claim
// (string) and converting to the typed value.
func ParseUserID(s string) (UserID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UserID{}, ErrInvalidInput
	}
	return UserID(id), nil
}

// Plan is the subscription plan a user is currently on. Embedded in
// User so quota checks are a single in-memory comparison after the
// repository fetch.
type Plan struct {
	ID         uuid.UUID
	Name       string
	DailyLimit int32
}

// DailyLimitOrUnlimited returns true when the plan has no quota cap.
// DailyLimit == 0 is the convention NestJS uses for "free unlimited"
// (rare, mostly internal staff accounts).
func (p Plan) DailyLimitOrUnlimited() bool {
	return p.DailyLimit <= 0
}

// User aggregates the data the rewrite flow needs about the caller.
// Fetched in one Postgres round-trip via the FindUserWithPlan query
// (see internal/adapter/pg/queries.sql).
type User struct {
	ID         UserID
	Email      string
	Role       string
	Plan       Plan // zero value when no active subscription
	UsedToday  int64 // count of usage_logs rows for today
}

// CheckQuota returns ErrQuotaExceeded if the user has hit their daily
// limit. Pure function — no I/O, instantly unit-testable.
//
// Free tier (no active subscription) is treated as a hard zero — the
// caller must not have a User without a Plan reach this function in
// production. (See use case `usecase/rewrite.go` for the guard.)
func (u *User) CheckQuota() error {
	if u.Plan.DailyLimitOrUnlimited() {
		return nil
	}
	if u.UsedToday >= int64(u.Plan.DailyLimit) {
		return ErrQuotaExceeded
	}
	return nil
}
