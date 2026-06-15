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
