# Subscription Lifecycle Nudges + Expiry→Free Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expired users keep a usable free tier (10/day) instead of a 403 lockout, and every client surfaces a subtle, backend-driven nudge back to Pro plus lifecycle emails.

**Architecture:** A single backend entitlement resolver computes PRO/FREE from the latest subscription (no more "active row or 403"). A pure `deriveBanner` builds a `NudgeState` that `GET /subscription` returns, and a reusable Flutter strip renders it. Subscription events flow through one `SubscriptionNotifier` to email.

**Tech Stack:** NestJS 10 + TypeORM (backend, Jest), Flutter 3 (Dart, flutter_test).

**Spec:** `docs/superpowers/specs/2026-06-04-subscription-nudges-design.md`

**Rule #1 (standing):** clean, reusable, extendable — enums over literals, one source of truth, pure functions, shared widgets, serializable models for the next platform.

---

## Phase A — Backend entitlement resolver

### Task A1: `EntitlementTier` enum + `Entitlement` interface + `resolveEntitlement`

**Files:**
- Create: `backend/src/subscriptions/entitlement.ts`
- Modify: `backend/src/subscriptions/subscriptions.service.ts` (constructor + new method)
- Modify: `backend/src/subscriptions/subscriptions.module.ts` (import PlansModule)
- Test: `backend/src/subscriptions/subscriptions.service.spec.ts` (create)

- [ ] **Step 1: Create the enum + interface**

Create `backend/src/subscriptions/entitlement.ts`:

```ts
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
```

- [ ] **Step 2: Write the failing test**

Create `backend/src/subscriptions/subscriptions.service.spec.ts`:

```ts
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
```

- [ ] **Step 3: Run the test — verify it fails**

Run: `cd backend && npx jest src/subscriptions/subscriptions.service.spec.ts`
Expected: FAIL — `resolveEntitlement is not a function` and constructor arity mismatch.

- [ ] **Step 4: Implement — inject PlansService + add resolver**

In `backend/src/subscriptions/subscriptions.service.ts`, update imports + constructor:

```ts
import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Subscription, SubscriptionStatus, StoreType } from './entities/subscription.entity';
import { PlansService } from '../plans/plans.service';
import { BillingPeriod } from '../plans/entities/plan.entity';
import { Entitlement, EntitlementTier } from './entitlement';

@Injectable()
export class SubscriptionsService {
  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
    private readonly plansService: PlansService,
  ) {}
```

Add the method (place it right after `findActiveByUserId`):

```ts
  /**
   * Single source of truth for what a user may do right now. An active
   * paid plan → PRO (unlimited). Everything else — active Free, EXPIRED,
   * CANCELLED, or no row at all — resolves to the canonical Free plan so
   * lapsed users keep 10/day instead of being locked out.
   */
  async resolveEntitlement(userId: string): Promise<Entitlement> {
    const active = await this.findActiveByUserId(userId);
    if (active?.plan && active.plan.billing_period !== BillingPeriod.NONE) {
      return {
        tier: EntitlementTier.PRO,
        dailyLimit: active.plan.daily_limit,
        status: active.status,
        expiresAt: active.expires_at ?? null,
        planName: active.plan.name,
      };
    }
    const free = await this.plansService.findFreePlan();
    return {
      tier: EntitlementTier.FREE,
      dailyLimit: free.daily_limit,
      status: active?.status ?? null,
      expiresAt: null,
      planName: free.name,
    };
  }
```

- [ ] **Step 5: Wire the module dependency**

In `backend/src/subscriptions/subscriptions.module.ts`, add to `imports` array:

```ts
import { PlansModule } from '../plans/plans.module';
// ...
  imports: [
    TypeOrmModule.forFeature([Subscription]),
    ScheduleModule.forRoot(),
    UsageModule,
    PlansModule,
  ],
```

(If `PlansModule` does not export `PlansService`, add `exports: [PlansService]` to `backend/src/plans/plans.module.ts`.)

- [ ] **Step 6: Run the test — verify it passes**

Run: `cd backend && npx jest src/subscriptions/subscriptions.service.spec.ts`
Expected: PASS (3 tests).

- [ ] **Step 7: Build check**

Run: `cd backend && npx tsc --noEmit`
Expected: no output (exit 0).

- [ ] **Step 8: Commit**

```bash
git add backend/src/subscriptions/entitlement.ts backend/src/subscriptions/subscriptions.service.ts backend/src/subscriptions/subscriptions.service.spec.ts backend/src/subscriptions/subscriptions.module.ts backend/src/plans/plans.module.ts
git commit -m "feat(subscriptions): resolveEntitlement — lapsed users resolve to Free"
```

---

### Task A2: `/rewrite` uses the resolver (no more 403 lockout)

**Files:**
- Modify: `backend/src/rewrite/rewrite.service.ts` (cache branch + main branch)
- Test: `backend/src/rewrite/rewrite.service.spec.ts` (add cases)

