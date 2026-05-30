import { Injectable } from '@nestjs/common';
import { createHash } from 'crypto';

/**
 * Server-controlled feature flag computation for per-user gradual
 * rollouts. Today's only flag is `use_go_backend`, which routes a
 * percentage of users to the Go /rewrite microservice instead of the
 * NestJS path (Task 11 of the Go rewrite plan).
 *
 * Design:
 *   - Deterministic per user. Same user_id + same ramp percent always
 *     yields the same answer, so a user never flips between backends
 *     mid-session.
 *   - Hash-then-bucket. SHA-256(user_id) → first 4 bytes → mod 100.
 *     User is "in" when bucket < rampPercent.
 *   - Ramp percent read from env var GO_BACKEND_RAMP_PERCENT (0-100).
 *     Default 0 = no users on Go path. Ops bumps this without a
 *     rebuild; clients re-poll /auth/me to pick up the change.
 *
 * Future flags follow the same shape: ramp env + bucket compare.
 */
@Injectable()
export class FeatureFlagsService {
  private static readonly RAMP_ENV = 'GO_BACKEND_RAMP_PERCENT';

  /**
   * True when the user is in the Go-backend cohort.
   * Ramp percent is clamped to [0, 100]; values outside that range
   * fall back to 0 (closed) so a typo never accidentally opens the
   * flag for everyone.
   */
  useGoBackend(userId: string): boolean {
    const ramp = this.rampPercent();
    if (ramp <= 0) return false;
    if (ramp >= 100) return true;
    return this.bucket(userId) < ramp;
  }

  /**
   * Bucket assignment: 0-99 inclusive, derived from a stable hash of
   * the user id. Public so tests can assert the distribution.
   */
  bucket(userId: string): number {
    const digest = createHash('sha256').update(userId).digest();
    // Take the first 4 bytes as a big-endian uint32, then mod 100.
    // 32 bits = 4 billion buckets before the mod, so the modulo
    // remainder distribution is uniform to within ~1 in 4e7.
    const n = digest.readUInt32BE(0);
    return n % 100;
  }

  /**
   * Current ramp percent, sanitised. Negative / NaN / >100 → 0.
   */
  rampPercent(): number {
    const raw = process.env[FeatureFlagsService.RAMP_ENV];
    const n = Number(raw);
    if (!Number.isFinite(n)) return 0;
    if (n < 0 || n > 100) return 0;
    return Math.floor(n);
  }
}
