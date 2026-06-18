// Package aiprovider — Postgres adapter for the ai_providers admin CRUD.
//
// Two SQL styles live here, mirroring how 4c-1 set up the listquery seam:
//
//   - STATIC queries (List, GetByID, DemoteDefaults, Insert, SoftDelete)
//     go through the sqlc-generated *sqlc.Queries — compile-time checked.
//   - DYNAMIC queries (ListPaginated's SELECT/COUNT, and Update's partial
//     SET) are assembled from listquery.Built / a non-nil patch field set
//     and run on the pgxpool directly, because their WHERE/ORDER/columns
//     aren't known until runtime. The allow-listed columns in
//     listquery.Build are the SQL-injection guard; Update's column names
//     are literals owned by this file, never user input.
//
// Temperature parity: the column is numeric(3,2) and Node returns it as a
// 2-decimal string ("0.30"). Every read casts `temperature::text AS
// temperature`, so both the sqlc rows AND the dynamic scans land a Go
// string with Postgres-preserved scale. On INSERT, the formatted string is
// parsed into pgtype.Numeric (the column coerces to scale 2 anyway, and we
// re-read via the RETURNING cast).
package aiprovider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pool is the subset of *pgxpool.Pool the dynamic queries need. Kept tiny
// so a test fake (if ever needed) is cheap; *pgxpool.Pool satisfies it.
type pool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgRepo serves the static queries via sqlc and the dynamic ones via the
// pool. One per process; the pgxpool is shared and its lifecycle owned by
// the caller (internal/platform/db).
type PgRepo struct {
	q    *sqlc.Queries
	pool pool
}

// NewPgRepo wires the sqlc querier + the shared pool.
func NewPgRepo(q *sqlc.Queries, p pool) *PgRepo { return &PgRepo{q: q, pool: p} }

// selectCols is the projection shared by every read path (static + dynamic)
// so the column list — and the temperature::text cast — lives in one place.
const selectCols = "id, name, type, endpoint_url, api_key, model, temperature::text AS temperature, " +
	"is_default, is_active, created_at, updated_at"

// paginatedSQL emits the rows SELECT for ListPaginated. The LIMIT/OFFSET
// placeholders are left as %d for the caller to fill with the positional
// arg indexes (len(args)+1, +2), since those depend on how many WHERE args
// listquery.Built produced.
func paginatedSQL(where, order string) string {
	return fmt.Sprintf("SELECT %s FROM ai_providers %s %s LIMIT $%%d OFFSET $%%d",
		selectCols, where, order)
}

// countSQL emits the total-count query for ListPaginated, sharing the same
// runtime WHERE as the rows query.
func countSQL(where string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM ai_providers %s", where)
}

// ListPaginated runs the dynamic WHERE/ORDER/LIMIT/OFFSET from
// listquery.Built, then a COUNT(*) over the same WHERE. Returns a non-nil
// empty slice when no rows match.
func (r *PgRepo) ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error) {
	limPos := len(b.Args) + 1
	offPos := len(b.Args) + 2
	rowsSQL := fmt.Sprintf(paginatedSQL(b.Where, b.Order), limPos, offPos)

	args := make([]any, 0, len(b.Args)+2)
	args = append(args, b.Args...)
	args = append(args, b.Limit, b.Offset)

	rows, err := r.pool.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AiProvider{}
	for rows.Next() {
		p, err := scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.pool.QueryRow(ctx, countSQL(b.Where), b.Args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// List returns all providers ordered by created_at ASC (Node findAll).
func (r *PgRepo) List(ctx context.Context) ([]AiProvider, error) {
	rows, err := r.q.ListAiProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AiProvider, 0, len(rows))
	for _, row := range rows {
		out = append(out, AiProvider{
			ID: uuidStr(row.ID), Name: row.Name, Type: string(row.Type),
			EndpointURL: row.EndpointUrl, APIKey: row.ApiKey, Model: row.Model,
			Temperature: row.Temperature, IsDefault: row.IsDefault, IsActive: row.IsActive,
			CreatedAt: tsTime(row.CreatedAt), UpdatedAt: tsTime(row.UpdatedAt),
		})
	}
	return out, nil
}

// GetByID loads one provider. A malformed id or missing row → ErrNotFound.
func (r *PgRepo) GetByID(ctx context.Context, id string) (AiProvider, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return AiProvider{}, ErrNotFound
	}
	row, err := r.q.GetAiProviderByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return AiProvider{}, ErrNotFound
	}
	if err != nil {
		return AiProvider{}, err
	}
	return AiProvider{
		ID: uuidStr(row.ID), Name: row.Name, Type: string(row.Type),
		EndpointURL: row.EndpointUrl, APIKey: row.ApiKey, Model: row.Model,
		Temperature: row.Temperature, IsDefault: row.IsDefault, IsActive: row.IsActive,
		CreatedAt: tsTime(row.CreatedAt), UpdatedAt: tsTime(row.UpdatedAt),
	}, nil
}