- [ ] **Step 1: Write the failing test**

Append to `backend/src/rewrite/rewrite.service.spec.ts`:

```ts
import { EntitlementTier } from '../subscriptions/entitlement';

describe('RewriteService — entitlement gating', () => {
  function build(entitlement: any, usageToday: number, callProvider: () => Promise<any>) {
    const subscriptions: any = { resolveEntitlement: async () => entitlement };
    const usage: any = { countTodayByUser: async () => usageToday, log: async () => undefined };
    const aiProviders: any = {
      findDefault: async () => ({ model: 'm', type: 'openai' }),
      callProvider,
    };
    const cache: any = { get: async () => null, set: async () => undefined };
    const rewriteLog: any = { log: async () => undefined };
    const metrics: any = { observe: () => undefined };
    return new (require('./rewrite.service').RewriteService)(
      subscriptions, usage, aiProviders, cache, rewriteLog, metrics,
    );
  }

  const FREE = { tier: EntitlementTier.FREE, dailyLimit: 10, status: 'expired', expiresAt: null, planName: 'Free' };

  it('expired user under cap gets a rewrite (not 403)', async () => {
    const svc = build(FREE, 3, async () => ({ text: 'ok', responseTimeMs: 1 }));
    const res = await svc.rewrite('u', 'hi', 'polished');
    expect(res.rewritten_text).toBe('ok');
    expect(res.daily_limit).toBe(10);
  });

  it('free user at cap is rejected with 429', async () => {
    const svc = build(FREE, 10, async () => ({ text: 'ok', responseTimeMs: 1 }));
    await expect(svc.rewrite('u', 'hi', 'polished')).rejects.toMatchObject({ status: 429 });
  });
});
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd backend && npx jest src/rewrite/rewrite.service.spec.ts`
Expected: FAIL — `resolveEntitlement is not a function` (service still calls `findActiveByUserId`).

- [ ] **Step 3: Implement — swap both lookups to the resolver**

In `backend/src/rewrite/rewrite.service.ts`, the cache-hit branch currently reads:

```ts
      const sub = await this.subscriptionsService.findActiveByUserId(userId);
      const dailyLimit = sub?.plan?.daily_limit ?? 0;
      const usageToday = await this.usageService.countTodayByUser(userId);
```

Replace with:

```ts
      const ent = await this.subscriptionsService.resolveEntitlement(userId);
      const usageToday = await this.usageService.countTodayByUser(userId);
      const dailyLimit = ent.dailyLimit;
```

The main branch currently reads:

```ts
    // Check subscription & limits
    const sub = await this.subscriptionsService.findActiveByUserId(userId);
    if (!sub || !sub.plan) {
      this.metrics.observe({
        outcome: REWRITE_OUTCOMES.userNotFound,
        tone,
        provider: 'n/a',
        durationMs: Date.now() - startedAt,
      });
      throw new HttpException({ error: 'No active subscription', usage_today: 0, daily_limit: 0 }, 403);
    }

    const dailyLimit = sub.plan.daily_limit;
    const usageToday = await this.usageService.countTodayByUser(userId);
```

Replace with:

```ts
    // Everyone resolves to at least Free (10/day) — no lockout on lapse.
    const ent = await this.subscriptionsService.resolveEntitlement(userId);
    const dailyLimit = ent.dailyLimit;
    const usageToday = await this.usageService.countTodayByUser(userId);
```

(The `REWRITE_OUTCOMES.userNotFound` branch is deleted; the 429 cap check below it is unchanged.)

- [ ] **Step 4: Run the test — verify it passes**

Run: `cd backend && npx jest src/rewrite/rewrite.service.spec.ts`
Expected: PASS (all rewrite specs, incl. the two new ones).

- [ ] **Step 5: Build check**

Run: `cd backend && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add backend/src/rewrite/rewrite.service.ts backend/src/rewrite/rewrite.service.spec.ts
git commit -m "fix(rewrite): resolve entitlement so lapsed users get Free 10/day, not 403"
```

---

## Phase B — NudgeState payload

### Task B1: `NudgeBanner` enum + pure `deriveBanner`

**Files:**
- Create: `backend/src/subscriptions/nudge.ts`
- Test: `backend/src/subscriptions/nudge.spec.ts`

- [ ] **Step 1: Write the failing test**

Create `backend/src/subscriptions/nudge.spec.ts`:

```ts
import { deriveBanner, NudgeBanner } from './nudge';
import { EntitlementTier } from './entitlement';

describe('deriveBanner', () => {
  const NOW = new Date('2026-06-04T12:00:00Z');
  const pro = (expiresAt: Date) => ({ tier: EntitlementTier.PRO, dailyLimit: -1, status: 'active' as const, expiresAt, planName: 'Pro' });
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
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd backend && npx jest src/subscriptions/nudge.spec.ts`
Expected: FAIL — module `./nudge` not found.

