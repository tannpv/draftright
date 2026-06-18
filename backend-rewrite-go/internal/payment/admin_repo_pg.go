// Admin payment Postgres adapter (Phase 4c-3). Mirrors plans/admin_repo_pg.go
// and user/admin_repo_pg.go:
//
//   - The STATIC aggregate (Stats) goes through the sqlc-generated
//     *sqlc.Queries — compile-time checked.
//   - The DYNAMIC bespoke list (FindAll: runtime WHERE from status/search +
//     ORDER from a 6-key allow-list, LEFT JOIN to users + plans for the nested
//     relations) is assembled in Go and run on the pgxpool directly, because
//     its WHERE/ORDER aren't known until runtime.
//
// Injection guard: ORDER BY columns come only from the fixed paymentSortMap
// allow-list (mirroring Node payment.service findAll's sortMap), the direction
// is a hardcoded ASC/DESC literal, and every value (status, the search %term%,
// LIMIT/OFFSET) binds as a $N placeholder. Raw sort_by / sort_order strings
// NEVER reach the SQL text.
//
// Bespoke (NON-listquery) list semantics — byte-identical to Node
// payment.service findAll. listquery.Build is deliberately NOT used: it pushes
// a BOOLEAN status filter (wrong — payment.status is a string enum) and clamps
// limit to [1,100] (wrong — Node take(limit) has no clamp).
//
//   - leftJoinAndSelect user + plan: Node has NO ClassSerializerInterceptor, so
//     the nested `user` is the RAW user row (incl password_hash + reset fields)
//     and `plan` is the raw plan row. We project every user column (UserDetail
//     order) + every plan column (PlanEntity order) and assemble the nested
//     *user.UserDetail / *plans.PlanEntity, which carry pinned MarshalJSON that
//     reproduce Node's raw-entity key order. A LEFT-JOIN miss → the nested
//     object is nil → JSON null.
//   - status (non-empty): payment.status = $N (STRING equality).
//   - search (non-empty, trimmed): the 5-col ILIKE over reference_code, the
//     joined user.email / user.name / plan.name, and payment.method, bound to
//     one %term% arg.
//   - sort_by allow-list (reference_code, amount, method, status, created_at,
//     user.email) → its qualified column; anything else / unset → p.created_at.
//     sort_order 'ASC' iff exactly "ASC", else DESC.
//   - OFFSET (page-1)*limit LIMIT limit (NO clamp); total via COUNT(*) over the
//     same FROM+joins+WHERE (Node getManyAndCount = 2 queries; joins kept
//     because the search references the joined u/pl columns).
package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

// AdminRepo runs the admin payment reads. It holds the sqlc Queries for the
// static aggregate (Stats) and a *pgxpool.Pool for the dynamic FindAll.
type AdminRepo struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewAdminRepo wires the sqlc querier + the shared pool.
func NewAdminRepo(q *sqlc.Queries, pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{q: q, pool: pool}
}

// Stats maps the PaymentStats aggregate row (int64 → int, Node parseInt parity).
func (r *AdminRepo) Stats(ctx context.Context) (PaymentStats, error) {
	row, err := r.q.PaymentStats(ctx)
	if err != nil {
		return PaymentStats{}, err
	}
	return PaymentStats{
		Total:     int(row.Total),
		Completed: int(row.Completed),
		Pending:   int(row.Pending),
		Revenue:   int(row.Revenue),
	}, nil
}

// AdminPaymentRow is one full Payment entity with its nested raw user + plan,
// as Node's leftJoinAndSelect serialises it (no ClassSerializerInterceptor).
// MarshalJSON pins the Payment-entity field order; the nested *user.UserDetail
// and *plans.PlanEntity emit via their own pinned MarshalJSON (nil → null).
type AdminPaymentRow struct {
	ID            string
	UserID        string
	User          *user.UserDetail
	PlanID        string
	Plan          *plans.PlanEntity
	Amount        int
	Currency      string
	Method        string
	Status        string
	ProviderRef   *string
	ReferenceCode string
	QRData        *string
	Notes         *string
	ExpiresAt     *time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (p AdminPaymentRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string            `json:"id"`
		UserID        string            `json:"user_id"`
		User          *user.UserDetail  `json:"user"`
		PlanID        string            `json:"plan_id"`
		Plan          *plans.PlanEntity `json:"plan"`
		Amount        int               `json:"amount"`
		Currency      string            `json:"currency"`
		Method        string            `json:"method"`
		Status        string            `json:"status"`
		ProviderRef   *string           `json:"provider_ref"`
		ReferenceCode string            `json:"reference_code"`
		QRData        *string           `json:"qr_data"`
		Notes         *string           `json:"notes"`
		ExpiresAt     *string           `json:"expires_at"`
		CompletedAt   *string           `json:"completed_at"`
		CreatedAt     string            `json:"created_at"`
		UpdatedAt     string            `json:"updated_at"`
	}{
		ID:            p.ID,
		UserID:        p.UserID,
		User:          p.User,
		PlanID:        p.PlanID,
		Plan:          p.Plan,
		Amount:        p.Amount,
		Currency:      p.Currency,
		Method:        p.Method,
		Status:        p.Status,
		ProviderRef:   p.ProviderRef,
		ReferenceCode: p.ReferenceCode,
		QRData:        p.QRData,
		Notes:         p.Notes,
		ExpiresAt:     isoPtr(p.ExpiresAt),
		CompletedAt:   isoPtr(p.CompletedAt),
		CreatedAt:     shared.ISOMillis(p.CreatedAt),
		UpdatedAt:     shared.ISOMillis(p.UpdatedAt),
	})
}

