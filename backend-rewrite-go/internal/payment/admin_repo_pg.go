package payment

import (
	"context"

	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AdminRepo runs the admin payment reads/writes. For now it holds only the
// sqlc Queries; a later task adds a *pgxpool.Pool for the dynamic FindAll sort.
type AdminRepo struct {
	q *sqlc.Queries
}

// NewAdminRepo wires the sqlc querier.
func NewAdminRepo(q *sqlc.Queries) *AdminRepo { return &AdminRepo{q: q} }

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