// DemoteDefaults clears is_default on every currently-default row. Used to
// enforce the single-default invariant before promoting another (Node
// demotes inside create/update when data.is_default is truthy).
func (r *PgRepo) DemoteDefaults(ctx context.Context) error {
	return r.q.DemoteDefaultAiProviders(ctx)
}

// Insert writes all 8 columns and returns the created row.
func (r *PgRepo) Insert(ctx context.Context, in NewProvider) (AiProvider, error) {
	temp, err := numeric(in.Temperature)
	if err != nil {
		return AiProvider{}, err
	}
	row, err := r.q.InsertAiProvider(ctx, sqlc.InsertAiProviderParams{
		Name: in.Name, Type: sqlc.AiProvidersTypeEnum(in.Type), EndpointUrl: in.EndpointURL,
		ApiKey: in.APIKey, Model: in.Model, Temperature: temp,
		IsDefault: in.IsDefault, IsActive: in.IsActive,
	})
	if err != nil {
		return AiProvider{}, err
	}
	return AiProvider{
		ID: uuidStr(row.ID), Name: row.Name, Type: string(row.Type),
		EndpointURL: row.EndpointUrl, APIKey: row.ApiKey, Model: row.Model,
		Temperature: row.Temperature, IsDefault: row.IsDefault, IsActive: row.IsActive,
		CreatedAt: tsTime(row.CreatedAt), UpdatedAt: tsTime(row.UpdatedAt),
	}, nil
}

// Update applies the non-nil patch fields via a dynamic UPDATE ... SET ...
// RETURNING, mirroring TypeORM's partial .update() (only provided keys are
// written) followed by findOneOrFail's re-read. An empty patch is a no-op
// re-read (GetByID), matching .update(id, {}) + findOneOrFail. A malformed
// id or missing row → ErrNotFound.
func (r *PgRepo) Update(ctx context.Context, id string, p ProviderPatch) (AiProvider, error) {
	if _, err := parseUUID(id); err != nil {
		return AiProvider{}, ErrNotFound
	}

	var sets []string
	var args []any
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.Name != nil {
		add("name", *p.Name)
	}
	if p.Type != nil {
		add("type", sqlc.AiProvidersTypeEnum(*p.Type))
	}
	if p.EndpointURL != nil {
		add("endpoint_url", *p.EndpointURL)
	}
	if p.APIKey != nil {
		add("api_key", *p.APIKey)
	}
	if p.Model != nil {
		add("model", *p.Model)
	}
	if p.Temperature != nil {
		temp, err := numeric(*p.Temperature)
		if err != nil {
			return AiProvider{}, err
		}
		add("temperature", temp)
	}
	if p.IsDefault != nil {
		add("is_default", *p.IsDefault)
	}
	if p.IsActive != nil {
		add("is_active", *p.IsActive)
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	sql := fmt.Sprintf("UPDATE ai_providers SET %s WHERE id = $%d RETURNING %s",
		strings.Join(sets, ", "), len(args), selectCols)

	row, err := scanRow(r.pool.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return AiProvider{}, ErrNotFound
	}
	if err != nil {
		return AiProvider{}, err
	}
	return row, nil
}

// SoftDelete clears is_default + is_active (Node softDelete).
func (r *PgRepo) SoftDelete(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	return r.q.SoftDeleteAiProvider(ctx, uid)
}

// scanRow maps the 11 selectCols (in order) from a pgx.Row into an
// AiProvider. temperature is already text (the cast), so it scans straight
// into a Go string.
func scanRow(row pgx.Row) (AiProvider, error) {
	var (
		id                   pgtype.UUID
		typ                  string
		createdAt, updatedAt pgtype.Timestamp
		p                    AiProvider
	)
	if err := row.Scan(
		&id, &p.Name, &typ, &p.EndpointURL, &p.APIKey, &p.Model, &p.Temperature,
		&p.IsDefault, &p.IsActive, &createdAt, &updatedAt,
	); err != nil {
		return AiProvider{}, err
	}
	p.ID = uuidStr(id)
	p.Type = typ
	p.CreatedAt = tsTime(createdAt)
	p.UpdatedAt = tsTime(updatedAt)
	return p, nil
}

// numeric parses a decimal string ("0.30") into pgtype.Numeric for the
// INSERT/UPDATE bind. pgtype.Numeric.Scan handles the text→numeric parse;
// the numeric(3,2) column coerces to scale 2 regardless, and reads come
// back via the temperature::text cast.
func numeric(s string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// tsTime unwraps a pgtype.Timestamp; an invalid (NULL) timestamp yields the
// zero time. These columns are NOT NULL, so Valid is always true in practice.
func tsTime(t pgtype.Timestamp) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// parseUUID builds a pgtype.UUID from a string id (matches the user repo
// idiom). An unparseable id returns the error the callers map to ErrNotFound.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

// uuidStr renders a pgtype.UUID as canonical lowercase hyphenated form.
func uuidStr(u pgtype.UUID) string { return u.String() }
