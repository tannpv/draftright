// Plans admin Postgres adapter (Phase 4c-2). Mirrors aiprovider/repo_pg.go:
//
//   - STATIC queries (ListAll, Create, SoftDelete) go through the
//     sqlc-generated *sqlc.Queries — compile-time checked.
//   - DYNAMIC queries (ListPaginated's SELECT/COUNT, Update's partial SET)
//     are assembled from listquery.Built / a non-nil patch field set and run
//     on the pgxpool directly, because their WHERE/ORDER/columns aren't known
//     until runtime. listquery.Build's allow-listed columns are the
//     SQL-injection guard; plansPatchSQL's column names are literals owned by
//     this file, never user input.
//
// billing_period parity: the column is the plans_billing_period_enum. sqlc
// types it as PlansBillingPeriodEnum, so Create binds the enum directly. The
// dynamic SELECT/RETURNING force `billing_period::text` so the pool scan
// lands a Go string (mirroring how aiprovider forces temperature::text).
package plans

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AdminRepo serves the static plan queries via sqlc and the dynamic ones via
// the pool. One per process; the pgxpool is shared and owned by the caller.
type AdminRepo struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewAdminRepo wires the sqlc querier + the shared pool.
func NewAdminRepo(q *sqlc.Queries, pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{q: q, pool: pool}
}

// selectCols is the projection shared by the dynamic read paths so the column
// list — and the billing_period::text cast — lives in one place.
const selectCols = "id, name, daily_limit, price_cents, currency, stripe_price_id, " +
	"trial_days, billing_period::text, is_active, created_at, updated_at"

// plansPaginatedSQL emits the rows SELECT for ListPaginated. The LIMIT/OFFSET
// placeholders are left as %d for the caller to fill with the positional arg
// indexes, since those depend on how many WHERE args listquery.Built produced.
func plansPaginatedSQL(where, order string) string {
	return fmt.Sprintf("SELECT %s FROM plans %s %s LIMIT $%%d OFFSET $%%d",
		selectCols, where, order)
}

// plansCountSQL emits the total-count query for ListPaginated, sharing the
// same runtime WHERE as the rows query.
func plansCountSQL(where string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM plans %s", where)
}

// ListAll returns every plan ordered by created_at ASC (Node findAll).
func (r *AdminRepo) ListAll(ctx context.Context) ([]PlanEntity, error) {
	rows, err := r.q.ListAllPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanEntity, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlanEntity{
			ID:            uuid.UUID(row.ID.Bytes).String(),
			Name:          row.Name,
			DailyLimit:    int(row.DailyLimit),
			PriceCents:    int(row.PriceCents),
			Currency:      row.Currency,
			StripePriceID: row.StripePriceID,
			TrialDays:     int(row.TrialDays),
			BillingPeriod: string(row.BillingPeriod),
			IsActive:      row.IsActive,
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		})
	}
	return out, nil
}