// paymentSortMap is the sort_by allow-list (mirrors Node sortMap). The joined
// aliases are p (payments), u (users), pl (plans). Any key not present resolves
// to the default p.created_at.
var paymentSortMap = map[string]string{
	"reference_code": "p.reference_code",
	"amount":         "p.amount",
	"method":         "p.method",
	"status":         "p.status",
	"created_at":     "p.created_at",
	"user.email":     "u.email",
}

// resolvePaymentSortCol maps an untrusted sort_by to an allow-listed column,
// falling back to p.created_at. The returned value is always a literal owned by
// this file — never the caller's string — the injection guard for ORDER.
func resolvePaymentSortCol(sortBy string) string {
	if col, ok := paymentSortMap[sortBy]; ok {
		return col
	}
	return "p.created_at"
}

// resolvePaymentSortOrder returns "ASC" iff the input is exactly "ASC", else
// "DESC" (Node: sort_order === 'ASC' ? 'ASC' : 'DESC').
func resolvePaymentSortOrder(order string) string {
	if order == "ASC" {
		return "ASC"
	}
	return "DESC"
}

// paymentUserCols projects every users column in UserDetail entity-declaration
// order, aliased u, with the enum columns cast ::text so the pool scan lands Go
// strings. A LEFT-JOIN miss makes the whole block NULL (even the table's
// NOT-NULL columns), so the scan reads them as nullable temporaries.
const paymentUserCols = "u.id, u.email, u.password_hash, u.name, u.is_active, " +
	"u.role::text, u.auth_provider::text, u.google_id, u.facebook_id, u.tiktok_id, " +
	"u.apple_id, u.avatar_url, u.stripe_customer_id, u.email_verified, " +
	"u.email_verification_code, u.email_verification_expires, u.password_reset_code, " +
	"u.password_reset_expires, u.password_reset_attempts, u.lemonsqueezy_customer_id, " +
	"u.created_at, u.updated_at"

// paymentPlanCols projects every plans column in PlanEntity entity-declaration
// order, aliased pl, billing_period cast ::text. A LEFT-JOIN miss → all NULL.
const paymentPlanCols = "pl.id, pl.name, pl.daily_limit, pl.price_cents, pl.currency, " +
	"pl.stripe_price_id, pl.trial_days, pl.billing_period::text, pl.is_active, " +
	"pl.created_at, pl.updated_at"

// paymentCols projects every payments column, aliased p, in entity order.
const paymentCols = "p.id, p.user_id, p.plan_id, p.amount, p.currency, p.method, " +
	"p.status, p.provider_ref, p.reference_code, p.qr_data, p.notes, p.expires_at, " +
	"p.completed_at, p.created_at, p.updated_at"

// paymentFrom is the shared FROM + LEFT JOINs (kept on both the rows and count
// queries because the search predicate references u/pl columns).
const paymentFrom = "FROM payments p " +
	"LEFT JOIN users u ON u.id = p.user_id " +
	"LEFT JOIN plans pl ON pl.id = p.plan_id"

