import { INestApplication } from '@nestjs/common';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Plan, BillingPeriod } from '../../src/plans/entities/plan.entity';

/**
 * Shared seed helpers for integration tests.
 *
 * The test containers DB is empty after `synchronize:true` creates
 * the tables; flows that depend on DB-resident rows (free plan,
 * default AI provider, …) need them seeded before exercising the
 * code path.  Each helper is idempotent — safe to call from multiple
 * `beforeAll`s in the same Jest run.
 */

/**
 * Ensures a Free plan row exists. Required by AuthService.register,
 * which calls `plansService.findFreePlan()` and assigns a free
 * subscription on success.
 *
 * Idempotent: re-running with an existing Free plan returns the
 * existing row instead of creating a duplicate.
 */
export async function seedFreePlan(app: INestApplication): Promise<Plan> {
  const repo: Repository<Plan> = app.get(getRepositoryToken(Plan));
  const existing = await repo.findOne({
    where: { billing_period: BillingPeriod.NONE, is_active: true },
  });
  if (existing) return existing;
  return repo.save(
    repo.create({
      name: 'Free',
      price_cents: 0,
      billing_period: BillingPeriod.NONE,
      daily_limit: 100,
      is_active: true,
    }),
  );
}
