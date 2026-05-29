/** Shared app constants + env-derived URLs. One place for values that were
 * previously magic literals scattered across services. */

// --- Durations (ms) ---
const MINUTE = 60 * 1000;
const DAY = 24 * 60 * MINUTE;

/** How long an email-verification code stays valid. */
export const EMAIL_CODE_TTL_MS = 15 * MINUTE;
/** How long a pending payment / QR stays valid before it expires. */
export const PAYMENT_PENDING_TTL_MS = 30 * MINUTE;
/** Fallback subscription period for LemonSqueezy when no renewal date is given. */
export const LS_PERIOD_MS = 31 * DAY;

// --- URLs (env with sane local fallback) ---
/** Public website base, e.g. for payment success/cancel redirects. */
export const websiteUrl = (): string => process.env.WEBSITE_URL || 'http://localhost:4000';
/** Backend base, e.g. for provider IPN/webhook callbacks. */
export const backendUrl = (): string => process.env.BACKEND_URL || 'http://localhost:3000';
