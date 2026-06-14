package payment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the payment repo needs (consumer-side port so
// tests fake it without a DB).
type Querier interface {
	GetPaymentByReference(ctx context.Context, referenceCode string) (sqlc.GetPaymentByReferenceRow, error)
	ListPaymentsByUser(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListPaymentsByUserRow, error)
}

// StatusRow is the GetByReference projection feeding the status endpoint.
// PlanName/CompletedAt/ExpiresAt are nil when the underlying column is NULL.
type StatusRow struct {
	Status        string
	Method        string
	Amount        int
	Currency      string
	ReferenceCode string
	PlanName      *string
	CompletedAt   *time.Time
	ExpiresAt     *time.Time
}

// PaymentRow is the full ListByUser projection (every payments column + the
// joined plan), serialised by the history endpoint to match TypeORM's entity
// JSON. Nullable columns are pointers.
type PaymentRow struct {
	ID            string
	UserID        string
	PlanID        string
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
	Plan          *PlanBrief
}

// PlanBrief is the nested plan object on a history row, field-for-field with
// plans.PlanEntity (same column set TypeORM loads via relations:['plan']).
type PlanBrief struct {
	ID            string
	Name          string
	DailyLimit    int
	PriceCents    int
	Currency      *string
	StripePriceID *string
	TrialDays     int
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repo is the concrete persistence adapter over the sqlc Querier.
type Repo struct{ q Querier }

// NewRepo wires the querier (accept interface, return struct).
func NewRepo(q Querier) *Repo { return &Repo{q: q} }

// GetByReference returns the payment for a reference code, or (nil,nil) when
// none exists (Node returns null → controller emits {status:"not_found"}).
func (r *Repo) GetByReference(ctx context.Context, ref string) (*StatusRow, error) {
	row, err := r.q.GetPaymentByReference(ctx, ref)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &StatusRow{
		Status:        row.Status,
		Method:        row.Method,
		Amount:        int(row.Amount),
		Currency:      row.Currency,
		ReferenceCode: row.ReferenceCode,
		PlanName:      row.PlanName,
		CompletedAt:   tsPtr(row.CompletedAt),
		ExpiresAt:     tsPtr(row.ExpiresAt),
	}, nil
}

// ListByUser returns the user's 20 newest payments, each with its plan. Always
// returns a non-nil slice ([] not null). A malformed user id yields empty.
func (r *Repo) ListByUser(ctx context.Context, userID string) ([]PaymentRow, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return []PaymentRow{}, nil
	}
	rows, err := r.q.ListPaymentsByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentRow, 0, len(rows))
	for _, row := range rows {
		pr := PaymentRow{
			ID:            uuid.UUID(row.ID.Bytes).String(),
			UserID:        uuid.UUID(row.UserID.Bytes).String(),
			PlanID:        uuid.UUID(row.PlanID.Bytes).String(),
			Amount:        int(row.Amount),
			Currency:      row.Currency,
			Method:        row.Method,
			Status:        row.Status,
			ProviderRef:   row.ProviderRef,
			ReferenceCode: row.ReferenceCode,
			QRData:        row.QrData,
			Notes:         row.Notes,
			ExpiresAt:     tsPtr(row.ExpiresAt),
			CompletedAt:   tsPtr(row.CompletedAt),
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		}
		if row.PlanPk.Valid {
			pr.Plan = &PlanBrief{
				ID:            uuid.UUID(row.PlanPk.Bytes).String(),
				Name:          derefStr(row.PlanName),
				DailyLimit:    derefInt(row.PlanDailyLimit),
				PriceCents:    derefInt(row.PlanPriceCents),
				Currency:      row.PlanCurrency,
				StripePriceID: row.PlanStripePriceID,
				TrialDays:     derefInt(row.PlanTrialDays),
				BillingPeriod: derefBillingPeriod(row.PlanBillingPeriod),
				IsActive:      derefBool(row.PlanIsActive),
				CreatedAt:     row.PlanCreatedAt.Time,
				UpdatedAt:     row.PlanUpdatedAt.Time,
			}
		}
		out = append(out, pr)
	}
	return out, nil
}

// --- pgtype / pointer helpers --------------------------------------------

func tsPtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int32) int {
	if i == nil {
		return 0
	}
	return int(*i)
}

func derefBool(b *bool) bool {
	return b != nil && *b
}

func derefBillingPeriod(p *sqlc.PlansBillingPeriodEnum) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
