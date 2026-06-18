-- Plans admin CRUD (Phase 4c-2). GET /plans (reader.go) is cheapest-first
-- (price_cents ASC); the ADMIN findAll orders by created_at ASC — see
-- Node plansService.findAll() (`order: { created_at: 'ASC' }`). The
-- billing_period column is the plans_billing_period_enum; sqlc types it as
-- the generated PlansBillingPeriodEnum, so InsertPlan binds the enum value
-- directly (no SQL cast needed — the param already carries the enum type).

-- name: ListAllPlans :many
SELECT id, name, daily_limit, price_cents, currency, stripe_price_id,
       trial_days, billing_period, is_active, created_at, updated_at
FROM plans ORDER BY created_at ASC;

-- name: InsertPlan :one
INSERT INTO plans (name, daily_limit, price_cents, billing_period, currency, trial_days, stripe_price_id)
VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id, name, daily_limit, price_cents, currency, stripe_price_id,
          trial_days, billing_period, is_active, created_at, updated_at;

-- name: SoftDeletePlan :exec
UPDATE plans SET is_active = false WHERE id = $1;
