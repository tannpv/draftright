import { z } from 'zod';

/**
 * Single source of truth for every environment variable the backend
 * reads. Boot fails fast if a required var is missing or malformed —
 * no more "service started fine, then half the endpoints 500 because
 * STRIPE_SECRET_KEY was a typo".
 *
 * Categorisation:
 *   - required ALWAYS:    the service can't even boot without these.
 *   - required if FEATURE: enforced lazily by the consuming module.
 *   - optional:           sensible default OR truly opt-in.
 *
 * Adding a new env var:
 *   1. Add it here with the appropriate Zod shape + description.
 *   2. Read it via ConfigService.get<...>('VAR_NAME') in the consumer.
 *      Never reach for process.env directly outside this file.
 *   3. Standard S14 in docs/architecture-standards.md mandates this.
 */
export const envSchema = z.object({
  // --- Runtime --------------------------------------------------------
  NODE_ENV: z
    .enum(['development', 'production', 'test'])
    .default('development'),
  PORT: z.coerce.number().int().positive().default(3000),

  // --- Auth -----------------------------------------------------------
  JWT_SECRET: z
    .string()
    .min(16, 'JWT_SECRET must be at least 16 characters for HS256 safety'),
  JWT_REFRESH_SECRET: z
    .string()
    .min(16, 'JWT_REFRESH_SECRET must be at least 16 characters'),

  // --- Persistence ----------------------------------------------------
  DATABASE_URL: z
    .string()
    .url('DATABASE_URL must be a valid Postgres URL'),
  REDIS_URL: z
    .string()
    .url('REDIS_URL must be a valid Redis URL')
    .optional(),

  // --- AI provider envs (provider rows in DB are the real source) -----
  // OPENAI_API_KEY today seeds the default ai_providers row at first
  // boot; subsequent rows live in the DB. May be absent on a running
  // system after seeding.
  OPENAI_API_KEY: z.string().optional(),

  // --- Admin bootstrap ------------------------------------------------
  ADMIN_EMAIL: z.string().email().optional(),
  ADMIN_PASSWORD: z.string().optional(),

  // --- Feature flags --------------------------------------------------
  GO_BACKEND_RAMP_PERCENT: z.coerce
    .number()
    .int()
    .min(0)
    .max(100)
    .default(0),

  // --- URLs -----------------------------------------------------------
  BACKEND_URL: z.string().url().optional(),
  WEBSITE_URL: z.string().url().optional(),

  // --- Bug reports ----------------------------------------------------
  BUG_REPORTS_DIR: z.string().default('/var/lib/draftright/bug-reports'),

  // --- Email (Resend) -------------------------------------------------
  RESEND_API_KEY: z.string().optional(),
  // Display-name + email form accepted ("DraftRight <noreply@…>"), so
  // we don't constrain to strict RFC-5321 here — Resend's own parser
  // does that and surfaces clearer errors than Zod's `.email()`.
  EMAIL_FROM: z.string().optional(),
  // ADMIN_EMAIL stays strict because it's used as a Postgres lookup key.


  // --- Payment: Stripe ------------------------------------------------
  STRIPE_SECRET_KEY: z.string().optional(),
  STRIPE_WEBHOOK_SECRET: z.string().optional(),

  // --- Payment: Lemon Squeezy ----------------------------------------
  LEMONSQUEEZY_API_KEY: z.string().optional(),
  LEMONSQUEEZY_WEBHOOK_SECRET: z.string().optional(),
  LEMONSQUEEZY_STORE_ID: z.coerce.number().int().positive().optional(),
  LEMONSQUEEZY_PRO_VARIANT_ID: z.coerce.number().int().positive().optional(),

  // --- Payment: VietQR / SePay / Casso -------------------------------
  VIETQR_BANK_ID: z.string().optional(),
  VIETQR_ACCOUNT_NUMBER: z.string().optional(),
  VIETQR_ACCOUNT_NAME: z.string().optional(),
  SEPAY_API_KEY: z.string().optional(),
  CASSO_API_KEY: z.string().optional(),

  // --- Payment routing -----------------------------------------------
  // Comma-separated whitelist of enabled methods. Parsed lazily by
  // the payment module.
  PAYMENT_ENABLED_METHODS: z.string().optional(),

  // --- Operational toggles -------------------------------------------
  DISABLE_FIX_PROPOSAL_CRON: z
    .enum(['true', 'false', '1', '0'])
    .optional(),

  // --- Observability --------------------------------------------------
  // OTel OTLP-HTTP collector endpoint, e.g.
  //   OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.internal:4318
  // Empty → no SDK started, zero overhead.  Mirrors the Go service env
  // so both backends accept the same value.
  OTEL_EXPORTER_OTLP_ENDPOINT: z.string().optional(),
  // Head-based sample rate for trace export. 1.0 = every request.
  OTEL_SAMPLE_RATIO: z.coerce.number().min(0).max(1).default(1.0),
});

/**
 * Strongly-typed runtime config inferred from the schema.  Imported by
 * ConfigService consumers via `ConfigService<EnvSchema, true>` so
 * `cfg.get('JWT_SECRET')` returns `string` (not `string | undefined`)
 * and `cfg.get('GO_BACKEND_RAMP_PERCENT')` returns `number`.
 */
export type EnvSchema = z.infer<typeof envSchema>;

/**
 * Validation hook handed to @nestjs/config. Throws a single error
 * listing every failed field, so an operator fixing a misconfigured
 * deploy sees ALL problems at once instead of fix-one-redeploy-loop.
 *
 * Pass to ConfigModule.forRoot({ validate: validateEnv, … }).
 */
export function validateEnv(raw: Record<string, unknown>): EnvSchema {
  const result = envSchema.safeParse(raw);
  if (!result.success) {
    const issues = result.error.issues
      .map(i => `  - ${i.path.join('.')}: ${i.message}`)
      .join('\n');
    throw new Error(
      `Environment validation failed — refusing to start:\n${issues}`,
    );
  }
  return result.data;
}
