import { Entity, PrimaryGeneratedColumn, Column, UpdateDateColumn, ValueTransformer } from 'typeorm';
import { encryptSecret, decryptSecret } from '../../common/crypto/secret-cipher';

/**
 * At-rest encryption for the secret columns (#50). `to` encrypts on write,
 * `from` decrypts on read — transparent to every consumer of AppSettings.
 * Null-safe and a no-op without SECRETS_ENCRYPTION_KEY, so legacy plaintext
 * rows keep working. Columns are `text` because ciphertext is longer than the
 * original secret and would overflow the old varchar(100/500) widths.
 */
const secretTransformer: ValueTransformer = {
  to: (value?: string | null) => (value == null ? value : encryptSecret(value)),
  from: (value?: string | null) => (value == null ? value : decryptSecret(value)),
};

@Entity('app_settings')
export class AppSettings {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  // --- General ---
  @Column({ type: 'varchar', length: 20, default: 'testing' })
  environment: string;

  @Column({ type: 'int', default: 3 })
  trial_limit: number;

  @Column({ type: 'int', default: 15 })
  token_expiry_minutes: number;

  @Column({ type: 'int', default: 90 })
  refresh_token_expiry_days: number;

  @Column({ type: 'int', default: 3000 })
  max_input_length: number;

  @Column({ type: 'text', default: 'Arabic,Chinese (Simplified),Chinese (Traditional),Czech,Danish,Dutch,English,Finnish,French,German,Greek,Hebrew,Hindi,Hungarian,Indonesian,Italian,Japanese,Korean,Malay,Norwegian,Polish,Portuguese,Romanian,Russian,Spanish,Swedish,Thai,Turkish,Ukrainian,Vietnamese' })
  supported_languages: string;

  // --- Payment ---
  /**
   * Comma-separated payment methods shown on the storefront + accepted at
   * checkout, e.g. "stripe,vietqr". Empty falls back to the
   * PAYMENT_ENABLED_METHODS env var. Admin-controlled (no restart needed).
   */
  @Column({ type: 'varchar', length: 200, default: '' })
  payment_methods_enabled: string;

  // --- Payment: Stripe ---
  @Column({ type: 'text', default: '', transformer: secretTransformer })
  stripe_secret_key: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  stripe_webhook_secret: string;

  @Column({ type: 'varchar', length: 20, default: 'test' })
  stripe_mode: string;

  // --- Payment: PayPal ---
  @Column({ type: 'varchar', length: 500, default: '' })
  paypal_client_id: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  paypal_client_secret: string;

  @Column({ type: 'varchar', length: 20, default: 'sandbox' })
  paypal_mode: string;

  // PayPal webhook ID — used to verify inbound webhook signatures via
  // PayPal's verify-webhook-signature API (PayPal does not sign with HMAC).
  @Column({ type: 'text', default: '', transformer: secretTransformer })
  paypal_webhook_id: string;

  // PayPal billing-plan IDs (P-XXXX), one per DraftRight billing period.
  // Created once via scripts/paypal-create-plans.ts against the USD plans.
  @Column({ type: 'varchar', length: 100, default: '' })
  paypal_plan_monthly: string;

  @Column({ type: 'varchar', length: 100, default: '' })
  paypal_plan_yearly: string;

  // --- Payment: Momo ---
  @Column({ type: 'varchar', length: 500, default: '' })
  momo_partner_code: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  momo_access_key: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  momo_secret_key: string;

  @Column({ type: 'varchar', length: 20, default: 'sandbox' })
  momo_mode: string;

  // --- Payment: VietQR / Bank ---
  @Column({ type: 'varchar', length: 20, default: 'MB' })
  vietqr_bank_id: string;

  @Column({ type: 'varchar', length: 100, default: '' })
  vietqr_account_number: string;

  @Column({ type: 'varchar', length: 200, default: '' })
  vietqr_account_name: string;

  // --- Payment: Casso / SePay ---
  @Column({ type: 'text', default: '', transformer: secretTransformer })
  casso_api_key: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  sepay_api_key: string;

  @Column({ type: 'varchar', length: 20, default: 'sandbox' })
  sepay_mode: string;

  // --- Email: Resend ---
  @Column({ type: 'text', default: '', transformer: secretTransformer })
  resend_api_key: string;

  @Column({ type: 'varchar', length: 200, default: 'DraftRight <noreply@draftright.info>' })
  email_from: string;

  // --- Google OAuth ---
  @Column({ type: 'varchar', length: 500, default: '22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com' })
  google_client_id: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  google_client_secret: string;

  // --- Apple Sign In ---
  @Column({ type: 'varchar', length: 500, default: '' })
  apple_client_id: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  apple_team_id: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  apple_key_id: string;

  // --- Lemon Squeezy (MoR credit-card payments) ---
  @Column({ type: 'text', default: '', transformer: secretTransformer })
  lemonsqueezy_api_key: string;

  @Column({ type: 'varchar', length: 100, default: '' })
  lemonsqueezy_store_id: string;

  @Column({ type: 'text', default: '', transformer: secretTransformer })
  lemonsqueezy_webhook_secret: string;

  @Column({ type: 'varchar', length: 100, default: '' })
  lemonsqueezy_variant_monthly: string;

  @Column({ type: 'varchar', length: 100, default: '' })
  lemonsqueezy_variant_yearly: string;

  // --- Diagnostics ---
  // Minimum severity desktop/mobile clients should write to their local logs:
  // 'off' | 'errors' | 'warnings' | 'info'. Surfaced via GET /health so every
  // client applies it on its next poll. 'info' = full logging (default);
  // 'off' is the absolute kill-switch (silences even errors).
  @Column({ type: 'varchar', length: 20, default: 'info' })
  client_log_level: string;

  @UpdateDateColumn()
  updated_at: Date;
}
