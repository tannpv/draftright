-- internal/shared/pg/queries_payment.sql
-- Payment persistence (read paths for Phase 3a). Table public.payments is
-- owned by the live NestJS-managed schema; mirror PaymentService read
-- semantics exactly. Timestamps are "without time zone".

-- name: GetPaymentByReference :one
-- getStatus(referenceCode): the pending/settled payment plus the plan name the
-- controller surfaces as plan_name. LEFT JOIN so a payment whose plan row was
-- deleted still returns (plan_name NULL), matching TypeORM relations:['plan']
-- with a possibly-null relation.
SELECT
  p.id, p.user_id, p.plan_id, p.amount, p.currency, p.method, p.status,
  p.provider_ref, p.reference_code, p.qr_data, p.notes,
  p.expires_at, p.completed_at, p.created_at, p.updated_at,
  pl.name AS plan_name
FROM payments p
LEFT JOIN plans pl ON pl.id = p.plan_id
WHERE p.reference_code = $1
LIMIT 1;

-- name: ListPaymentsByUser :many
-- findByUser(userId): the user's 20 most-recent payments, newest first, each
-- with its plan (TypeORM relations:['plan'] order:{created_at:'DESC'} take:20).
SELECT
  p.id, p.user_id, p.plan_id, p.amount, p.currency, p.method, p.status,
  p.provider_ref, p.reference_code, p.qr_data, p.notes,
  p.expires_at, p.completed_at, p.created_at, p.updated_at,
  pl.id AS plan_pk, pl.name AS plan_name, pl.daily_limit AS plan_daily_limit,
  pl.price_cents AS plan_price_cents, pl.currency AS plan_currency,
  pl.stripe_price_id AS plan_stripe_price_id, pl.trial_days AS plan_trial_days,
  pl.billing_period AS plan_billing_period, pl.is_active AS plan_is_active,
  pl.created_at AS plan_created_at, pl.updated_at AS plan_updated_at
FROM payments p
LEFT JOIN plans pl ON pl.id = p.plan_id
WHERE p.user_id = $1
ORDER BY p.created_at DESC
LIMIT 20;

-- name: GetPlanForCheckout :one
-- Full plan entity createCheckout needs: price_cents drives the free-plan guard +
-- payment.amount, the strategy reads name/stripe_price_id/trial_days/billing_period,
-- and the nested `payment.plan` response object serializes the WHOLE plan
-- (daily_limit/is_active/created_at/updated_at included — Node attaches
-- plansService.findById, the full entity). No row → pgx.ErrNoRows.
SELECT id, name, daily_limit, price_cents, currency, stripe_price_id, trial_days,
       billing_period, is_active, created_at, updated_at
FROM plans
WHERE id = $1;

-- name: GetUserForCheckout :one
-- User fields createCheckout / portal / cancel need. No row → pgx.ErrNoRows.
SELECT id, email, stripe_customer_id, lemonsqueezy_customer_id
FROM users
WHERE id = $1;

-- name: CreatePayment :one
-- Insert a pending payment. Defaults (id, created_at, updated_at) are returned.
INSERT INTO payments (user_id, plan_id, amount, currency, method, status, reference_code, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at, updated_at;

-- name: UpdatePaymentQRData :exec
UPDATE payments SET qr_data = $2, updated_at = now() WHERE id = $1;

-- name: MarkPaymentFailed :exec
UPDATE payments SET status = 'failed', notes = $2, updated_at = now() WHERE id = $1;

-- name: GetPaymentForWebhook :one
-- completePayment / activate-subscription projection: the webhook resolves a
-- payment by reference, then needs user/plan ids + current status + currency +
-- the plan's billing_period (to re-resolve the active plan by variant). LEFT
-- JOIN so billing_period is nullable in this row even though the column is NOT
-- NULL on plans (payments always carry a plan in practice).
SELECT pay.id, pay.user_id, pay.plan_id, pay.status, pay.currency, pay.method,
       p.billing_period
FROM payments pay
LEFT JOIN plans p ON p.id = pay.plan_id
WHERE pay.reference_code = $1
LIMIT 1;

-- name: MarkPaymentCompleted :exec
UPDATE payments SET status = 'completed', completed_at = NOW(), updated_at = NOW()
WHERE reference_code = $1;

-- name: MarkPaymentFailedByRef :exec
UPDATE payments SET status = 'failed', updated_at = NOW()
WHERE reference_code = $1;

-- name: SetUserStripeCustomerID :exec
UPDATE users SET stripe_customer_id = $2, updated_at = NOW() WHERE id = $1;

-- name: SetUserLemonSqueezyCustomerID :exec
UPDATE users SET lemonsqueezy_customer_id = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdatePaymentPlan :exec
UPDATE payments SET plan_id = $2, updated_at = NOW() WHERE id = $1;

-- name: FindFirstActivePlanByPeriodCurrency :one
-- Ports plansService.findFirstActive({is_active, billing_period, currency},
-- order created_at ASC). Returns the active plan id for that (period, currency).
SELECT id FROM plans
WHERE is_active = true AND billing_period = $1 AND currency = $2
ORDER BY created_at ASC
LIMIT 1;

-- name: GetUserEmailName :one
-- activateSubscription's notify step: the webhook needs the paying user's email
-- + display name to send the "subscription active" mail. name is NOT NULL
-- (schema.sql), so sqlc generates a plain string (no pointer).
SELECT email, name FROM users WHERE id = $1;

-- name: PaymentStats :one
-- getStats(): aggregate counts + completed revenue for GET /admin/payments/stats.
-- Mirrors payment.service getStats — total/completed/pending counts plus
-- COALESCE(SUM(amount),0) over completed rows. revenue is cast ::bigint so sqlc
-- types it int64 (no numeric); the repo narrows to int (Node parseInt).
SELECT
  COUNT(*) AS total,
  COUNT(*) FILTER (WHERE status = 'completed') AS completed,
  COUNT(*) FILTER (WHERE status = 'pending') AS pending,
  COALESCE(SUM(amount) FILTER (WHERE status = 'completed'), 0)::bigint AS revenue
FROM payments;
