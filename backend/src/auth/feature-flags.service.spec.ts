import { ConfigService } from '@nestjs/config';
import { FeatureFlagsService } from './feature-flags.service';

/**
 * A minimal stand-in for ConfigService that returns the value of a
 * single env-keyed slot. Keeps the test focused on FeatureFlagsService
 * logic without spinning up @nestjs/testing.
 */
function makeService(ramp: number): FeatureFlagsService {
  // Sat-`any` because the strict<true> ConfigService overload requires
  // the full inferred env shape — we only need one key here.
  const fakeCfg: any = {
    get<T>(key: string): T {
      if (key === 'GO_BACKEND_RAMP_PERCENT') return ramp as unknown as T;
      throw new Error(`unexpected key: ${key}`);
    },
  };
  return new FeatureFlagsService(fakeCfg);
}

describe('FeatureFlagsService', () => {
  const userA = '11111111-1111-1111-1111-111111111111';
  const userB = '22222222-2222-2222-2222-222222222222';
  const userC = '33333333-3333-3333-3333-333333333333';

  it('bucket() is deterministic per user', () => {
    const svc = makeService(0);
    expect(svc.bucket(userA)).toBe(svc.bucket(userA));
    expect(svc.bucket(userB)).toBe(svc.bucket(userB));
  });

  it('bucket() returns 0-99', () => {
    const svc = makeService(0);
    for (const u of [userA, userB, userC]) {
      const b = svc.bucket(u);
      expect(b).toBeGreaterThanOrEqual(0);
      expect(b).toBeLessThan(100);
    }
  });

  it('useGoBackend false at ramp=0 (default)', () => {
    const svc = makeService(0);
    expect(svc.useGoBackend(userA)).toBe(false);
    expect(svc.useGoBackend(userB)).toBe(false);
    expect(svc.useGoBackend(userC)).toBe(false);
  });

  it('useGoBackend true for all users at ramp=100', () => {
    const svc = makeService(100);
    expect(svc.useGoBackend(userA)).toBe(true);
    expect(svc.useGoBackend(userB)).toBe(true);
    expect(svc.useGoBackend(userC)).toBe(true);
  });

  it('useGoBackend partial ramp matches bucket < rampPercent', () => {
    const svc = makeService(50);
    for (const u of [userA, userB, userC]) {
      const expected = svc.bucket(u) < 50;
      expect(svc.useGoBackend(u)).toBe(expected);
    }
  });

  it('distribution across many users is roughly uniform', () => {
    const svc = makeService(20);
    let inCohort = 0;
    const N = 1000;
    for (let i = 0; i < N; i++) {
      const u = `user-${i.toString().padStart(8, '0')}`;
      if (svc.useGoBackend(u)) inCohort++;
    }
    expect(inCohort).toBeGreaterThan(150);
    expect(inCohort).toBeLessThan(250);
  });

  it('rampPercent returns whatever ConfigService gives it', () => {
    expect(makeService(0).rampPercent()).toBe(0);
    expect(makeService(5).rampPercent()).toBe(5);
    expect(makeService(100).rampPercent()).toBe(100);
  });
});