- [ ] **Step 3: Implement**

Create `backend/src/subscriptions/nudge.ts`:

```ts
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
```

- [ ] **Step 4: Run the test — verify it passes**

Run: `cd backend && npx jest src/subscriptions/nudge.spec.ts`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/src/subscriptions/nudge.ts backend/src/subscriptions/nudge.spec.ts
git commit -m "feat(subscriptions): NudgeBanner enum + pure deriveBanner"
```

---

### Task B2: `buildNudgeState` + return `nudge` from `GET /subscription`

**Files:**
- Modify: `backend/src/subscriptions/subscriptions.service.ts` (add buildNudgeState + UsageService dep)
- Modify: `backend/src/subscriptions/subscriptions.controller.ts` (include nudge)
- Test: `backend/src/subscriptions/subscriptions.service.spec.ts` (add buildNudgeState cases)

- [ ] **Step 1: Write the failing test**

Append to `backend/src/subscriptions/subscriptions.service.spec.ts`:

```ts
import { NudgeBanner } from './nudge';

describe('SubscriptionsService.buildNudgeState', () => {
  const freePlan = { name: 'Free', daily_limit: 10, billing_period: BillingPeriod.NONE };

  function build(activeRow: any, usageToday: number, lastExpiredRow: any) {
    const subsRepo: any = {
      findOne: async (opts: any) =>
        opts?.where?.status === SubscriptionStatus.EXPIRED ? lastExpiredRow : activeRow,
    };
    const plansService: any = { findFreePlan: async () => freePlan };
    const usageService: any = { countTodayByUser: async () => usageToday };
    return new SubscriptionsService(subsRepo, plansService, usageService);
  }

  it('lapsed user → FREE_COUNTER with usage', async () => {
    const s = await build(null, 7, null).buildNudgeState('u');
    expect(s.tier).toBe(EntitlementTier.FREE);
    expect(s.usageToday).toBe(7);
    expect(s.dailyLimit).toBe(10);
    expect(s.banner).toBe(NudgeBanner.FREE_COUNTER);
  });
});
```

(Note: this changes the constructor to 3 args. Update the existing `build()` helpers in the earlier `describe` blocks to pass a third `usageService` arg of `{ countTodayByUser: async () => 0 }`.)

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd backend && npx jest src/subscriptions/subscriptions.service.spec.ts`
Expected: FAIL — constructor arity + `buildNudgeState` not a function.

- [ ] **Step 3: Implement — add UsageService dep + buildNudgeState**

In `backend/src/subscriptions/subscriptions.service.ts`, extend the constructor:

```ts
import { UsageService } from '../usage/usage.service';
import { NudgeState, deriveBanner } from './nudge';
// ...
  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
    private readonly plansService: PlansService,
    private readonly usageService: UsageService,
  ) {}
```

Add the method:

```ts
  /**
   * Backend-owned nudge payload — the single place any client surface
   * reads its strip state from. Pure banner logic lives in deriveBanner.
   */
  async buildNudgeState(userId: string): Promise<NudgeState> {
    const ent = await this.resolveEntitlement(userId);
    const usageToday = await this.usageService.countTodayByUser(userId);
    const lastExpired = await this.subsRepo.findOne({
      where: { user_id: userId, status: SubscriptionStatus.EXPIRED },
      order: { updated_at: 'DESC' },
    });
    const banner = deriveBanner(ent, usageToday, lastExpired?.updated_at ?? null, new Date());
    return {
      tier: ent.tier,
      usageToday,
      dailyLimit: ent.dailyLimit,
      expiresAt: ent.expiresAt ? ent.expiresAt.toISOString() : null,
      banner,
    };
  }
```

- [ ] **Step 4: Return nudge from the controller**

In `backend/src/subscriptions/subscriptions.controller.ts`, replace the body of `getMySubscription`:

```ts
  @Get()
  async getMySubscription(@Request() req: any) {
    const sub = await this.subscriptionsService.findActiveByUserId(req.user.id);
    const nudge = await this.subscriptionsService.buildNudgeState(req.user.id);
    return {
      plan: sub?.plan ? {
        name: sub.plan.name,
        daily_limit: sub.plan.daily_limit,
        billing_period: sub.plan.billing_period,
      } : null,
      status: sub?.status || null,
      expires_at: sub?.expires_at || null,
      usage_today: nudge.usageToday,
      nudge,
    };
  }
```

- [ ] **Step 5: Run tests + build**

Run: `cd backend && npx jest src/subscriptions && npx tsc --noEmit`
Expected: PASS, exit 0.

- [ ] **Step 6: Regenerate OpenAPI (the response shape changed)**

Run: `cd backend && npm run openapi:generate`
Then: `npm run openapi:check`
Expected: check PASS (no remaining drift).

- [ ] **Step 7: Commit**

```bash
git add backend/src/subscriptions/ backend/openapi.json
git commit -m "feat(subscriptions): buildNudgeState + return nudge from GET /subscription"
```

