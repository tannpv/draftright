/**
 * Canonical kebab-case error codes the API surfaces to clients.
 *
 * Why a registry: handler code throws an HttpException whose body
 * carries a stable, machine-readable code rather than a localised
 * English message.  Clients pattern-match on the code; ops use it as
 * a metric label.  Identical codes are mirrored on the Go /rewrite
 * service (internal/http/errors.go), so a single client decoder
 * speaks to both backends.
 *
 * Adding a new code:
 *   1. Add it here with a 1-line comment explaining when it fires.
 *   2. Map it to an HTTP status in `httpStatusForCode()` below.
 *   3. (Optional) mirror in the Go service if the same condition can
 *      arise there.
 */
export const ERROR_CODES = {
  /** Generic unhandled failure — last resort. */
  internal: 'internal',
  /** Per-minute rate limit hit. */
  rateLimited: 'rate-limited',
  /** Daily quota for the user's plan exceeded. */
  quotaExceeded: 'quota-exceeded',
  /** Auth header missing / token invalid / expired. */
  invalidToken: 'invalid-token',
  /** Auth header was present + parsed, but no user row matched. */
  userNotFound: 'user-not-found',
  /** Request body fails validation. */
  invalidInput: 'invalid-input',
  /** Upstream provider returned 5xx / network failure. */
  providerFailed: 'provider-failed',
  /** No active default provider configured. */
  providerUnavailable: 'provider-unavailable',
  /** Client requested a resource that doesn't exist. */
  notFound: 'not-found',
  /** Caller has a valid identity but isn't permitted this action. */
  forbidden: 'forbidden',
  /** The resource already exists. */
  conflict: 'conflict',
} as const;

export type ErrorCode = (typeof ERROR_CODES)[keyof typeof ERROR_CODES];

/**
 * Stable mapping code → HTTP status. Centralised so handlers throw
 * by code (semantic) and the filter assigns the status (mechanical).
 *
 * Codes not listed here default to 500.
 */
export function httpStatusForCode(code: string): number {
  switch (code) {
    case ERROR_CODES.invalidInput:        return 400;
    case ERROR_CODES.invalidToken:        return 401;
    case ERROR_CODES.userNotFound:        return 401;
    case ERROR_CODES.quotaExceeded:       return 402;
    case ERROR_CODES.forbidden:           return 403;
    case ERROR_CODES.notFound:            return 404;
    case ERROR_CODES.conflict:            return 409;
    case ERROR_CODES.rateLimited:         return 429;
    case ERROR_CODES.providerFailed:      return 502;
    case ERROR_CODES.providerUnavailable: return 503;
    case ERROR_CODES.internal:            return 500;
    default:                              return 500;
  }
}
