// Package rewritelog — Postgres adapter for the rewrite_logs table
// (admin training-data endpoints).
//
// All queries are static (fixed WHERE/ORDER), so every path goes through
// the sqlc-generated *sqlc.Queries via the rewriteQuerier interface. No
// dynamic SQL is assembled here.
//
// Node parity notes (RewriteLogService):
//   - Count()         → rewriteLogRepo.count()           (no filter)
//   - CountByQuality() → 3 × count({where:{quality}})    (per-value)
//   - FindPending()   → findAndCount({where,order,skip,take})
//   - UpdateQuality() → update(id, {quality})            — malformed uuid →
//     error (Node 500s via Postgres invalid-uuid); a valid-but-absent uuid →
//     0 rows updated, nil error (Node 200 success).
//   - FindApprovedAsc() → find({where:{quality:'approved'},order:{created_at:'ASC'}})
package rewritelog

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// rewriteQuerier is the subset of *sqlc.Queries the PgRepo uses.
// Declared on the consumer side (CLAUDE.md Rule: interfaces belong to the
// consumer), and only as wide as needed. *sqlc.Queries satisfies this; a
// test fake can too.
type rewriteQuerier interface {
	CountRewriteLogs(ctx context.Context) (int64, error)
	CountRewriteLogsByQuality(ctx context.Context) ([]sqlc.CountRewriteLogsByQualityRow, error)
	ListPendingRewriteLogs(ctx context.Context, arg sqlc.ListPendingRewriteLogsParams) ([]sqlc.RewriteLog, error)
	CountPendingRewriteLogs(ctx context.Context) (int64, error)
	UpdateRewriteLogQuality(ctx context.Context, arg sqlc.UpdateRewriteLogQualityParams) error
	ListApprovedRewriteLogsAsc(ctx context.Context) ([]sqlc.RewriteLog, error)
	InsertRewriteLog(ctx context.Context, arg sqlc.InsertRewriteLogParams) error
}

// PgRepo is the Postgres adapter for rewrite_logs.
type PgRepo struct {
	q rewriteQuerier
}

// NewPgRepo wires the sqlc querier. *sqlc.Queries satisfies rewriteQuerier.
func NewPgRepo(q rewriteQuerier) *PgRepo { return &PgRepo{q: q} }

// Count returns the total number of rows in rewrite_logs.
// Mirrors Node: rewriteLogRepo.count()
func (r *PgRepo) Count(ctx context.Context) (int, error) {
	n, err := r.q.CountRewriteLogs(ctx)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// CountByQuality returns (pending, approved, rejected) counts.
// Missing quality values in the DB result in 0 for that bucket.
// Unknown quality values (future schema extensions) are silently ignored.
// Mirrors Node: 3 × rewriteLogRepo.count({ where: { quality: X } })
func (r *PgRepo) CountByQuality(ctx context.Context) (pending, approved, rejected int, err error) {
	rows, err := r.q.CountRewriteLogsByQuality(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, row := range rows {
		switch row.Quality {
		case "pending":
			pending = int(row.N)
		case "approved":
			approved = int(row.N)
		case "rejected":
			rejected = int(row.N)
			// unknown quality values are intentionally ignored
		}
	}
	return pending, approved, rejected, nil
}

// FindPending returns the pending-quality logs (newest first) for the given
// page/limit, plus the total count of pending logs.
// Mirrors Node: rewriteLogRepo.findAndCount({ where:{quality:'pending'},
//
//	order:{created_at:'DESC'}, skip:(page-1)*limit, take:limit })
func (r *PgRepo) FindPending(ctx context.Context, page, limit int) ([]RewriteLog, int, error) {
	offset := (page - 1) * limit
	rows, err := r.q.ListPendingRewriteLogs(ctx, sqlc.ListPendingRewriteLogsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountPendingRewriteLogs(ctx)
	if err != nil {
		return nil, 0, err
	}
	return mapRows(rows), int(total), nil
}

// UpdateQuality sets the quality field for the given log id.
// A malformed id returns an error without touching the DB — Node 500s via
// Postgres "invalid input syntax for type uuid". A valid-but-absent id
// results in 0 rows updated and nil error (Node 200 success).
// Mirrors Node: rewriteLogRepo.update(id, { quality })
func (r *PgRepo) UpdateQuality(ctx context.Context, id, quality string) error {
	uid, err := parseUUID(id)
	if err != nil {
		// Bad UUID: Postgres would throw "invalid input syntax for type uuid",
		// causing Node to 500. Surface the error so our handler can 500 too.
		return fmt.Errorf("invalid uuid %q: %w", id, err)
	}
	return r.q.UpdateRewriteLogQuality(ctx, sqlc.UpdateRewriteLogQualityParams{
		ID:      uid,
		Quality: quality,
	})
}

// Insert writes one training-data row after a successful rewrite.
// id, quality, created_at come from DB defaults (uuid, 'pending', now()).
// Mirrors Node: rewriteLogRepo.save({ tone, input_text, output_text, model,
// provider_type, response_time_ms }).
func (r *PgRepo) Insert(ctx context.Context, in RewriteLogInput) error {
	return r.q.InsertRewriteLog(ctx, sqlc.InsertRewriteLogParams{
		Tone:           in.Tone,
		InputText:      in.InputText,
		OutputText:     in.OutputText,
		Model:          in.Model,
		ProviderType:   in.ProviderType,
		ResponseTimeMs: int32(in.ResponseTimeMs),
	})
}

// FindApprovedAsc returns all approved logs ordered oldest-first.
// Used by the export endpoints (exportApproved, exportAll).
// Mirrors Node: rewriteLogRepo.find({ where:{quality:'approved'}, order:{created_at:'ASC'} })
func (r *PgRepo) FindApprovedAsc(ctx context.Context) ([]RewriteLog, error) {
	rows, err := r.q.ListApprovedRewriteLogsAsc(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// mapRows converts a slice of sqlc.RewriteLog rows into domain RewriteLog values.
func mapRows(rows []sqlc.RewriteLog) []RewriteLog {
	out := make([]RewriteLog, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRow(row))
	}
	return out
}

// mapRow maps one sqlc.RewriteLog (DB types) into the domain RewriteLog.
// pgtype.UUID → string, int32 → int, pgtype.Timestamp → time.Time.
func mapRow(row sqlc.RewriteLog) RewriteLog {
	return RewriteLog{
		ID:             uuidStr(row.ID),
		Tone:           row.Tone,
		InputText:      row.InputText,
		OutputText:     row.OutputText,
		Model:          row.Model,
		ProviderType:   row.ProviderType,
		ResponseTimeMs: int(row.ResponseTimeMs),
		Quality:        row.Quality,
		CreatedAt:      tsTime(row.CreatedAt),
	}
}

// parseUUID builds a pgtype.UUID from a string id.
// An unparseable id returns an error (callers map to no-op or ErrNotFound).
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

// uuidStr renders a pgtype.UUID as canonical lowercase hyphenated form.
func uuidStr(u pgtype.UUID) string { return u.String() }

// tsTime unwraps a pgtype.Timestamp; a NULL yields the zero time.
// created_at is NOT NULL in the schema, so Valid is always true in practice.
func tsTime(t pgtype.Timestamp) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
