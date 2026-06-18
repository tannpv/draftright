// Admin email-logs Postgres adapter (bespoke list). Mirrors user/admin_repo_pg.go
// ListUsers: the dynamic paginated SELECT + COUNT over the same WHERE run on the
// pgxpool directly because the status filter is runtime-optional. Simpler than
// ListUsers — only an optional `status = $N` predicate, no search, no sort
// allow-list, fixed ORDER BY created_at DESC.
//
// Two queries (mirroring Node findAndCount):
//   - rows:  SELECT <cols> FROM email_logs {WHERE status=$1} ORDER BY created_at DESC LIMIT $.. OFFSET $..
//   - count: SELECT COUNT(*) FROM email_logs {WHERE status=$1}
//
// No sqlc query file is added for this list — like ListUsers (which also uses no
// sqlc query for its dynamic list), the whole thing is assembled on the pool.
package email

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminLogsRepo runs the bespoke list on the shared pool. The pgxpool is owned
// by the caller (wired in main.go in a later task).
type AdminLogsRepo struct {
	pool *pgxpool.Pool
}

// NewAdminLogsRepo wires the shared pool.
func NewAdminLogsRepo(pool *pgxpool.Pool) *AdminLogsRepo {
	return &AdminLogsRepo{pool: pool}
}

// emailLogCols is the 8-column projection (entity declaration order).
const emailLogCols = "id, to_email, email_type, subject, status, provider_id, error, created_at"

// ListLogs runs the bespoke paginated list + a COUNT(*) over the same WHERE.
// Page/Limit arrive already clamped from the handler (Node min/max), but the
// OFFSET is clamped to ≥ 0 defensively so a stray negative limit can't produce
// a negative offset. Returns a non-nil empty slice when no rows match.
func (r *AdminLogsRepo) ListLogs(ctx context.Context, p AdminLogsParams) ([]EmailLogRow, int, error) {
	var where string
	var args []any
	if p.Status != "" {
		args = append(args, p.Status)
		where = fmt.Sprintf("WHERE status = $%d", len(args))
	}

	offset := (p.Page - 1) * p.Limit
	if offset < 0 {
		offset = 0
	}

	limPos := len(args) + 1
	offPos := len(args) + 2
	rowsSQL := fmt.Sprintf(
		"SELECT %s FROM email_logs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		emailLogCols, where, limPos, offPos,
	)
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM email_logs %s", where)

	rowsArgs := make([]any, 0, len(args)+2)
	rowsArgs = append(rowsArgs, args...)
	rowsArgs = append(rowsArgs, p.Limit, offset)

	rows, err := r.pool.Query(ctx, rowsSQL, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []EmailLogRow{}
	for rows.Next() {
		var (
			id         pgtype.UUID
			providerID pgtype.Text
			errText    pgtype.Text
			createdAt  pgtype.Timestamptz
			row        EmailLogRow
		)
		if err := rows.Scan(&id, &row.ToEmail, &row.EmailType, &row.Subject, &row.Status, &providerID, &errText, &createdAt); err != nil {
			return nil, 0, err
		}
		row.ID = id.String()
		if providerID.Valid {
			v := providerID.String
			row.ProviderID = &v
		}
		if errText.Valid {
			v := errText.String
			row.Error = &v
		}
		row.CreatedAt = createdAt.Time
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