---

## Phase C — Lifecycle notifications

### Task C1: `SubscriptionEvent` enum + `SubscriptionNotifier`

**Files:**
- Create: `backend/src/subscriptions/subscription-notifier.ts`
- Modify: `backend/src/subscriptions/subscriptions.module.ts` (register provider)
- Test: `backend/src/subscriptions/subscription-notifier.spec.ts`

- [ ] **Step 1: Write the failing test**

Create `backend/src/subscriptions/subscription-notifier.spec.ts`:

```ts
import { SubscriptionNotifier } from './subscription-notifier';
import { SubscriptionEvent } from './subscription-event';

describe('SubscriptionNotifier', () => {
  function build() {
    const email: any = {
      sendSubscriptionActivated: jest.fn().mockResolvedValue(undefined),
      sendRenewalReminder: jest.fn().mockResolvedValue(undefined),
      sendSubscriptionExpired: jest.fn().mockResolvedValue(undefined),
    };
    return { notifier: new SubscriptionNotifier(email), email };
  }

  it('UPGRADED → sendSubscriptionActivated', async () => {
    const { notifier, email } = build();
    await notifier.notify(SubscriptionEvent.UPGRADED, {
      email: 'a@b.com', name: 'A', planName: 'Pro',
      expiresAt: new Date(), currency: 'USD', amountCents: 499,
    });
    expect(email.sendSubscriptionActivated).toHaveBeenCalledTimes(1);
  });

  it('a send failure is swallowed (never throws)', async () => {
    const { notifier, email } = build();
    email.sendSubscriptionExpired.mockRejectedValue(new Error('smtp down'));
    await expect(
      notifier.notify(SubscriptionEvent.EXPIRED, { email: 'a@b.com', name: 'A', planName: 'Pro' }),
    ).resolves.toBeUndefined();
  });
});
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd backend && npx jest src/subscriptions/subscription-notifier.spec.ts`
Expected: FAIL — modules not found.

- [ ] **Step 3: Implement the enum**

Create `backend/src/subscriptions/subscription-event.ts`:

```ts
export enum SubscriptionEvent {
  UPGRADED = 'upgraded',
  PLAN_CHANGED = 'plan_changed',
  EXPIRING_SOON = 'expiring_soon',
  EXPIRED = 'expired',
}

export interface SubscriptionEventPayload {
  email: string;
  name: string;
  planName: string;
  expiresAt?: Date;
  currency?: string;
  amountCents?: number;
}
```

- [ ] **Step 4: Implement the notifier**

Create `backend/src/subscriptions/subscription-notifier.ts`:

```ts
import { Injectable, Logger } from '@nestjs/common';
import { EmailService } from '../email/email.service';
import { SubscriptionEvent, SubscriptionEventPayload } from './subscription-event';

/**
 * One place that maps a subscription lifecycle event to a notification
 * channel. Today: email. Adding push later = one extra send call here,
 * no caller changes. Sends are best-effort — a failure is logged, never
 * thrown, so a batch (cron) never aborts on one bad recipient.
 */
@Injectable()
export class SubscriptionNotifier {
  private readonly logger = new Logger(SubscriptionNotifier.name);

  constructor(private readonly email: EmailService) {}

  async notify(event: SubscriptionEvent, p: SubscriptionEventPayload): Promise<void> {
    try {
      switch (event) {
        case SubscriptionEvent.UPGRADED:
        case SubscriptionEvent.PLAN_CHANGED:
          await this.email.sendSubscriptionActivated(
            p.email, p.name, p.planName, p.expiresAt ?? new Date(), p.currency ?? 'USD', p.amountCents ?? 0,
          );
          break;
        case SubscriptionEvent.EXPIRING_SOON:
          await this.email.sendRenewalReminder(
            p.email, p.name, p.planName, p.expiresAt ?? new Date(), p.currency ?? 'USD', p.amountCents ?? 0,
          );
          break;
        case SubscriptionEvent.EXPIRED:
          await this.email.sendSubscriptionExpired(p.email, p.name, p.planName);
          break;
      }
    } catch (err: any) {
      this.logger.error(`notify(${event}) failed for ${p.email}: ${err?.message}`);
    }
  }
}
```

- [ ] **Step 5: Register the provider**

In `backend/src/subscriptions/subscriptions.module.ts`, add `SubscriptionNotifier` to `providers` and `exports`:

```ts
import { SubscriptionNotifier } from './subscription-notifier';
// ...
  providers: [SubscriptionsService, SubscriptionsCron, SubscriptionNotifier],
  exports: [SubscriptionsService, SubscriptionNotifier],
```

- [ ] **Step 6: Run the test — verify it passes**

Run: `cd backend && npx jest src/subscriptions/subscription-notifier.spec.ts`
Expected: PASS (2 tests).

- [ ] **Step 7: Commit**

