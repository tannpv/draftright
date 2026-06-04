import { deriveBanner, NudgeBanner } from './nudge';
import { EntitlementTier } from './entitlement';
import { SubscriptionStatus } from './entities/subscription.entity';

describe('deriveBanner', () => {
  const NOW = new Date('2026-06-04T12:00:00Z');
  const pro = (expiresAt: Date) => ({ tier: EntitlementTier.PRO, dailyLimit: -1, status: SubscriptionStatus.ACTIVE, expiresAt, planName: 'Pro' });
  const free = { tier: EntitlementTier.FREE, dailyLimit: 10, status: null, expiresAt: null, planName: 'Free' };

  it('active Pro, far from expiry → NONE', () => {
    expect(deriveBanner(pro(new Date('2026-07-01T00:00:00Z')), 0, null, NOW)).toBe(NudgeBanner.NONE);
  });

  it('active Pro expiring within 3 days → PRO_EXPIRING', () => {
    expect(deriveBanner(pro(new Date('2026-06-06T00:00:00Z')), 0, null, NOW)).toBe(NudgeBanner.PRO_EXPIRING);
  });

  it('free, expired within last 7 days → JUST_EXPIRED', () => {
    const lastExpired = new Date('2026-06-02T00:00:00Z');
    expect(deriveBanner(free, 1, lastExpired, NOW)).toBe(NudgeBanner.JUST_EXPIRED);
  });

  it('free, expired long ago → FREE_COUNTER', () => {
    const lastExpired = new Date('2026-05-01T00:00:00Z');
    expect(deriveBanner(free, 1, lastExpired, NOW)).toBe(NudgeBanner.FREE_COUNTER);
  });

  it('free, never had Pro → FREE_COUNTER', () => {
    expect(deriveBanner(free, 1, null, NOW)).toBe(NudgeBanner.FREE_COUNTER);
  });
});
