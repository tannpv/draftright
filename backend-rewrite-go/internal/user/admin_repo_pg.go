// Admin user Postgres adapter (Phase 4c-2). Mirrors plans/admin_repo_pg.go:
//
//   - The STATIC full-row read (GetFull, 22 columns) goes through the
//     sqlc-generated *sqlc.Queries — compile-time checked.
//   - The DYNAMIC bespoke list (ListUsers: runtime WHERE from status/search +
//     ORDER from an allow-list) and the partial Update (only the non-nil patch
//     fields) are assembled in Go and run on the pgxpool directly, because
//     their WHERE/ORDER/SET aren't known until runtime.
//
// Injection guard: ORDER BY columns come only from a fixed allow-list
// (sortMap, mirroring Node usersService.findAll's sortMap), the sort
// direction is a hardcoded ASC/DESC literal, and every value (search %term%,
// the is_active flag, LIMIT/OFFSET) binds as a $N placeholder. Raw sort_by /
// sort_order strings NEVER reach the SQL text.
//
// Bespoke (NON-listquery) list semantics — byte-identical to Node
// usersService.findAll (src/users/users.service.ts findAll):
//   - page default 1, limit default 20.
//   - status: ” / 'all' → no is_active filter; 'active' → is_active = true;
//     'inactive' → is_active = false.
//   - search (non-empty): (email ILIKE %term% OR name ILIKE %term%).
//   - sort_by allow-list (email, name, role, is_active, created_at) → bare
//     column; anything else / unset → created_at. sort_order 'ASC' iff exactly
//     "ASC", else DESC.
//   - OFFSET (page-1)*limit LIMIT limit; total via COUNT(*) over the same
//     WHERE (Node getManyAndCount = 2 queries).
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AdminRepo serves the static full-row read via sqlc and the dynamic
// list/update via the pool. One per process; the pgxpool is shared and owned
// by the caller.
type AdminRepo struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewAdminRepo wires the sqlc querier + the shared pool.
func NewAdminRepo(q *sqlc.Queries, pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{q: q, pool: pool}
}

// ListUsersParams carries the bespoke-list inputs. Page/Limit are already
// defaulted by the caller (the handler applies 1/20); ListUsers re-applies the
// same defaults defensively so the repo is correct in isolation.
type ListUsersParams struct {
	Search    string
	Status    string
	SortBy    string
	SortOrder string
	Page      int
	Limit     int
}

// UserListRow is the slim projection the admin list usecase consumes. Node
// fetches full entities then the controller maps to exactly these 6 fields
// (plan/usage_today are added later via sibling ports), so the rows query
// selects only these columns — the discarded full-entity fetch in Node is
// invisible to the client.
type UserListRow struct {
	ID        string
	Email     string
	Name      string
	Role      string
	IsActive  bool
	CreatedAt time.Time
}

// userListCols is the 6-column projection for the bespoke list rows.
const userListCols = "id, email, name, role, is_active, created_at"

// userSortMap is the sort_by allow-list (mirrors Node sortMap, columns bare).
// Any key not present resolves to the default created_at.
var userSortMap = map[string]string{
	"email":      "email",
	"name":       "name",
	"role":       "role",
	"is_active":  "is_active",
	"created_at": "created_at",
}

// resolveSortCol maps an untrusted sort_by to an allow-listed column, falling
// back to created_at. The returned value is always a literal owned by this
// file — never the caller's string — which is the injection guard for ORDER.
func resolveSortCol(sortBy string) string {
	if col, ok := userSortMap[sortBy]; ok {
		return col
	}
	return "created_at"
}

// resolveSortOrder returns "ASC" iff the input is exactly "ASC", else "DESC"
// (Node: sort_order === 'ASC' ? 'ASC' : 'DESC').
func resolveSortOrder(order string) string {
	if order == "ASC" {
		return "ASC"
	}
	return "DESC"
}