```bash
git add backend/src/subscriptions/subscription-event.ts backend/src/subscriptions/subscription-notifier.ts backend/src/subscriptions/subscription-notifier.spec.ts backend/src/subscriptions/subscriptions.module.ts
git commit -m "feat(subscriptions): SubscriptionEvent enum + SubscriptionNotifier"
```

---

### Task C2: Emit events from the cron + activation; reword expired email

**Files:**
- Modify: `backend/src/subscriptions/subscriptions.cron.ts` (inject notifier, route through it)
- Modify: `backend/src/subscriptions/subscriptions.service.ts` (emit on `grant`)
- Modify: `backend/src/email/templates/*` (reword expired copy) — see note
- Test: `backend/src/subscriptions/subscriptions.cron.spec.ts` (create)

- [ ] **Step 1: Write the failing test**

Create `backend/src/subscriptions/subscriptions.cron.spec.ts`:

```ts
import { SubscriptionsCron } from './subscriptions.cron';
import { SubscriptionEvent } from './subscription-event';

describe('SubscriptionsCron — emits events via notifier', () => {
  it('expired lapse routes through notifier with EXPIRED', async () => {
    const lapsed = [{ id: 's1', user: { email: 'a@b.com', name: 'A' }, plan: { name: 'Pro' }, expires_at: new Date() }];
    const subsRepo: any = {
      find: jest.fn()
        .mockResolvedValueOnce([])      // renewal reminders window
        .mockResolvedValueOnce(lapsed), // lapsed query
      createQueryBuilder: () => ({
        update: () => ({ set: () => ({ whereInIds: () => ({ execute: async () => undefined }) }) }),
      }),
    };
    const notifier: any = { notify: jest.fn().mockResolvedValue(undefined) };
    const cron = new SubscriptionsCron(subsRepo, notifier);

    await cron.runDailyMaintenance();

    expect(notifier.notify).toHaveBeenCalledWith(
      SubscriptionEvent.EXPIRED,
      expect.objectContaining({ email: 'a@b.com', planName: 'Pro' }),
    );
  });
});
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd backend && npx jest src/subscriptions/subscriptions.cron.spec.ts`
Expected: FAIL — cron constructor takes `(subsRepo, emailService)` not a notifier.

- [ ] **Step 3: Implement — cron uses the notifier**

In `backend/src/subscriptions/subscriptions.cron.ts`, replace the `EmailService` dependency with `SubscriptionNotifier` and route both paths through it. Constructor:

```ts
import { SubscriptionNotifier } from './subscription-notifier';
import { SubscriptionEvent } from './subscription-event';
// ...
  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
    private readonly notifier: SubscriptionNotifier,
  ) {}
```

In `sendRenewalReminders`, replace the `this.emailService.sendRenewalReminder(...)` call with:

```ts
        await this.notifier.notify(SubscriptionEvent.EXPIRING_SOON, {
          email: sub.user.email,
          name: sub.user.name || sub.user.email,
          planName: sub.plan.name,
          expiresAt: sub.expires_at,
          currency: (sub.plan as any).currency || 'USD',
          amountCents: sub.plan.price_cents,
        });
```

In `expireLapsedSubscriptions`, replace the `this.emailService.sendSubscriptionExpired(...)` call with:

```ts
        await this.notifier.notify(SubscriptionEvent.EXPIRED, {
          email: sub.user.email,
          name: sub.user.name || sub.user.email,
          planName: sub.plan.name,
        });
```

(The surrounding try/catch in the cron can be removed for these calls since `notifier.notify` never throws; keep the `if (!sub.user?.email ...) continue;` guards.)

- [ ] **Step 4: Emit on activation (`grant`)**

In `backend/src/subscriptions/subscriptions.service.ts`, inject `SubscriptionNotifier` into the constructor (add as 4th param) and, at the end of `grant(...)` after the new active row is saved, emit:

```ts
    // Distinguish first upgrade vs plan switch by whether a prior paid
    // sub existed for this user.
    const event = hadPriorPaidSub ? SubscriptionEvent.PLAN_CHANGED : SubscriptionEvent.UPGRADED;
    if (user?.email) {
      await this.notifier.notify(event, {
        email: user.email,
        name: user.name || user.email,
        planName: plan.name,
        expiresAt: saved.expires_at ?? undefined,
        currency: (plan as any).currency || 'USD',
        amountCents: plan.price_cents,
      });
    }
```

