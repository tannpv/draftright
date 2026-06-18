package plans

import (
	"context"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// plansStore is the admin usecase's consumer-side persistence port, satisfied
// by Task 9's concrete *AdminRepo. Named distinctly from that concrete struct
// (AdminRepo) per the clean-arch guardrail: the consumer owns the interface,
// the adapter owns the implementation. Methods are 1:1 the Task-9 methods this
// service uses (the public reader's findAll/findFirstActive etc. live on the
// other Service and are not needed here).
type plansStore interface {
	ListAll(ctx context.Context) ([]PlanEntity, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]PlanEntity, int, error)
	Create(ctx context.Context, in NewPlan) (PlanEntity, error)
	Update(ctx context.Context, id string, p PlanPatch) (PlanEntity, error)
	SoftDelete(ctx context.Context, id string) error
}

// AdminService is the admin plans usecase — parity with Node PlansService as
// driven by the admin controller's plans routes. A thin pass-through: the
// dual-mode list decision (bare array vs { rows, total }) lives in the handler,
// matching Node's controller-level branch (admin.controller.ts listPlans).
type AdminService struct {
	repo plansStore
}

// NewAdminService wires the store.
func NewAdminService(repo plansStore) *AdminService { return &AdminService{repo: repo} }

// ListAll returns every plan ordered created_at ASC (Node findAll).
func (s *AdminService) ListAll(ctx context.Context) ([]PlanEntity, error) {
	return s.repo.ListAll(ctx)
}

// ListPaginated returns the paginated/filtered set (Node findAllPaginated).
func (s *AdminService) ListPaginated(ctx context.Context, b listquery.Built) ([]PlanEntity, int, error) {
	return s.repo.ListPaginated(ctx, b)
}

// Create inserts a plan (Node create). Defaulting of absent optionals is the
// handler's job (it builds NewPlan); the repo binds whatever it carries.
func (s *AdminService) Create(ctx context.Context, in NewPlan) (PlanEntity, error) {
	return s.repo.Create(ctx, in)
}

// Update patches a plan then re-reads it (Node update → findOneOrFail). A
// missing id surfaces as a repo error, which the handler maps to 500 (parity
// with TypeORM's EntityNotFoundError → AllExceptionsFilter internal).
func (s *AdminService) Update(ctx context.Context, id string, p PlanPatch) (PlanEntity, error) {
	return s.repo.Update(ctx, id, p)
}

// SoftDelete clears is_active (Node softDelete).
func (s *AdminService) SoftDelete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}
