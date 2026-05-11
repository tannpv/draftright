-- Payment Phase 3a — extend `plans` for multi-currency Stripe subscriptions + trial config
--
-- Adds:
--   currency          char(3)        — 'USD' / 'VND'; NULL for free tier
--   stripe_price_id   varchar(255)   — set after Stripe Price object creation (Stage 2)
--   trial_days        integer NOT NULL DEFAULT 30  — admin-changeable per plan
--
-- Replaces the legacy single Pro row with 4 paid plan rows
-- (Pro USD monthly/yearly, Pro VND monthly/yearly).
--
-- Idempotent: re-running this script is safe — IF NOT EXISTS guards on column
-- adds; DELETE only matches legacy row by name+price; INSERTs use ON CONFLICT DO NOTHING.

BEGIN;

-- ── Column additions ─────────────────────────────────────────
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS currency        CHAR(3),
    ADD COLUMN IF NOT EXISTS stripe_price_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS trial_days      INTEGER NOT NULL DEFAULT 30;

-- Free tier: trial_days irrelevant; explicitly NULL currency
UPDATE plans SET currency = NULL, trial_days = 0 WHERE price_cents = 0;

-- ── Convert legacy Pro row in-place (preserve existing FKs) ──
-- The pre-Phase3a 'Pro' row at 499 cents / monthly becomes the canonical
-- "Pro USD monthly" row by stamping currency = 'USD'. This keeps existing
-- subscriptions/payments referencing it valid — no FK breakage, no orphaned subs.
UPDATE plans
SET currency = 'USD'
WHERE name = 'Pro' AND currency IS NULL AND price_cents = 499 AND billing_period = 'monthly';

-- Unique constraint so the same (name, currency, billing_period) can't be
-- inserted twice. PG treats NULLs as distinct by default — fine for the
-- single (Free, NULL, none) row.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'plans'::regclass AND conname = 'plans_name_currency_period_uniq'
  ) THEN
    ALTER TABLE plans
      ADD CONSTRAINT plans_name_currency_period_uniq
      UNIQUE (name, currency, billing_period);
  END IF;
END $$;

-- ── Seed Pro tiers ──────────────────────────────────────────
-- Pricing approved 2026-05-09: USD $4.99/mo, $39.99/yr; VND 99,000/mo, 799,000/yr.
-- daily_limit -1 = unlimited. 30-day trial on every paid plan.

-- Storage convention: column holds Stripe's native amount unit per currency.
-- USD: cents (×100) — $4.99 = 499.
-- VND: whole VND (no divisor) — 99,000 VND = 99000.
-- Column name `price_cents` is misleading for VND but renaming is deferred to
-- Phase 3c (downstream consumers + admin UI need updating). The stripe.strategy
-- and presenter layers must NOT multiply by 100 when currency is VND.

INSERT INTO plans (name, daily_limit, price_cents, currency, billing_period, trial_days, is_active)
VALUES
    ('Pro', -1,    499, 'USD', 'monthly', 30, true),  -- $4.99 USD
    ('Pro', -1,   3999, 'USD', 'yearly',  30, true),  -- $39.99 USD
    ('Pro', -1,  99000, 'VND', 'monthly', 30, true),  -- 99,000 VND
    ('Pro', -1, 799000, 'VND', 'yearly',  30, true)   -- 799,000 VND
ON CONFLICT ON CONSTRAINT plans_name_currency_period_uniq DO NOTHING;

COMMIT;
