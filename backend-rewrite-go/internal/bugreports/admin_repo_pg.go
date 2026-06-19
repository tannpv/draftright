// Bug-reports admin Postgres adapter (Phase 4c-4). Mirrors plans/admin_repo_pg.go:
//
//   - STATIC queries (AdminGet, AdminDelete, AdminUpdate, AdminSetFixProposal,
//     AdminFixCandidates) go through the sqlc-generated *sqlc.Queries —
//     compile-time checked.
//   - The DYNAMIC admin LIST (GET /admin/bug-reports) is a listquery consumer:
//     listquery.Built supplies the search ILIKE WHERE + ORDER BY + LIMIT/OFFSET
//     and the three custom filters (status/kind/target_platform) — already
//     validated + selected by the use case (Node findAllPaginated layers them
//     as qb.andWhere BEFORE applyListQuery) — bind as additional predicates.
//     Assembled in Go and run on the pgxpool because the WHERE/ORDER aren't
//     known until runtime.
//
// Injection guard: listquery.Build's search/sort columns come only from the
// fixed allow-list the use case passes to Build; the custom-filter column
// names here are literals; every value binds as a $N placeholder. Raw
// sort_by / sort_order strings NEVER reach the SQL text.
package bugreports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AdminPgRepo serves the static bug-report admin queries via sqlc and the
// dynamic list via the pool. One per process; the pgxpool is shared and owned
// by the caller.
type AdminPgRepo struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewAdminPgRepo wires the sqlc querier + the shared pool.
func NewAdminPgRepo(q *sqlc.Queries, pool *pgxpool.Pool) *AdminPgRepo {
	return &AdminPgRepo{q: q, pool: pool}
}

// mapBugEntity converts a sqlc bug_reports row to the admin entity. Used by
// every admin read/write path so the JSON projection is identical.
func mapBugEntity(r sqlc.BugReport) BugReportEntity {
	e := BugReportEntity{
		DisplayNo:          strconv.FormatInt(r.DisplayNo, 10),
		Source:             r.Source,
		Description:        r.Description,
		ScreenshotPath:     r.ScreenshotPath,
		ScreenshotFilename: r.ScreenshotFilename,
		AppVersion:         r.AppVersion,
		OsInfo:             r.OsInfo,
		UserEmail:          r.UserEmail,
		Context:            json.RawMessage(r.Context),
		Kind:               r.Kind,
		Title:              r.Title,
		TargetPlatform:     r.TargetPlatform,
		VoteCount:          int(r.VoteCount),
		IsPublic:           r.IsPublic,
		AdminNotes:         r.AdminNotes,
		AiFixProposal:      r.AiFixProposal,
	}
	if r.ID.Valid {
		e.ID = uuid.UUID(r.ID.Bytes).String()
	}
	if r.UserID.Valid {
		s := uuid.UUID(r.UserID.Bytes).String()
		e.UserID = &s
	}
	if r.Status != nil {
		e.Status = *r.Status
	}
	if r.AiFixProposedAt.Valid {
		s := shared.ISOMillis(r.AiFixProposedAt.Time)
		e.AiFixProposedAt = &s
	}
	if r.CreatedAt.Valid {
		e.CreatedAt = shared.ISOMillis(r.CreatedAt.Time)
	}
	if r.UpdatedAt.Valid {
		e.UpdatedAt = shared.ISOMillis(r.UpdatedAt.Time)
	}
	return e
}

// bugCustomFilters renders the status/kind/target_platform predicates that
// Node layers via qb.andWhere, binding each provided (already-validated) value
// as $N starting at startPos. Empty filter strings add nothing. Returns the
// predicate fragments + their args, in domain order (status, kind,
// target_platform) so placeholder numbering is deterministic.
func bugCustomFilters(f AdminListFilter, startPos int) (conds []string, args []any) {
	pos := startPos
	add := func(col, v string) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf("%s = $%d", col, pos))
		pos++
	}
	if f.Status != "" {
		add("br.status", f.Status)
	}
	if f.Kind != "" {
		add("br.kind", f.Kind)
	}
	if f.TargetPlatform != "" {
		add("br.target_platform", f.TargetPlatform)
	}
	return conds, args
}