// userListSQL builds the rows + count SQL for the bespoke list. sortCol/
// sortOrder MUST already be allow-list-resolved literals (see resolveSortCol /
// resolveSortOrder). status maps to an optional is_active predicate; a
// non-empty search adds the ILIKE predicate. Value args (search term, the
// is_active flag) are returned for $N binding; LIMIT/OFFSET placeholders are
// appended to rowsSQL by the caller (their indexes depend on how many args
// the WHERE produced). countSQL shares the identical WHERE.
func userListSQL(status, search, sortCol, sortOrder string) (rowsSQL, countSQL string, args []any) {
	var conds []string
	// add binds one value and renders cond, replacing every "$#" token with the
	// value's positional $N (so a predicate may reference the same arg twice,
	// e.g. the ILIKE pair).
	add := func(cond string, v any) {
		args = append(args, v)
		ph := fmt.Sprintf("$%d", len(args))
		conds = append(conds, strings.ReplaceAll(cond, "$#", ph))
	}
	if status == "active" {
		add("is_active = $#", true)
	} else if status == "inactive" {
		add("is_active = $#", false)
	}
	if search != "" {
		add("(email ILIKE $# OR name ILIKE $#)", "%"+search+"%")
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	order := fmt.Sprintf("ORDER BY %s %s", sortCol, sortOrder)

	rowsSQL = fmt.Sprintf("SELECT %s FROM users %s %s LIMIT $%%d OFFSET $%%d",
		userListCols, where, order)
	countSQL = fmt.Sprintf("SELECT COUNT(*) FROM users %s", where)
	return rowsSQL, countSQL, args
}

// ListUsers runs the bespoke paginated list + a COUNT(*) over the same WHERE.
// Returns a non-nil empty slice when no rows match.
func (r *AdminRepo) ListUsers(ctx context.Context, p ListUsersParams) ([]UserListRow, int, error) {
	page, limit := p.Page, p.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	sortCol := resolveSortCol(p.SortBy)
	sortOrder := resolveSortOrder(p.SortOrder)
	rowsTmpl, countSQL, args := userListSQL(p.Status, p.Search, sortCol, sortOrder)

	limPos := len(args) + 1
	offPos := len(args) + 2
	rowsSQL := fmt.Sprintf(rowsTmpl, limPos, offPos)

	rowsArgs := make([]any, 0, len(args)+2)
	rowsArgs = append(rowsArgs, args...)
	rowsArgs = append(rowsArgs, limit, (page-1)*limit)

	rows, err := r.pool.Query(ctx, rowsSQL, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []UserListRow{}
	for rows.Next() {
		var (
			id        pgtype.UUID
			role      sqlc.UsersRoleEnum
			createdAt pgtype.Timestamp
			row       UserListRow
		)
		if err := rows.Scan(&id, &row.Email, &row.Name, &role, &row.IsActive, &createdAt); err != nil {
			return nil, 0, err
		}
		row.ID = uuidStr(id)
		row.Role = string(role)
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

// GetFull returns the complete 22-column user row (Node usersService.findById,
// surfaced by GET /admin/users/:id). A missing/malformed id or no row →
// ErrNotFound (the package sentinel, mirroring the other reads).
func (r *AdminRepo) GetFull(ctx context.Context, id string) (UserDetail, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return UserDetail{}, ErrNotFound
	}
	row, err := r.q.GetUserFull(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserDetail{}, ErrNotFound
	}
	if err != nil {
		return UserDetail{}, err
	}
	return UserDetail{
		ID:                       uuidStr(row.ID),
		Email:                    row.Email,
		PasswordHash:             row.PasswordHash,
		Name:                     row.Name,
		IsActive:                 row.IsActive,
		Role:                     string(row.Role),
		AuthProvider:             string(row.AuthProvider),
		GoogleID:                 row.GoogleID,
		FacebookID:               row.FacebookID,
		TiktokID:                 row.TiktokID,
		AppleID:                  row.AppleID,
		AvatarURL:                row.AvatarUrl,
		StripeCustomerID:         row.StripeCustomerID,
		EmailVerified:            row.EmailVerified,
		EmailVerificationCode:    row.EmailVerificationCode,
		EmailVerificationExpires: tsValue(row.EmailVerificationExpires),
		PasswordResetCode:        row.PasswordResetCode,
		PasswordResetExpires:     tsValue(row.PasswordResetExpires),
		PasswordResetAttempts:    int(row.PasswordResetAttempts),
		LemonsqueezyCustomerID:   row.LemonsqueezyCustomerID,
		CreatedAt:                row.CreatedAt.Time,
		UpdatedAt:                row.UpdatedAt.Time,
	}, nil
}

// Update applies the non-nil patch fields via a dynamic UPDATE ... SET,
// mirroring TypeORM's partial .update() (only provided keys are written) plus
// the @UpdateDateColumn (updated_at = now()). The caller (Task 13) re-reads
// via GetFull, so returning error is sufficient. Fields are walked in a fixed
// order (no map range) with hardcoded column literals + $N values.
func (r *AdminRepo) Update(ctx context.Context, id string, p UserPatchAdmin) error {
	var sets []string
	var args []any
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.IsActive != nil {
		add("is_active", *p.IsActive)
	}
	if p.Role != nil {
		add("role", *p.Role)
	}
	if p.Name != nil {
		add("name", *p.Name)
	}
	sets = append(sets, "updated_at = now()")

	args = append(args, id)
	sql := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", strings.Join(sets, ", "), len(args))
	_, err := r.pool.Exec(ctx, sql, args...)
	return err
}
