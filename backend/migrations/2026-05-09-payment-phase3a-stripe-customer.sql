-- Payment Phase 3a — Stripe customer reuse + subscription tracking
--
-- Adds:
--   users.stripe_customer_id    — populated on first Stripe checkout, reused on subsequent
--   subscriptions store_type 'stripe' enum value — for native Stripe subs (not Lemon Squeezy)
--
-- The existing subscriptions.store_transaction_id (varchar 500) is reused
-- to hold the Stripe Subscription ID (sub_XXXX). No new column needed.
--
-- Idempotent.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS users_stripe_customer_id_idx
    ON users (stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL;

-- Add 'stripe' to subscriptions_store_type_enum if not present.
-- PG can't add enum values inside a transaction in older versions; use savepoint pattern.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum
    WHERE enumlabel = 'stripe'
      AND enumtypid = 'subscriptions_store_type_enum'::regtype
  ) THEN
    ALTER TYPE subscriptions_store_type_enum ADD VALUE 'stripe';
  END IF;
END $$;

-- Index by store_transaction_id (for stripe webhook dedup lookups by sub_XXXX)
CREATE INDEX IF NOT EXISTS subscriptions_store_txn_idx
    ON subscriptions (store_transaction_id)
    WHERE store_transaction_id IS NOT NULL;

COMMIT;
