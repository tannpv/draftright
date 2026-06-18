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

// paymentAdminRepo is the AdminService's consumer-side port. *AdminRepo
// satisfies it. (More methods get added by later 4c-3 tasks.)
type paymentAdminRepo interface {
	Stats(ctx context.Context) (PaymentStats, error)
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
