-- Extend subscriptions.store_type enum with VietQR / BankTransfer / PayPal.
--
-- Required because the LemonSqueezy webhook now correctly stamps
-- `lemonsqueezy` on the resulting subscription row (previously fell
-- through to `admin_granted`), and the analogous correctness fix for
-- VietQR / bank-transfer / PayPal needs the corresponding enum values
-- to exist server-side.
--
-- On dev `synchronize: true` adds these automatically when TypeORM
-- starts.  Prod (`synchronize: false`) needs this migration run BEFORE
-- the new backend image starts — otherwise every activate-subscription
-- INSERT crashes with `invalid input value for enum`.
--
-- Operator step on prod:
--   docker exec -i postgres-prod psql -U draftright -d draftright \
--     < backend/sql/2026-06-01-store-type-extend.sql
--   (then) deploy new backend image
--
-- Idempotent: `ADD VALUE IF NOT EXISTS` skips when the value already
-- exists (Postgres 9.6+).
ALTER TYPE subscriptions_store_type_enum ADD VALUE IF NOT EXISTS 'vietqr';
ALTER TYPE subscriptions_store_type_enum ADD VALUE IF NOT EXISTS 'bank_transfer';
ALTER TYPE subscriptions_store_type_enum ADD VALUE IF NOT EXISTS 'paypal';
