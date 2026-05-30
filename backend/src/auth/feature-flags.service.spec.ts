import { FeatureFlagsService } from './feature-flags.service';

describe('FeatureFlagsService', () => {
  const svc = new FeatureFlagsService();
  const userA = '11111111-1111-1111-1111-111111111111';
  const userB = '22222222-2222-2222-2222-222222222222';
  const userC = '33333333-3333-3333-3333-333333333333';

  afterEach(() => {
    delete process.env.GO_BACKEND_RAMP_PERCENT;
  });

  it('bucket() is deterministic per user', () => {
    expect(svc.bucket(userA)).toBe(svc.bucket(userA));
    expect(svc.bucket(userB)).toBe(svc.bucket(userB));
  });

  it('bucket() returns 0-99', () => {
    for (const u of [userA, userB, userC]) {
      const b = svc.bucket(u);
      expect(b).toBeGreaterThanOrEqual(0);
      expect(b).toBeLessThan(100);
    }
  });

  it('useGoBackend false when ramp env unset (default 0)', () => {
    expect(svc.useGoBackend(userA)).toBe(false);
  });

  it('useGoBackend true for all users at 100% ramp', () => {
    process.env.GO_BACKEND_RAMP_PERCENT = '100';
    expect(svc.useGoBackend(userA)).toBe(true);
    expect(svc.useGoBackend(userB)).toBe(true);
    expect(svc.useGoBackend(userC)).toBe(true);
  });

  it('useGoBackend false for all users at 0% ramp', () => {
    process.env.GO_BACKEND_RAMP_PERCENT = '0';
    expect(svc.useGoBackend(userA)).toBe(false);
    expect(svc.useGoBackend(userB)).toBe(false);
    expect(svc.useGoBackend(userC)).toBe(false);
  });

  it('ramp percent rejects malformed values (defaults to 0)', () => {
    process.env.GO_BACKEND_RAMP_PERCENT = 'not-a-number';
    expect(svc.rampPercent()).toBe(0);
    process.env.GO_BACKEND_RAMP_PERCENT = '-5';
    expect(svc.rampPercent()).toBe(0);
    process.env.GO_BACKEND_RAMP_PERCENT = '150';
    expect(svc.rampPercent()).toBe(0);
  });

  it('useGoBackend partial ramp matches bucket < rampPercent', () => {
    process.env.GO_BACKEND_RAMP_PERCENT = '50';
    for (const u of [userA, userB, userC]) {
      const expected = svc.bucket(u) < 50;
      expect(svc.useGoBackend(u)).toBe(expected);
    }
  });

  it('distribution across many users is roughly uniform', () => {
    process.env.GO_BACKEND_RAMP_PERCENT = '20';
    let inCohort = 0;
    const N = 1000;
    for (let i = 0; i < N; i++) {
      const u = `user-${i.toString().padStart(8, '0')}`;
      if (svc.useGoBackend(u)) inCohort++;
    }
    // Expect ~20% ± 5% over 1000 samples.
    expect(inCohort).toBeGreaterThan(150);
    expect(inCohort).toBeLessThan(250);
  });
});
