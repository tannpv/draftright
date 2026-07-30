-- #50: widen secret columns to TEXT before at-rest encryption.
--
-- AES-256-GCM ciphertext (enc:v1:base64(nonce||ct||tag)) is materially longer
-- than the plaintext secret, so the existing varchar(100)/varchar(500) widths
-- would overflow (e.g. paypal_webhook_id varchar(100)). TEXT is unbounded.
--
-- Idempotent: ALTER ... TYPE text is a no-op when already text. Run BEFORE
-- setting SECRETS_ENCRYPTION_KEY and running scripts/encrypt-secrets.ts.
ALTER TABLE ai_providers ALTER COLUMN api_key TYPE text;

ALTER TABLE app_settings ALTER COLUMN stripe_secret_key           TYPE text;
ALTER TABLE app_settings ALTER COLUMN stripe_webhook_secret       TYPE text;
ALTER TABLE app_settings ALTER COLUMN paypal_client_secret        TYPE text;
ALTER TABLE app_settings ALTER COLUMN paypal_webhook_id           TYPE text;
ALTER TABLE app_settings ALTER COLUMN momo_access_key             TYPE text;
ALTER TABLE app_settings ALTER COLUMN momo_secret_key             TYPE text;
ALTER TABLE app_settings ALTER COLUMN casso_api_key               TYPE text;
ALTER TABLE app_settings ALTER COLUMN sepay_api_key               TYPE text;
ALTER TABLE app_settings ALTER COLUMN resend_api_key              TYPE text;
ALTER TABLE app_settings ALTER COLUMN google_client_secret        TYPE text;
ALTER TABLE app_settings ALTER COLUMN lemonsqueezy_api_key        TYPE text;
ALTER TABLE app_settings ALTER COLUMN lemonsqueezy_webhook_secret TYPE text;