// paymentFindAllSQL builds the rows + count SQL and the bound value args for
// FindAll. sortCol/sortOrder come pre-resolved through the allow-list helpers.
// The LIMIT/OFFSET placeholders are left as %d for the caller to fill with the
// positional arg indexes (those depend on how many WHERE args were produced).
// limit/offset are returned for binding: limit = params.Limit (NO clamp),
// offset = (page-1)*limit with page defaulted to 1.
func paymentFindAllSQL(p FindAllParams) (rowsSQL, countSQL string, args []any, limit, offset int) {
	sortCol := resolvePaymentSortCol(p.SortBy)
	sortOrder := resolvePaymentSortOrder(p.SortOrder)

	var conds []string
	// add binds one value and renders cond, replacing every "$#" token with the
	// value's positional $N (so a predicate may reference the same arg many
	// times, e.g. the 5-col ILIKE).
	add := func(cond string, v any) {
		args = append(args, v)
		ph := fmt.Sprintf("$%d", len(args))
		conds = append(conds, strings.ReplaceAll(cond, "$#", ph))
	}
	if p.Status != "" {
		add("p.status = $#", p.Status)
	}
	if trim := strings.TrimSpace(p.Search); trim != "" {
		add("(p.reference_code ILIKE $# OR u.email ILIKE $# OR u.name ILIKE $# "+
			"OR pl.name ILIKE $# OR p.method ILIKE $#)", "%"+trim+"%")
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	order := fmt.Sprintf("ORDER BY %s %s", sortCol, sortOrder)

	selectCols := paymentCols + ", " + paymentUserCols + ", " + paymentPlanCols
	rowsSQL = fmt.Sprintf("SELECT %s %s %s %s LIMIT $%%d OFFSET $%%d",
		selectCols, paymentFrom, where, order)
	countSQL = fmt.Sprintf("SELECT COUNT(*) %s %s", paymentFrom, where)

	page := p.Page
	if page < 1 {
		page = 1
	}
	limit = p.Limit
	offset = (page - 1) * limit
	return rowsSQL, countSQL, args, limit, offset
}

// FindAll runs the bespoke paginated list (LEFT JOIN user + plan) plus a
// COUNT(*) over the same WHERE. Returns a non-nil empty slice when no rows
// match.
func (r *AdminRepo) FindAll(ctx context.Context, p FindAllParams) ([]AdminPaymentRow, int, error) {
	rowsTmpl, countSQL, args, limit, offset := paymentFindAllSQL(p)

	limPos := len(args) + 1
	offPos := len(args) + 2
	rowsSQL := fmt.Sprintf(rowsTmpl, limPos, offPos)

	rowsArgs := make([]any, 0, len(args)+2)
	rowsArgs = append(rowsArgs, args...)
	rowsArgs = append(rowsArgs, limit, offset)

	rows, err := r.pool.Query(ctx, rowsSQL, rowsArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AdminPaymentRow{}
	for rows.Next() {
		row, err := scanPayment(rows)
		if err != nil {
			return nil, 0, err
		}
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

// scanPayment maps one row (paymentCols, then paymentUserCols, then
// paymentPlanCols, in order) into an AdminPaymentRow. The user/plan blocks are
// scanned into nullable temporaries; the nested object is assembled only when
// its id is non-NULL (a LEFT-JOIN miss leaves every joined column NULL, even
// the source table's NOT-NULL ones). user_id/plan_id come from the always-
// present p.user_id / p.plan_id.
func scanPayment(row pgx.Row) (AdminPaymentRow, error) {
	var (
		pID, pUserID, pPlanID                   pgtype.UUID
		amount                                  int32
		currency, method, status                string
		providerRef                             *string
		referenceCode                           string
		qrData, notes                           *string
		expiresAt, completedAt                  pgtype.Timestamp
		createdAt, updatedAt                    pgtype.Timestamp
		uID                                     pgtype.UUID
		uEmail                                  *string
		uPasswordHash                           *string
		uName                                   *string
		uIsActive                               *bool
		uRole, uAuthProvider                    *string
		uGoogleID, uFacebookID, uTiktokID       *string
		uAppleID, uAvatarURL, uStripeCustomerID *string
		uEmailVerified                          *bool
		uEmailVerificationCode                  *string
		uEmailVerificationExpires               pgtype.Timestamptz
		uPasswordResetCode                      *string
		uPasswordResetExpires                   pgtype.Timestamptz
		uPasswordResetAttempts                  *int32
		uLemonsqueezyCustomerID                 *string
		uCreatedAt, uUpdatedAt                  pgtype.Timestamp
		plID                                    pgtype.UUID
		plName                                  *string
		plDailyLimit, plPriceCents              *int32
		plCurrency, plStripePriceID             *string
		plTrialDays                             *int32
		plBillingPeriod                         *string
		plIsActive                              *bool
		plCreatedAt, plUpdatedAt                pgtype.Timestamp
	)

	if err := row.Scan(
		// payments (entity order)
		&pID, &pUserID, &pPlanID, &amount, &currency, &method, &status,
		&providerRef, &referenceCode, &qrData, &notes, &expiresAt, &completedAt,
		&createdAt, &updatedAt,
		// users (UserDetail order)
		&uID, &uEmail, &uPasswordHash, &uName, &uIsActive, &uRole, &uAuthProvider,
		&uGoogleID, &uFacebookID, &uTiktokID, &uAppleID, &uAvatarURL,
		&uStripeCustomerID, &uEmailVerified, &uEmailVerificationCode,
		&uEmailVerificationExpires, &uPasswordResetCode, &uPasswordResetExpires,
		&uPasswordResetAttempts, &uLemonsqueezyCustomerID, &uCreatedAt, &uUpdatedAt,
		// plans (PlanEntity order)
		&plID, &plName, &plDailyLimit, &plPriceCents, &plCurrency, &plStripePriceID,
		&plTrialDays, &plBillingPeriod, &plIsActive, &plCreatedAt, &plUpdatedAt,
	); err != nil {
		return AdminPaymentRow{}, err
	}

	out := AdminPaymentRow{
		ID:            uuid.UUID(pID.Bytes).String(),
		UserID:        uuid.UUID(pUserID.Bytes).String(),
		PlanID:        uuid.UUID(pPlanID.Bytes).String(),
		Amount:        int(amount),
		Currency:      currency,
		Method:        method,
		Status:        status,
		ProviderRef:   providerRef,
		ReferenceCode: referenceCode,
		QRData:        qrData,
		Notes:         notes,
		ExpiresAt:     tsPtr(expiresAt),
		CompletedAt:   tsPtr(completedAt),
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
	}

	if uID.Valid {
		out.User = &user.UserDetail{
			ID:                       uuid.UUID(uID.Bytes).String(),
			Email:                    derefStr(uEmail),
			PasswordHash:             uPasswordHash,
			Name:                     derefStr(uName),
			IsActive:                 derefBool(uIsActive),
			Role:                     derefStr(uRole),
			AuthProvider:             derefStr(uAuthProvider),
			GoogleID:                 uGoogleID,
			FacebookID:               uFacebookID,
			TiktokID:                 uTiktokID,
			AppleID:                  uAppleID,
			AvatarURL:                uAvatarURL,
			StripeCustomerID:         uStripeCustomerID,
			EmailVerified:            derefBool(uEmailVerified),
			EmailVerificationCode:    uEmailVerificationCode,
			EmailVerificationExpires: tstzPtr(uEmailVerificationExpires),
			PasswordResetCode:        uPasswordResetCode,
			PasswordResetExpires:     tstzPtr(uPasswordResetExpires),
			PasswordResetAttempts:    derefInt(uPasswordResetAttempts),
			LemonsqueezyCustomerID:   uLemonsqueezyCustomerID,
			CreatedAt:                uCreatedAt.Time,
			UpdatedAt:                uUpdatedAt.Time,
		}
	}

	if plID.Valid {
		out.Plan = &plans.PlanEntity{
			ID:            uuid.UUID(plID.Bytes).String(),
			Name:          derefStr(plName),
			DailyLimit:    derefInt(plDailyLimit),
			PriceCents:    derefInt(plPriceCents),
			Currency:      plCurrency,
			StripePriceID: plStripePriceID,
			TrialDays:     derefInt(plTrialDays),
			BillingPeriod: derefStr(plBillingPeriod),
			IsActive:      derefBool(plIsActive),
			CreatedAt:     plCreatedAt.Time,
			UpdatedAt:     plUpdatedAt.Time,
		}
	}

	return out, nil
}

// paymentDetailSelect is the single-row projection for LoadPaymentWithPlan:
// every payments column (entity order) + every plans column (PlanEntity order),
// LEFT JOIN plans (a deleted plan → all plan columns NULL → nested plan nil →
// JSON null), matching Node adminConfirm's findOne(relations:['plan']). NO user
// join — adminConfirm does not load the user relation.
const paymentDetailSelect = "SELECT " + paymentCols + ", " + paymentPlanCols + " " +
	"FROM payments p LEFT JOIN plans pl ON pl.id = p.plan_id WHERE p.id = $1 LIMIT 1"

// LoadPaymentWithPlan loads one payment by id with its plan relation (no user),
// returning ErrPaymentNotFound when no row matches. Run on the pool (single row,
// reusing the same scan style as FindAll) rather than via sqlc because it shares
// the FindAll column ordering for the nested *plans.PlanEntity assembly.
func (r *AdminRepo) LoadPaymentWithPlan(ctx context.Context, id string) (AdminPaymentDetailRow, error) {
	uid, ok := parseUUID(id)
	if !ok {
		return AdminPaymentDetailRow{}, ErrPaymentNotFound
	}
	row, err := scanPaymentDetail(r.pool.QueryRow(ctx, paymentDetailSelect, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminPaymentDetailRow{}, ErrPaymentNotFound
	}
	if err != nil {
		return AdminPaymentDetailRow{}, err
	}
	return row, nil
}

// scanPaymentDetail maps one row (paymentCols then paymentPlanCols, in order)
// into an AdminPaymentDetailRow. The plan block is scanned into nullable
// temporaries; the nested *plans.PlanEntity is assembled only when its id is
// non-NULL (a LEFT-JOIN miss leaves every plan column NULL). No user block.
func scanPaymentDetail(row pgx.Row) (AdminPaymentDetailRow, error) {
	var (
		pID, pUserID, pPlanID       pgtype.UUID
		amount                      int32
		currency, method, status    string
		providerRef                 *string
		referenceCode               string
		qrData, notes               *string
		expiresAt, completedAt      pgtype.Timestamp
		createdAt, updatedAt        pgtype.Timestamp
		plID                        pgtype.UUID
		plName                      *string
		plDailyLimit, plPriceCents  *int32
		plCurrency, plStripePriceID *string
		plTrialDays                 *int32
		plBillingPeriod             *string
		plIsActive                  *bool
		plCreatedAt, plUpdatedAt    pgtype.Timestamp
	)

	if err := row.Scan(
		// payments (entity order)
		&pID, &pUserID, &pPlanID, &amount, &currency, &method, &status,
		&providerRef, &referenceCode, &qrData, &notes, &expiresAt, &completedAt,
		&createdAt, &updatedAt,
		// plans (PlanEntity order)
		&plID, &plName, &plDailyLimit, &plPriceCents, &plCurrency, &plStripePriceID,
		&plTrialDays, &plBillingPeriod, &plIsActive, &plCreatedAt, &plUpdatedAt,
	); err != nil {
		return AdminPaymentDetailRow{}, err
	}

	out := AdminPaymentDetailRow{
		ID:            uuid.UUID(pID.Bytes).String(),
		UserID:        uuid.UUID(pUserID.Bytes).String(),
		PlanID:        uuid.UUID(pPlanID.Bytes).String(),
		Amount:        int(amount),
		Currency:      currency,
		Method:        method,
		Status:        status,
		ProviderRef:   providerRef,
		ReferenceCode: referenceCode,
		QRData:        qrData,
		Notes:         notes,
		ExpiresAt:     tsPtr(expiresAt),
		CompletedAt:   tsPtr(completedAt),
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
	}

	if plID.Valid {
		out.Plan = &plans.PlanEntity{
			ID:            uuid.UUID(plID.Bytes).String(),
			Name:          derefStr(plName),
			DailyLimit:    derefInt(plDailyLimit),
			PriceCents:    derefInt(plPriceCents),
			Currency:      plCurrency,
			StripePriceID: plStripePriceID,
			TrialDays:     derefInt(plTrialDays),
			BillingPeriod: derefStr(plBillingPeriod),
			IsActive:      derefBool(plIsActive),
			CreatedAt:     plCreatedAt.Time,
			UpdatedAt:     plUpdatedAt.Time,
		}
	}

	return out, nil
}

// ConfirmPayment flips the payment to completed, stamping completed_at + notes
// (static sqlc :exec UPDATE). The use case supplies the injected now() and the
// already-defaulted notes.
func (r *AdminRepo) ConfirmPayment(ctx context.Context, id string, completedAt time.Time, notes string) error {
	uid, ok := parseUUID(id)
	if !ok {
		return ErrPaymentNotFound
	}
	return r.q.ConfirmPayment(ctx, sqlc.ConfirmPaymentParams{
		ID:          uid,
		CompletedAt: pgtype.Timestamp{Time: completedAt, Valid: true},
		Notes:       &notes,
	})
}

// RefundPayment flips the payment to refunded, overwriting notes with the
// composed refund note (static sqlc :exec UPDATE). The use case supplies the
// already-composed final notes string.
func (r *AdminRepo) RefundPayment(ctx context.Context, id, notes string) error {
	uid, ok := parseUUID(id)
	if !ok {
		return ErrPaymentNotFound
	}
	return r.q.RefundPayment(ctx, sqlc.RefundPaymentParams{
		ID:    uid,
		Notes: &notes,
	})
}

// tstzPtr converts a nullable pgtype.Timestamptz to *time.Time (the two user
// reset/verification timestamps are timestamptz; tsPtr in reader.go covers the
// payment-side timestamp columns).
func tstzPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