// AdminList runs the dynamic paginated list + a COUNT(*) over the same WHERE,
// mirroring Node's getManyAndCount (2 queries). The custom filter predicates
// bind AFTER listquery.Built's search arg(s) so numbering stays consistent
// (AND order is SQL-irrelevant). Returns a non-nil empty slice when no rows
// match. f.Built carries the listquery search ILIKE WHERE + ORDER + page/limit.
func (r *AdminPgRepo) AdminList(ctx context.Context, f AdminListFilter) ([]BugReportEntity, int, error) {
	b := f.Built

	// Custom filters number after Built.Args; the LIMIT/OFFSET come last.
	custConds, custArgs := bugCustomFilters(f, len(b.Args)+1)
	whereArgs := make([]any, 0, len(b.Args)+len(custArgs))
	whereArgs = append(whereArgs, b.Args...)
	whereArgs = append(whereArgs, custArgs...)

	where := mergeWhere(b.Where, custConds)

	limPos := len(whereArgs) + 1
	offPos := len(whereArgs) + 2
	rowsSQL := fmt.Sprintf("SELECT %s FROM bug_reports br %s %s LIMIT $%d OFFSET $%d",
		bugReportCols, where, b.Order, limPos, offPos)

	rowsArgs := make([]any, 0, len(whereArgs)+2)
	rowsArgs = append(rowsArgs, whereArgs...)
	rowsArgs = append(rowsArgs, b.Limit, b.Offset)

	rows, err := r.pool.Query(ctx, rowsSQL, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []BugReportEntity{}
	for rows.Next() {
		row, err := scanBugReport(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, mapBugEntity(row))
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM bug_reports br %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// mergeWhere folds the custom-filter predicates into listquery.Built's WHERE
// fragment. Built.Where is either "" or starts with "WHERE ". The custom
// conditions AND onto it (or open a fresh WHERE when Built had none).
func mergeWhere(builtWhere string, custConds []string) string {
	if len(custConds) == 0 {
		return builtWhere
	}
	joined := strings.Join(custConds, " AND ")
	if builtWhere == "" {
		return "WHERE " + joined
	}
	return builtWhere + " AND " + joined
}

// bugReportCols is the explicit projection for the dynamic admin list, in the
// exact order scanBugReport reads them. It MUST match the sqlc static reads'
// column list. A bare `SELECT *` would bind by the table's PHYSICAL column
// order, which can differ from this logical order (e.g. columns added by later
// migrations) → "can't scan into dest[N] (col: os_info): cannot parse UUID"
// when a text column lands in a UUID dest. See issue #44.
const bugReportCols = "id, source, description, screenshot_path, screenshot_filename, " +
	"app_version, os_info, user_id, user_email, context, status, admin_notes, " +
	"created_at, updated_at, ai_fix_proposal, ai_fix_proposed_at, kind, title, " +
	"target_platform, vote_count, is_public, display_no"

// scanBugReport maps a bug_reports row (projected via bugReportCols) into the
// sqlc model in that column order, so mapBugEntity can reuse the same
// projection as the static reads.
func scanBugReport(row pgx.Row) (sqlc.BugReport, error) {
	var b sqlc.BugReport
	err := row.Scan(
		&b.ID, &b.Source, &b.Description, &b.ScreenshotPath, &b.ScreenshotFilename,
		&b.AppVersion, &b.OsInfo, &b.UserID, &b.UserEmail, &b.Context, &b.Status,
		&b.AdminNotes, &b.CreatedAt, &b.UpdatedAt, &b.AiFixProposal, &b.AiFixProposedAt,
		&b.Kind, &b.Title, &b.TargetPlatform, &b.VoteCount, &b.IsPublic, &b.DisplayNo,
	)
	return b, err
}

// AdminGet loads one bug-report row, returning ErrNotFound when absent (Node
// findById → 404 NotFoundException).
func (r *AdminPgRepo) AdminGet(ctx context.Context, id string) (BugReportEntity, error) {
	uid, perr := parseBugUUID(id)
	if perr != nil {
		return BugReportEntity{}, ErrNotFound
	}
	row, err := r.q.AdminGetBugReport(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return BugReportEntity{}, ErrNotFound
	}
	if err != nil {
		return BugReportEntity{}, err
	}
	return mapBugEntity(row), nil
}

// AdminDelete hard-deletes by id; returns whether a row was removed. The use
// case checks existence first (404), so a 0-row delete here is benign.
func (r *AdminPgRepo) AdminDelete(ctx context.Context, id string) (bool, error) {
	uid, perr := parseBugUUID(id)
	if perr != nil {
		return false, nil
	}
	affected, err := r.q.AdminDeleteBugReport(ctx, uid)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// AdminUpdate writes the already-merged patch columns and returns the saved
// row (Node update → repo.save then return entity).
func (r *AdminPgRepo) AdminUpdate(ctx context.Context, id string, status string, adminNotes, title, targetPlatform *string, isPublic bool) (BugReportEntity, error) {
	uid, perr := parseBugUUID(id)
	if perr != nil {
		return BugReportEntity{}, ErrNotFound
	}
	st := status
	row, err := r.q.AdminUpdateBugReport(ctx, sqlc.AdminUpdateBugReportParams{
		ID:             uid,
		Status:         &st,
		AdminNotes:     adminNotes,
		Title:          title,
		TargetPlatform: targetPlatform,
		IsPublic:       isPublic,
	})
	if err != nil {
		return BugReportEntity{}, err
	}
	return mapBugEntity(row), nil
}

// AdminSetFixProposal stores the AI proposal + proposed-at + new status,
// returning the saved row (Node suggestFix → ai_fix_proposal +
// ai_fix_proposed_at + conditional status).
func (r *AdminPgRepo) AdminSetFixProposal(ctx context.Context, id string, proposal string, proposedAt time.Time, status string) (BugReportEntity, error) {
	uid, perr := parseBugUUID(id)
	if perr != nil {
		return BugReportEntity{}, ErrNotFound
	}
	var pa pgtype.Timestamptz
	pa.Time = proposedAt
	pa.Valid = true
	st := status
	p := proposal
	row, err := r.q.AdminSetBugFixProposal(ctx, sqlc.AdminSetBugFixProposalParams{
		ID:              uid,
		AiFixProposal:   &p,
		AiFixProposedAt: pa,
		Status:          &st,
	})
	if err != nil {
		return BugReportEntity{}, err
	}
	return mapBugEntity(row), nil
}

// AdminFixCandidates returns the ids of the newest un-analyzed bug rows
// (Node cron: status='new', kind='bug', ai_fix_proposal IS NULL, LIMIT 5).
func (r *AdminPgRepo) AdminFixCandidates(ctx context.Context, limit int32) ([]string, error) {
	rows, err := r.q.AdminBugFixCandidates(ctx, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID.Valid {
			ids = append(ids, uuid.UUID(row.ID.Bytes).String())
		}
	}
	return ids, nil
}

// parseBugUUID builds a pgtype.UUID from a string id (for the static reads).
func parseBugUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}