(Compute `hadPriorPaidSub` from the cancel-prior-active step already in `grant`: it cancels any prior ACTIVE row — capture whether that row's plan was paid before cancelling. Read `user`/`plan` from the entities `grant` already loads. Adjust variable names to match the existing method.)

- [ ] **Step 5: Reword the expired email copy**

The expired email renders from a DB template (`email_templates`, key for "subscription expired"), not a hardcoded string — `EmailService.sendSubscriptionExpired` calls `sendTemplated(key, ...)`. Update the template body via the admin template editor or seed so it reads:

> "Your Pro plan has ended. You're now on the Free plan with 10 rewrites per day. Restore Pro anytime to go unlimited."

Record the exact key by running:

Run: `cd backend && grep -n "sendSubscriptionExpired" -A2 src/email/email.service.ts`
Then update that template key's body in the DB (dev) and add the change to the seed if templates are seeded.

- [ ] **Step 6: Run tests + build**

Run: `cd backend && npx jest src/subscriptions && npx tsc --noEmit`
Expected: PASS, exit 0.

- [ ] **Step 7: Commit**

```bash
git add backend/src/subscriptions/
git commit -m "feat(subscriptions): emit lifecycle events from cron + grant; reword expired copy"
```

---

## Phase D — Flutter nudge strip

### Task D1: `NudgeState` model + parse from `/subscription`

**Files:**
- Create: `DraftRightMobile/lib/models/nudge_state.dart`
- Modify: `DraftRightMobile/lib/services/backend_client.dart` (add nudge to SubscriptionInfo)
- Test: `DraftRightMobile/test/nudge_state_test.dart`

- [ ] **Step 1: Write the failing test**

Create `DraftRightMobile/test/nudge_state_test.dart`:

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/nudge_state.dart';

void main() {
  test('parses free_counter nudge', () {
    final n = NudgeState.fromJson({
      'tier': 'free',
      'usage_today': 7,
      'daily_limit': 10,
      'expires_at': null,
      'banner': 'free_counter',
    });
    expect(n.banner, NudgeBanner.freeCounter);
    expect(n.usageToday, 7);
    expect(n.dailyLimit, 10);
  });

  test('unknown banner falls back to none (forward-compatible)', () {
    final n = NudgeState.fromJson({'banner': 'something_new'});
    expect(n.banner, NudgeBanner.none);
  });
}
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd DraftRightMobile && flutter test test/nudge_state_test.dart`
Expected: FAIL — `nudge_state.dart` not found.

- [ ] **Step 3: Implement the model**

Create `DraftRightMobile/lib/models/nudge_state.dart`:

```dart
/// Mirrors the backend NudgeBanner enum. Unknown values map to [none]
/// so older clients stay forward-compatible with new server states.
enum NudgeBanner { none, proExpiring, justExpired, freeCounter }

NudgeBanner _bannerFrom(String? raw) {
  switch (raw) {
    case 'pro_expiring':
      return NudgeBanner.proExpiring;
    case 'just_expired':
      return NudgeBanner.justExpired;
    case 'free_counter':
      return NudgeBanner.freeCounter;
    default:
      return NudgeBanner.none;
  }
}

class NudgeState {
  final String tier;
  final int usageToday;
  final int dailyLimit;
  final String? expiresAt;
  final NudgeBanner banner;

  const NudgeState({
    required this.tier,
    required this.usageToday,
    required this.dailyLimit,
    required this.expiresAt,
    required this.banner,
  });

  bool get isUnlimited => dailyLimit == -1;
  int get remaining => isUnlimited ? -1 : (dailyLimit - usageToday).clamp(0, dailyLimit);

  factory NudgeState.fromJson(Map<String, dynamic> json) {
    return NudgeState(
      tier: (json['tier'] ?? 'free').toString(),
      usageToday: (json['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: (json['daily_limit'] as num?)?.toInt() ?? 10,
      expiresAt: json['expires_at']?.toString(),
      banner: _bannerFrom(json['banner']?.toString()),
    );
  }
}
```

- [ ] **Step 4: Add nudge to SubscriptionInfo**

In `DraftRightMobile/lib/services/backend_client.dart`, add `import '../models/nudge_state.dart';` at the top, add a field + parse it in `SubscriptionInfo`:

Add field + constructor param:

```dart
  final NudgeState? nudge;
```
```dart
    this.nudge,
```

In `SubscriptionInfo.fromJson`, before `return SubscriptionInfo(`:

```dart
    final nudgeJson = json['nudge'];
    final nudge = nudgeJson is Map<String, dynamic> ? NudgeState.fromJson(nudgeJson) : null;
```

And pass `nudge: nudge,` into the returned `SubscriptionInfo(...)`.

- [ ] **Step 5: Run the test — verify it passes**

Run: `cd DraftRightMobile && flutter test test/nudge_state_test.dart`
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add DraftRightMobile/lib/models/nudge_state.dart DraftRightMobile/lib/services/backend_client.dart DraftRightMobile/test/nudge_state_test.dart
git commit -m "feat(mobile): NudgeState model + parse from /subscription"
```

---

### Task D2: `SubscriptionNudgeStrip` widget

**Files:**
- Create: `DraftRightMobile/lib/widgets/subscription_nudge_strip.dart`
- Test: `DraftRightMobile/test/subscription_nudge_strip_test.dart`

- [ ] **Step 1: Write the failing test**

Create `DraftRightMobile/test/subscription_nudge_strip_test.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/nudge_state.dart';
import 'package:draftright_mobile/widgets/subscription_nudge_strip.dart';

NudgeState _state(NudgeBanner b, {int used = 7, int limit = 10}) => NudgeState(
      tier: b == NudgeBanner.proExpiring ? 'pro' : 'free',
      usageToday: used, dailyLimit: limit, expiresAt: null, banner: b,
    );

void main() {
  Future<void> pump(WidgetTester t, NudgeState s) => t.pumpWidget(
        MaterialApp(home: Scaffold(body: SubscriptionNudgeStrip(nudge: s, onUpgrade: () {}))),
      );

  testWidgets('NONE renders nothing', (t) async {
    await pump(t, _state(NudgeBanner.none));
    expect(find.byType(SizedBox), findsWidgets);
    expect(find.textContaining('left today'), findsNothing);
  });

  testWidgets('FREE_COUNTER shows remaining count', (t) async {
    await pump(t, _state(NudgeBanner.freeCounter, used: 7, limit: 10));
    expect(find.textContaining('3 / 10 left today'), findsOneWidget);
  });

  testWidgets('JUST_EXPIRED shows restore copy', (t) async {
    await pump(t, _state(NudgeBanner.justExpired));
    expect(find.textContaining("You're on Free"), findsOneWidget);
  });
}
```

- [ ] **Step 2: Run the test — verify it fails**

Run: `cd DraftRightMobile && flutter test test/subscription_nudge_strip_test.dart`
Expected: FAIL — widget file not found.

- [ ] **Step 3: Implement the widget**

Create `DraftRightMobile/lib/widgets/subscription_nudge_strip.dart`:

```dart
import 'package:flutter/material.dart';
import '../models/nudge_state.dart';

/// Thin, app-wide nudge strip driven entirely by backend [NudgeState].
/// Renders nothing for NONE. Copy is derived here from the banner enum;
/// the same model serializes to macOS/Windows/Linux later.
class SubscriptionNudgeStrip extends StatelessWidget {
  final NudgeState? nudge;
  final VoidCallback onUpgrade;
  final VoidCallback? onDismiss;

  const SubscriptionNudgeStrip({
    super.key,
    required this.nudge,
    required this.onUpgrade,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final n = nudge;
    if (n == null || n.banner == NudgeBanner.none) {
      return const SizedBox.shrink();
    }
    final theme = Theme.of(context);
    return Material(
      color: theme.colorScheme.secondaryContainer,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        child: Row(
          children: [
            Expanded(
              child: Text(
                _text(n),
                style: theme.textTheme.bodySmall,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            TextButton(onPressed: onUpgrade, child: Text(_cta(n.banner))),
            if (onDismiss != null)
              IconButton(
                icon: const Icon(Icons.close, size: 16),
                onPressed: onDismiss,
                tooltip: 'Hide for today',
              ),
          ],
        ),
      ),
    );
  }

  String _text(NudgeState n) {
    switch (n.banner) {
      case NudgeBanner.proExpiring:
        return 'Pro ends soon';
      case NudgeBanner.justExpired:
        return "You're on Free now — 10/day";
      case NudgeBanner.freeCounter:
        return '${n.remaining} / ${n.dailyLimit} left today';
      case NudgeBanner.none:
        return '';
    }
  }

  String _cta(NudgeBanner b) {
    switch (b) {
      case NudgeBanner.proExpiring:
        return 'Renew';
      case NudgeBanner.justExpired:
        return 'Restore Pro';
      default:
        return 'Go Pro';
    }
  }
}
```

- [ ] **Step 4: Run the test — verify it passes**

Run: `cd DraftRightMobile && flutter test test/subscription_nudge_strip_test.dart`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/widgets/subscription_nudge_strip.dart DraftRightMobile/test/subscription_nudge_strip_test.dart
git commit -m "feat(mobile): SubscriptionNudgeStrip widget"
```

---

### Task D3: Mount the strip app-wide with dismiss-until-tomorrow

**Files:**
- Create: `DraftRightMobile/lib/widgets/nudge_scaffold.dart`
- Modify: `DraftRightMobile/lib/screens/playground_screen.dart` (wrap body)
- Test: manual (documented)

- [ ] **Step 1: Implement a reusable scaffold wrapper**

Create `DraftRightMobile/lib/widgets/nudge_scaffold.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/nudge_state.dart';
import '../services/backend_client.dart';
import 'subscription_nudge_strip.dart';

/// Fetches the nudge once and renders the strip above [child], honoring a
/// dismiss-until-next-local-day flag. Drop this into any screen body so
/// the nudge is app-wide without duplicating fetch/dismiss logic.
class NudgeBanner_Host extends StatefulWidget {
  final BackendClient backend;
  final Widget child;
  final VoidCallback onUpgrade;

  const NudgeBanner_Host({
    super.key,
    required this.backend,
    required this.child,
    required this.onUpgrade,
  });

  @override
  State<NudgeBanner_Host> createState() => _NudgeBannerHostState();
}

class _NudgeBannerHostState extends State<NudgeBanner_Host> {
  static const _dismissKey = 'nudge_dismissed_on';
  NudgeState? _nudge;
  bool _dismissedToday = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  String _todayKey() {
    final now = DateTime.now();
    return '${now.year}-${now.month}-${now.day}';
  }

  Future<void> _load() async {
    try {
      final info = await widget.backend.getSubscription();
      final prefs = await SharedPreferences.getInstance();
      final dismissed = prefs.getString(_dismissKey) == _todayKey();
      if (!mounted) return;
      setState(() {
        _nudge = info.nudge;
        _dismissedToday = dismissed;
      });
    } catch (_) {
      // Best-effort: never block the screen on a nudge fetch.
    }
  }

  Future<void> _dismiss() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_dismissKey, _todayKey());
    if (!mounted) return;
    setState(() => _dismissedToday = true);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        if (!_dismissedToday)
          SubscriptionNudgeStrip(
            nudge: _nudge,
            onUpgrade: widget.onUpgrade,
            onDismiss: _dismiss,
          ),
        Expanded(child: widget.child),
      ],
    );
  }
}
```

(Naming note: keep the class name idiomatic — if the codebase prefers a different convention, rename to `NudgeHost`. Verify `shared_preferences` is already a dependency — it is, per `DraftRightMobile/CLAUDE.md`.)

- [ ] **Step 2: Mount it on the main screen**

In `DraftRightMobile/lib/screens/playground_screen.dart`, wrap the `Scaffold`'s `body` with the host. The current body is `body: Padding(...)`. Change to:

```dart
      body: NudgeBanner_Host(
        backend: _backend, // use the screen's existing BackendClient instance
        onUpgrade: () => Navigator.of(context).push(
          MaterialPageRoute(builder: (_) => const SubscriptionScreen()),
        ),
        child: Padding(
          // ... existing padding/body unchanged ...
        ),
      ),
