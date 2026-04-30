-- 2026-04-30: payment foundation for Lemon Squeezy
-- 1. Add 'lemonsqueezy' to subscriptions.store_type enum
-- 2. Add users.lemonsqueezy_customer_id for Customer Portal lookups
-- 3. Insert Pro plan ($4.99/mo, unlimited)
-- 4. Tighten Free plan to 20/day (was -1 for dogfooding)

ALTER TYPE subscriptions_store_type_enum ADD VALUE IF NOT EXISTS 'lemonsqueezy';

ALTER TABLE users ADD COLUMN IF NOT EXISTS lemonsqueezy_customer_id varchar(64);

INSERT INTO plans (name, daily_limit, price_cents, billing_period, is_active)
SELECT 'Pro', -1, 499, 'monthly', true
WHERE NOT EXISTS (SELECT 1 FROM plans WHERE name = 'Pro');

UPDATE plans SET daily_limit = 20 WHERE name = 'Free' AND daily_limit = -1;
