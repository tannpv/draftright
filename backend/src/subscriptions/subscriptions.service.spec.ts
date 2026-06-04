import { SubscriptionsService } from './subscriptions.service';
import { EntitlementTier } from './entitlement';
import { SubscriptionStatus } from './entities/subscription.entity';
import { BillingPeriod } from '../plans/entities/plan.entity';

describe('SubscriptionsService.resolveEntitlement', () => {
  const freePlan = { name: 'Free', daily_limit: 10, billing_period: BillingPeriod.NONE };
  const proPlan = { name: 'Pro', daily_limit: -1, billing_period: BillingPeriod.MONTHLY };

  function build(activeRow: any) {
    const subsRepo: any = { findOne: async () => activeRow };
    const plansService: any = { findFreePlan: async () => freePlan };
    return new SubscriptionsService(subsRepo, plansService);
  }

  it('active Pro → PRO, unlimited', async () => {
    const exp = new Date('2026-07-01T00:00:00Z');
    const e = await build({ status: SubscriptionStatus.ACTIVE, plan: proPlan, expires_at: exp }).resolveEntitlement('u');
    expect(e.tier).toBe(EntitlementTier.PRO);
    expect(e.dailyLimit).toBe(-1);
    expect(e.expiresAt).toEqual(exp);
    expect(e.planName).toBe('Pro');
  });

  it('active Free → FREE, 10/day', async () => {
    const e = await build({ status: SubscriptionStatus.ACTIVE, plan: freePlan, expires_at: null }).resolveEntitlement('u');
    expect(e.tier).toBe(EntitlementTier.FREE);
    expect(e.dailyLimit).toBe(10);
    expect(e.expiresAt).toBeNull();
  });

  it('no active row (expired/cancelled/missing) → FREE, 10/day', async () => {
    const e = await build(null).resolveEntitlement('u');
    expect(e.tier).toBe(EntitlementTier.FREE);
    expect(e.dailyLimit).toBe(10);
    expect(e.status).toBeNull();
  });
});