```

(Use the screen's existing backend client field name and `SubscriptionScreen` import; add the import if missing.)

- [ ] **Step 3: Analyze + run the mobile test suite**

Run: `cd DraftRightMobile && flutter analyze && flutter test`
Expected: analyze clean; all tests pass.

- [ ] **Step 4: Manual verification (documented)**

On a dev build pointed at dev: log in as a user with an EXPIRED sub →
the strip shows "N / 10 left today" → tap Go Pro routes to the
subscription screen → tap × hides it → reopen app same day stays hidden
→ (optional) change device date to next day → strip returns.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/widgets/nudge_scaffold.dart DraftRightMobile/lib/screens/playground_screen.dart
git commit -m "feat(mobile): mount nudge strip app-wide with dismiss-until-tomorrow"
```

---

## Phase E — Integration & merge

### Task E1: Full backend suite + merge to develop

- [ ] **Step 1: Run the full backend suite**

Run: `cd backend && npm test && npx tsc --noEmit && npm run openapi:check`
Expected: all green; openapi no drift.

- [ ] **Step 2: Run the full mobile suite**

Run: `cd DraftRightMobile && flutter analyze && flutter test`
Expected: clean.

- [ ] **Step 3: Merge to develop (no-ff) + push**

```bash
git checkout develop && git pull origin develop
git merge --no-ff feat/subscription-nudges-20260604 -m "Merge feat/subscription-nudges: expiry->Free + lifecycle nudges"
git push origin develop
```

