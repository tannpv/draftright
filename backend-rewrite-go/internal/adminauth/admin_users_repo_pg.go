// Admin-users CRUD Postgres adapter (Phase 4c-2). Mirrors plans/admin_repo_pg.go:
//
//   - STATIC queries (ListAll, EmailExists, Insert, SoftDelete) go through the
//     sqlc-generated *sqlc.Queries — compile-time checked.
//   - DYNAMIC queries (ListPaginated's SELECT/COUNT, Update's partial SET) are
//     assembled from listquery.Built / a non-nil patch field set and run on the
//     pgxpool directly, because their WHERE/ORDER/columns aren't known until
//     runtime. listquery.Build's allow-listed columns are the SQL-injection
//     guard; adminUsersPatchSQL's column names are literals owned by this file,
//     never user input.
//
// Every projection omits password_hash so the secret never leaves the DB and
// the response struct (AdminUserOut, 7 keys) can never carry it.
package adminauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// ErrAdminNotFound is returned by Update when the target id has no row. The
// Task 15 handler maps it to a 500 (Node findOneOrFail throws → 500), distinct
// from SoftDelete which never reports not-found (Node returns success
// unconditionally).
var ErrAdminNotFound = errors.New("admin user not found")

// AdminUserOut is the password_hash-stripped admin_users row as the admin
// endpoints serialise it: snake_case, 7 keys in entity-declaration order minus
// the secret. MarshalJSON pins field order + ms timestamps.
type AdminUserOut struct {
	ID        string
	Email     string
	Name      string
	IsActive  bool
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (a AdminUserOut) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		IsActive  bool   `json:"is_active"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		ID: a.ID, Email: a.Email, Name: a.Name, IsActive: a.IsActive, Role: a.Role,
		CreatedAt: shared.ISOMillis(a.CreatedAt), UpdatedAt: shared.ISOMillis(a.UpdatedAt),
	})
}

// NewAdminUser is the create payload — PasswordHash is already bcrypt-hashed by
// the caller (Task 15 usecase). Role defaulting ('admin') happens upstream.
type NewAdminUser struct {
	Email        string
	PasswordHash string
	Name         string
	Role         string
}

// AdminUserPatch is the update payload — a nil pointer = field unchanged
// (TypeORM partial .update()). PasswordHash is non-nil only when the caller
// supplied a truthy password (Node `if (body.password)`).
type AdminUserPatch struct {
	Name         *string
	Email        *string
	Role         *string
	IsActive     *bool
	PasswordHash *string
}

// AdminUsersRepo serves the static admin-user queries via sqlc and the dynamic
// ones via the pool. One per process; the pgxpool is shared and owned by the
// caller.
type AdminUsersRepo struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewAdminUsersRepo wires the sqlc querier + the shared pool.
func NewAdminUsersRepo(q *sqlc.Queries, pool *pgxpool.Pool) *AdminUsersRepo {
	return &AdminUsersRepo{q: q, pool: pool}
}

// adminUserSelectCols is the 7-col projection shared by the dynamic read paths
// (paginated SELECT, Update RETURNING). password_hash is deliberately absent.
const adminUserSelectCols = "id, email, name, is_active, role, created_at, updated_at"

// adminUsersPaginatedSQL emits the rows SELECT for ListPaginated. LIMIT/OFFSET
// stay as %d for the caller to fill with positional arg indexes (they depend on
// how many WHERE args listquery.Built produced).
func adminUsersPaginatedSQL(where, order string) string {
	return fmt.Sprintf("SELECT %s FROM admin_users %s %s LIMIT $%%d OFFSET $%%d",
		adminUserSelectCols, where, order)
}

// adminUsersCountSQL emits the total-count query, sharing the runtime WHERE.
func adminUsersCountSQL(where string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM admin_users %s", where)
}

// ListAll returns every admin user ordered by created_at ASC (Node
// adminUserRepo.find order ASC). Non-nil empty slice → JSON `[]`.
func (r *AdminUsersRepo) ListAll(ctx context.Context) ([]AdminUserOut, error) {
	rows, err := r.q.ListAdminUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdminUserOut, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminUserOut{
			ID:        uuidStr(row.ID),
			Email:     row.Email,
			Name:      row.Name,
			IsActive:  row.IsActive,
			Role:      row.Role,
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return out, nil
}

// ListPaginated runs the dynamic WHERE/ORDER/LIMIT/OFFSET from listquery.Built,
// then a COUNT(*) over the same WHERE. Returns a non-nil empty slice when no
// rows match.
func (r *AdminUsersRepo) ListPaginated(ctx context.Context, b listquery.Built) ([]AdminUserOut, int, error) {
	limPos := len(b.Args) + 1
	offPos := len(b.Args) + 2
	rowsSQL := fmt.Sprintf(adminUsersPaginatedSQL(b.Where, b.Order), limPos, offPos)

	args := make([]any, 0, len(b.Args)+2)
	args = append(args, b.Args...)
	args = append(args, b.Limit, b.Offset)

	rows, err := r.pool.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AdminUserOut{}
	for rows.Next() {
		a, err := scanAdminUser(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.pool.QueryRow(ctx, adminUsersCountSQL(b.Where), b.Args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// EmailExists reports whether an admin row has the EXACT email (case-sensitive,
// Node findOne({ where: { email } }) — NOT lowercased).
func (r *AdminUsersRepo) EmailExists(ctx context.Context, email string) (bool, error) {
	return r.q.AdminEmailExists(ctx, email)
}

// Insert writes a new admin (PasswordHash already hashed by the caller) and
// returns the created row without the secret.
func (r *AdminUsersRepo) Insert(ctx context.Context, in NewAdminUser) (AdminUserOut, error) {
	row, err := r.q.InsertAdminUser(ctx, sqlc.InsertAdminUserParams{
		Email:        in.Email,
		PasswordHash: in.PasswordHash,
		Name:         in.Name,
		Role:         in.Role,
	})
	if err != nil {
		return AdminUserOut{}, err
	}
	return AdminUserOut{
		ID:        uuidStr(row.ID),
		Email:     row.Email,
		Name:      row.Name,
		IsActive:  row.IsActive,
		Role:      row.Role,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

// Update applies the non-nil patch fields via a dynamic UPDATE ... SET ...
// RETURNING, mirroring TypeORM's partial .update() (only provided keys written)
// followed by findOneOrFail's re-read. A missing id (no RETURNING row) yields
// ErrAdminNotFound so the handler maps missing → 500.
func (r *AdminUsersRepo) Update(ctx context.Context, id string, p AdminUserPatch) (AdminUserOut, error) {
	set, args := adminUsersPatchSQL(p)
	args = append(args, id)
	sql := fmt.Sprintf("UPDATE admin_users SET %s WHERE id = $%d RETURNING %s",
		set, len(args), adminUserSelectCols)

	a, err := scanAdminUser(r.pool.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminUserOut{}, ErrAdminNotFound
	}
	return a, err
}

// SoftDelete clears is_active. ALWAYS returns nil on success even if 0 rows
// matched (Node returns { success: true } unconditionally, no existence check).
func (r *AdminUsersRepo) SoftDelete(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.q.SoftDeleteAdminUser(ctx, uid)
}

// adminUsersPatchSQL builds the SET clause from the non-nil patch fields,
// walking fields in domain order (deterministic — no map range), with hardcoded
// column names (injection-safe) and $N value placeholders. `updated_at = now()`
// is always appended last (TypeORM @UpdateDateColumn), guaranteeing ≥1 SET col.
func adminUsersPatchSQL(p AdminUserPatch) (set string, args []any) {
	var sets []string
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.Name != nil {
		add("name", *p.Name)
	}
	if p.Email != nil {
		add("email", *p.Email)
	}
	if p.Role != nil {
		add("role", *p.Role)
	}
	if p.IsActive != nil {
		add("is_active", *p.IsActive)
	}
	if p.PasswordHash != nil {
		add("password_hash", *p.PasswordHash)
	}
	sets = append(sets, "updated_at = now()")
	return strings.Join(sets, ", "), args
}

// scanAdminUser maps the 7 adminUserSelectCols (in order) from a pgx.Row into
// an AdminUserOut. password_hash is never selected, so the scan never touches
// the secret.
func scanAdminUser(row pgx.Row) (AdminUserOut, error) {
	var (
		id                   pgtype.UUID
		createdAt, updatedAt pgtype.Timestamp
		a                    AdminUserOut
	)
	if err := row.Scan(
		&id, &a.Email, &a.Name, &a.IsActive, &a.Role, &createdAt, &updatedAt,
	); err != nil {
		return AdminUserOut{}, err
	}
	a.ID = uuidStr(id)
	a.CreatedAt = createdAt.Time
	a.UpdatedAt = updatedAt.Time
	return a, nil
}
