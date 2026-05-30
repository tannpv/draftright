-- Lemon Squeezy (Merchant of Record / credit cards) settings columns.
-- Prod runs TypeORM with synchronize:false, so new @Column fields must be
-- added by hand or every query on app_settings 500s. Run BEFORE deploying the
-- backend build that adds these columns to AppSettings.
--
-- Idempotent: safe to re-run.
ALTER TABLE app_settings
  ADD COLUMN IF NOT EXISTS lemonsqueezy_api_key        VARCHAR(2000) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS lemonsqueezy_store_id       VARCHAR(50)   NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS lemonsqueezy_webhook_secret VARCHAR(100)  NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS lemonsqueezy_variant_monthly VARCHAR(50)  NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS lemonsqueezy_variant_yearly  VARCHAR(50)  NOT NULL DEFAULT '';

-- Configure the live values (store 361149, variants Monthly=1710530 / Yearly=1710538).
-- Replace <API_KEY> and <WEBHOOK_SECRET> with the real secrets (do NOT commit them).
-- Also enable the method on the storefront (append to the CSV; keep existing ones).
UPDATE app_settings SET
  lemonsqueezy_store_id        = '361149',
  lemonsqueezy_variant_monthly = '1710530',
  lemonsqueezy_variant_yearly  = '1710538',
  lemonsqueezy_api_key         = '<API_KEY>',
  lemonsqueezy_webhook_secret  = '<WEBHOOK_SECRET>',
  payment_methods_enabled = CASE
    WHEN payment_methods_enabled = '' OR payment_methods_enabled IS NULL THEN 'lemonsqueezy'
    WHEN payment_methods_enabled LIKE '%lemonsqueezy%' THEN payment_methods_enabled
    ELSE payment_methods_enabled || ',lemonsqueezy'
  END;