- [ ] **Step 4: Watch CI**

Run: `gh run watch $(gh run list --branch develop --limit 1 --json databaseId -q '.[0].databaseId') --exit-status`
Expected: exit 0 (all 4 checks incl. S26b drift gate).

- [ ] **Step 5: Deploy to testing + verify the behavior change**

Deploy develop to the testing server. Manually expire a test sub
(`UPDATE subscriptions SET status='expired', expires_at=now()-interval '1 day' WHERE ...`),
confirm the user can still rewrite (10/day, not 403) and the strip shows
the free counter.

---

## Self-Review Notes

- Spec §3 Part 1 → Tasks A1–A2 (resolver + rewrite + /subscription via B2).
- Spec §3 Part 2 → Tasks C1–C2 (events + notifier + emit + reword).
- Spec §3 Part 3 → Tasks B1–B2 (NudgeState/deriveBanner) + D1–D3 (model, widget, mount).
- Spec §6 testing → unit tests in every task; manual verify in D3/E1.
- Soft-paywall-at-0/10 reuses the existing 429 (Task A2 keeps the cap check) — no new endpoint, per YAGNI.
- Type consistency: `EntitlementTier`, `NudgeBanner`, `SubscriptionEvent`, `NudgeState`, `deriveBanner`, `resolveEntitlement`, `buildNudgeState`, `SubscriptionNotifier.notify` are referenced identically across tasks.
```
