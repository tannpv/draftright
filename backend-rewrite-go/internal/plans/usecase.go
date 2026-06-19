package plans

import "context"

// Lister is the consumer-side port the handler needs.
type Lister interface {
	ListActive(ctx context.Context) ([]PlanEntity, error)
}

// Service is the plans use case. Trivial today (one passthrough); it exists
// so the HTTP edge depends on a port, matching the module recipe.
type Service struct{ r Lister }

// NewService wires the lister.
func NewService(r Lister) *Service { return &Service{r: r} }

// ListActive returns active plans for GET /plans.
func (s *Service) ListActive(ctx context.Context) ([]PlanEntity, error) {
	return s.r.ListActive(ctx)
}
