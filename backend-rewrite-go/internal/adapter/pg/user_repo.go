// Package pg implements domain.UserRepo against Postgres via the
// sqlc-generated bindings in pg/sqlc. This is the production adapter;
// the in-memory equivalent (internal/adapter/memory) covers tests.
//
// Boundary: this file imports sqlc + pgxpool, the domain layer doesn't
// know either exists. Replace with a different driver later (sqlx,
// gorm, raw) by writing a new UserRepo here — domain + use case stay
// untouched. That's the whole point of the dependency-inversion seam.
package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/adapter/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// UserRepo is the Postgres-backed implementation of domain.UserRepo.
// One per process; the underlying pgxpool is the shared connection
// pool (set up in internal/platform/db). Safe for concurrent use.
type UserRepo struct {
	q *sqlc.Queries
}

// NewUserRepo wraps a pgxpool.Pool. Caller owns the pool's lifecycle —
// this adapter never closes it (multiple adapters share the same pool).
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{q: sqlc.New(pool)}
}

// Find loads user + active plan + today's usage in two queries. NestJS
// does the same shape via two TypeORM calls; we keep it identical so
// shadow-mode parity comparisons stay byte-for-byte until the cutover.
func (r *UserRepo) Find(ctx context.Context, id domain.UserID) (*domain.User, error) {
	uid := toPgUUID(uuid.UUID(id))

	row, err := r.q.FindUserWithPlan(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("pg.FindUserWithPlan: %w", err)
	}

	used, err := r.q.CountTodayUsage(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("pg.CountTodayUsage: %w", err)
	}

	return rowToDomain(row, used), nil
}

// LogUsage inserts one usage_logs row. Matches NestJS's
// UsageService.log shape so analytics queries don't care which
// backend produced a row.
func (r *UserRepo) LogUsage(ctx context.Context, log domain.UsageLog) error {
	err := r.q.InsertUsageLog(ctx, sqlc.InsertUsageLogParams{
		UserID:         toPgUUID(uuid.UUID(log.UserID)),
		Tone:           string(log.Tone),
		InputLength:    log.InputLength,
		OutputLength:   log.OutputLength,
		AiProviderID:   toPgUUID(log.AIProviderID),
		ResponseTimeMs: log.ResponseTimeMs,
	})
	if err != nil {
		return fmt.Errorf("pg.InsertUsageLog: %w", err)
	}
	return nil
}

// rowToDomain maps a sqlc row + usage count to a domain.User. One
// place owns the nullable plan handling — the LEFT JOIN can leave
// plan_* NULL when the user has no active subscription, which becomes
// a zero-limit Plan (treated as "free tier, no quota" by domain).
func rowToDomain(row sqlc.FindUserWithPlanRow, used int64) *domain.User {
	u := &domain.User{
		ID:        domain.UserID(uuidFromPg(row.UserID)),
		Email:     row.UserEmail,
		Role:      string(row.UserRole),
		UsedToday: used,
	}
	if row.PlanID.Valid {
		var planName string
		if row.PlanName != nil {
			planName = *row.PlanName
		}
		var dailyLimit int32
		if row.PlanDailyLimit != nil {
			dailyLimit = *row.PlanDailyLimit
		}
		u.Plan = domain.Plan{
			ID:         uuidFromPg(row.PlanID),
			Name:       planName,
			DailyLimit: dailyLimit,
		}
	}
	return u
}

// --- pgtype <-> google/uuid bridging -------------------------------

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidFromPg(v pgtype.UUID) uuid.UUID {
	if !v.Valid {
		return uuid.Nil
	}
	return uuid.UUID(v.Bytes)
}
