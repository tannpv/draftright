package payment

import "context"

// PaymentStats is the GET /admin/payments/stats body. Field order = JSON key
// order, matching Node payment.service getStats return
// ({ total, completed, pending, revenue }).
type PaymentStats struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
	Revenue   int `json:"revenue"`
}

// FindAllParams carries the GET /admin/payments query inputs. Page/Limit are
// already defaulted by the handler (Node defaults page=1, limit=20).
// SortOrder is already normalised to "ASC"/"DESC" by the handler; the repo
// re-applies the ASC-iff-"ASC" rule defensively. NO clamping of Limit (Node
// payment findAll calls take(limit) with no clamp).
type FindAllParams struct {
	Page, Limit int
	Status      string
	Search      string
	SortBy      string
	SortOrder   string
}

// PaymentsPage is the GET /admin/payments body. Field order = JSON key order,
// matching Node payment.service findAll return ({ payments, total }).
type PaymentsPage struct {
	Payments []AdminPaymentRow `json:"payments"`
	Total    int               `json:"total"`
}

// paymentAdminRepo is the AdminService's consumer-side port. *AdminRepo
// satisfies it. (More methods get added by later 4c-3 tasks.)
type paymentAdminRepo interface {
	Stats(ctx context.Context) (PaymentStats, error)
	FindAll(ctx context.Context, p FindAllParams) ([]AdminPaymentRow, int, error)
}

// AdminService serves the admin payment routes (stats now; findAll/confirm/
// refund added by later tasks). Separate from the webhook Service to keep that
// 12-dep struct untouched.
type AdminService struct {
	repo paymentAdminRepo
}

// NewAdminService accepts the consumer port; *AdminRepo satisfies it for the
// composition root, a fake satisfies it for tests (accept interfaces, return
// structs).
func NewAdminService(repo paymentAdminRepo) *AdminService { return &AdminService{repo: repo} }

// GetStats returns aggregate payment counts + completed revenue.
func (s *AdminService) GetStats(ctx context.Context) (PaymentStats, error) {
	return s.repo.Stats(ctx)
}

// FindAll returns the paginated payments + total, wrapped into PaymentsPage.
// A nil rows slice is normalised to [] so the JSON emits an empty array, never
// null (Node always returns an array). Errors surface with a zero PaymentsPage.
func (s *AdminService) FindAll(ctx context.Context, p FindAllParams) (PaymentsPage, error) {
	rows, total, err := s.repo.FindAll(ctx, p)
	if err != nil {
		return PaymentsPage{}, err
	}
	if rows == nil {
		rows = []AdminPaymentRow{}
	}
	return PaymentsPage{Payments: rows, Total: total}, nil
}
