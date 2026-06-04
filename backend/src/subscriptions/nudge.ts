import { Entitlement, EntitlementTier } from './entitlement';

/** Which nudge strip a client should render. NONE = render nothing. */
export enum NudgeBanner {
  NONE = 'none',
  PRO_EXPIRING = 'pro_expiring',
  JUST_EXPIRED = 'just_expired',
  FREE_COUNTER = 'free_counter',
}

export interface NudgeState {
  tier: EntitlementTier;
  usageToday: number;
  dailyLimit: number;
  expiresAt: string | null;
  banner: NudgeBanner;
}

const DAY_MS = 86_400_000;
const PRO_EXPIRING_DAYS = 3;
const JUST_EXPIRED_DAYS = 7;

/**
 * Pure mapping from entitlement → banner. `lastExpiredAt` is the
 * updated_at of the user's most recent EXPIRED row (null if never).
 * `now` is injected for testability.
 */
export function deriveBanner(
  entitlement: Entitlement,
  _usageToday: number,
  lastExpiredAt: Date | null,
  now: Date,
): NudgeBanner {
  if (entitlement.tier === EntitlementTier.PRO) {
    if (entitlement.expiresAt) {
      const daysLeft = (entitlement.expiresAt.getTime() - now.getTime()) / DAY_MS;
      if (daysLeft <= PRO_EXPIRING_DAYS) return NudgeBanner.PRO_EXPIRING;
    }
    return NudgeBanner.NONE;
  }
  if (lastExpiredAt) {
    const daysSince = (now.getTime() - lastExpiredAt.getTime()) / DAY_MS;
    if (daysSince <= JUST_EXPIRED_DAYS) return NudgeBanner.JUST_EXPIRED;
  }
  return NudgeBanner.FREE_COUNTER;
}