// ListPaginated runs the dynamic WHERE/ORDER/LIMIT/OFFSET from
// listquery.Built, then a COUNT(*) over the same WHERE. Returns a non-nil
// empty slice when no rows match.
func (r *AdminRepo) ListPaginated(ctx context.Context, b listquery.Built) ([]PlanEntity, int, error) {
	limPos := len(b.Args) + 1
	offPos := len(b.Args) + 2
	rowsSQL := fmt.Sprintf(plansPaginatedSQL(b.Where, b.Order), limPos, offPos)

	args := make([]any, 0, len(b.Args)+2)
	args = append(args, b.Args...)
	args = append(args, b.Limit, b.Offset)

	rows, err := r.pool.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []PlanEntity{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.pool.QueryRow(ctx, plansCountSQL(b.Where), b.Args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create writes the 7 admin-create columns and returns the created row
// (is_active/created_at/updated_at come from DB defaults via RETURNING).
func (r *AdminRepo) Create(ctx context.Context, in NewPlan) (PlanEntity, error) {
	row, err := r.q.InsertPlan(ctx, sqlc.InsertPlanParams{
		Name:          in.Name,
		DailyLimit:    int32(in.DailyLimit),
		PriceCents:    int32(in.PriceCents),
		BillingPeriod: sqlc.PlansBillingPeriodEnum(in.BillingPeriod),
		Currency:      in.Currency,
		TrialDays:     int32(in.TrialDays),
		StripePriceID: in.StripePriceID,
	})
	if err != nil {
		return PlanEntity{}, err
	}
	return PlanEntity{
		ID:            uuid.UUID(row.ID.Bytes).String(),
		Name:          row.Name,
		DailyLimit:    int(row.DailyLimit),
		PriceCents:    int(row.PriceCents),
		Currency:      row.Currency,
		StripePriceID: row.StripePriceID,
		TrialDays:     int(row.TrialDays),
		BillingPeriod: string(row.BillingPeriod),
		IsActive:      row.IsActive,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

// Update applies the non-nil patch fields via a dynamic UPDATE ... SET ...
// RETURNING, mirroring TypeORM's partial .update() (only provided keys are
// written) followed by findOneOrFail's re-read.
func (r *AdminRepo) Update(ctx context.Context, id string, p PlanPatch) (PlanEntity, error) {
	set, args := plansPatchSQL(p)
	args = append(args, id)
	sql := fmt.Sprintf("UPDATE plans SET %s WHERE id = $%d RETURNING %s",
		set, len(args), selectCols)

	return scanPlan(r.pool.QueryRow(ctx, sql, args...))
}

// SoftDelete clears is_active (Node softDelete).
func (r *AdminRepo) SoftDelete(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.q.SoftDeletePlan(ctx, uid)
}

// plansPatchSQL builds the SET clause from the non-nil patch fields, walking
// fields in domain order (deterministic — no map range), with hardcoded
// column names (injection-safe) and $N value placeholders. `updated_at =
// now()` is always appended last (TypeORM @UpdateDateColumn).
func plansPatchSQL(p PlanPatch) (set string, args []any) {
	var sets []string
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.Name != nil {
		add("name", *p.Name)
	}
	if p.DailyLimit != nil {
		add("daily_limit", *p.DailyLimit)
	}
	if p.PriceCents != nil {
		add("price_cents", *p.PriceCents)
	}
	if p.BillingPeriod != nil {
		add("billing_period", *p.BillingPeriod)
	}
	if p.Currency != nil {
		add("currency", *p.Currency)
	}
	if p.TrialDays != nil {
		add("trial_days", *p.TrialDays)
	}
	if p.StripePriceID != nil {
		add("stripe_price_id", *p.StripePriceID)
	}
	if p.IsActive != nil {
		add("is_active", *p.IsActive)
	}
	sets = append(sets, "updated_at = now()")
	return strings.Join(sets, ", "), args
}

// scanPlan maps the 11 selectCols (in order) from a pgx.Row into a
// PlanEntity. billing_period is already text (the ::text cast), so it scans
// straight into a Go string.
func scanPlan(row pgx.Row) (PlanEntity, error) {
	var (
		id                   pgtype.UUID
		createdAt, updatedAt pgtype.Timestamp
		p                    PlanEntity
	)
	if err := row.Scan(
		&id, &p.Name, &p.DailyLimit, &p.PriceCents, &p.Currency, &p.StripePriceID,
		&p.TrialDays, &p.BillingPeriod, &p.IsActive, &createdAt, &updatedAt,
	); err != nil {
		return PlanEntity{}, err
	}
	p.ID = uuid.UUID(id.Bytes).String()
	p.CreatedAt = createdAt.Time
	p.UpdatedAt = updatedAt.Time
	return p, nil
}

// parseUUID builds a pgtype.UUID from a string id for the static SoftDelete.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}
