import { Entity, PrimaryGeneratedColumn, Column, UpdateDateColumn } from 'typeorm';

@Entity('app_settings')
export class AppSettings {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  // --- General ---
  @Column({ type: 'varchar', length: 20, default: 'testing' })
  environment: string;

  @Column({ type: 'int', default: 3 })
  trial_limit: number;

  /**
   * Global payment test mode. When true, the storefront/admin treat payments as
   * sandbox: providers run in their test/sandbox mode and admins can simulate a
   * successful payment (manual confirm) without a real charge. Flip off to go
   * live.
   */
  @Column({ type: 'boolean', default: false })
  payment_test_mode: boolean;

  @Column({ type: 'int', default: 15 })
  token_expiry_minutes: number;

  @Column({ type: 'int', default: 90 })
  refresh_token_expiry_days: number;

  @Column({ type: 'int', default: 3000 })
  max_input_length: number;

  @Column({ type: 'text', default: 'Arabic,Chinese (Simplified),Chinese (Traditional),Czech,Danish,Dutch,English,Finnish,French,German,Greek,Hebrew,Hindi,Hungarian,Indonesian,Italian,Japanese,Korean,Malay,Norwegian,Polish,Portuguese,Romanian,Russian,Spanish,Swedish,Thai,Turkish,Ukrainian,Vietnamese' })
  supported_languages: string;

  // --- Payment: Stripe ---
  @Column({ type: 'varchar', length: 500, default: '' })
  stripe_secret_key: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  stripe_webhook_secret: string;

  @Column({ type: 'varchar', length: 20, default: 'test' })
  stripe_mode: string;

  // --- Payment: PayPal ---
  @Column({ type: 'varchar', length: 500, default: '' })
  paypal_client_id: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  paypal_client_secret: string;

  @Column({ type: 'varchar', length: 20, default: 'sandbox' })
  paypal_mode: string;

  // --- Payment: Momo ---
  @Column({ type: 'varchar', length: 500, default: '' })
  momo_partner_code: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  momo_access_key: string;

  @Column({ type: 'varchar', length: 500, default: '' })
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
  @Column({ type: 'varchar', length: 500, default: '' })
  casso_api_key: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  sepay_api_key: string;

  @Column({ type: 'varchar', length: 20, default: 'sandbox' })
  sepay_mode: string;

  // --- Email: Resend ---
  @Column({ type: 'varchar', length: 500, default: '' })
  resend_api_key: string;

  @Column({ type: 'varchar', length: 200, default: 'DraftRight <noreply@draftright.info>' })
  email_from: string;

  // --- Google OAuth ---
  @Column({ type: 'varchar', length: 500, default: '22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com' })
  google_client_id: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  google_client_secret: string;

  // --- Apple Sign In ---
  @Column({ type: 'varchar', length: 500, default: '' })
  apple_client_id: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  apple_team_id: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  apple_key_id: string;

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
