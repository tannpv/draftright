import { SubscriptionStatus } from './entities/subscription.entity';

/** What a user is allowed to do right now, computed from their latest sub. */
export enum EntitlementTier {
  PRO = 'pro',
  FREE = 'free',
}

export interface Entitlement {
  tier: EntitlementTier;
  /** -1 = unlimited (Pro); 10 = Free. */
  dailyLimit: number;
  /** Raw latest-row status for display/telemetry; null if user has no row. */
  status: SubscriptionStatus | null;
  /** Pro expiry; null for Free. */
  expiresAt: Date | null;
  planName: string;
}
