import { RewriteCacheService } from './rewrite-cache.service';

/**
 * RewriteCacheService is now unit-testable thanks to S7 — Redis is
 * injected, not constructed. This spec proves the inject seam works
 * AND that the per-op .catch keeps cache outages transparent.
 */

/** Minimal Redis-shaped stub: only the methods the cache uses. */
function makeRedisStub(overrides: Partial<Record<string, jest.Mock>> = {}) {
  return {
    get: jest.fn().mockResolvedValue(null),
    set: jest.fn().mockResolvedValue('OK'),
    incr: jest.fn().mockResolvedValue(1),
    expire: jest.fn().mockResolvedValue(1),
    ...overrides,
  } as any;
}

describe('RewriteCacheService', () => {
  const userId = 'u-1';
  const text = 'hello world';
  const tone = 'polished';

  it('get() returns null on miss', async () => {
    const svc = new RewriteCacheService(makeRedisStub());
    expect(await svc.get(userId, text, tone)).toBeNull();
  });

  it('get() returns null + does NOT throw when Redis errors', async () => {
    const svc = new RewriteCacheService(
      makeRedisStub({ get: jest.fn().mockRejectedValue(new Error('redis down')) }),
    );
    expect(await svc.get(userId, text, tone)).toBeNull();
  });

  it('set() writes with 300 s TTL', async () => {
    const redis = makeRedisStub();
    const svc = new RewriteCacheService(redis);
    await svc.set(userId, text, tone, 'Hello there.');
    expect(redis.set).toHaveBeenCalledWith(
      expect.stringMatching(/^rewrite:u-1:[a-f0-9]{16}:polished$/),
      'Hello there.',
      'EX',
      300,
    );
  });

  it('set() swallows Redis failures', async () => {
    const svc = new RewriteCacheService(
      makeRedisStub({ set: jest.fn().mockRejectedValue(new Error('redis down')) }),
    );
    await expect(svc.set(userId, text, tone, 'x')).resolves.toBeUndefined();
  });

  it('cache keys are deterministic + scoped to user/text/tone', async () => {
    const redis = makeRedisStub();
    const svc = new RewriteCacheService(redis);
    await svc.set(userId, text, tone, 'v1');
    const key1 = redis.set.mock.calls[0][0];
    await svc.set(userId, text, tone, 'v2');
    const key2 = redis.set.mock.calls[1][0];
    expect(key1).toBe(key2);
    // Different tone → different key.
    await svc.set(userId, text, 'casual', 'v3');
    const key3 = redis.set.mock.calls[2][0];
    expect(key3).not.toBe(key1);
  });

  it('isBatchStarted() returns true when Redis stores "1"', async () => {
    const svc = new RewriteCacheService(makeRedisStub({ get: jest.fn().mockResolvedValue('1') }));
    expect(await svc.isBatchStarted(userId, text)).toBe(true);
  });

  it('isBatchStarted() returns false on missing key', async () => {
    const svc = new RewriteCacheService(makeRedisStub({ get: jest.fn().mockResolvedValue(null) }));
    expect(await svc.isBatchStarted(userId, text)).toBe(false);
  });

  it('isBatchStarted() returns false when Redis errors', async () => {
    const svc = new RewriteCacheService(
      makeRedisStub({ get: jest.fn().mockRejectedValue(new Error('redis down')) }),
    );
    expect(await svc.isBatchStarted(userId, text)).toBe(false);
  });

  it('incrementWithExpiry() returns new counter and sets expiry on first hit', async () => {
    const redis = makeRedisStub({
      incr: jest.fn().mockResolvedValue(1),
      expire: jest.fn().mockResolvedValue(1),
    });
    const svc = new RewriteCacheService(redis);
    const count = await svc.incrementWithExpiry('key', 60);
    expect(count).toBe(1);
    expect(redis.expire).toHaveBeenCalledWith('key', 60);
  });

  it('incrementWithExpiry() skips expire on subsequent hits', async () => {
    const redis = makeRedisStub({ incr: jest.fn().mockResolvedValue(2) });
    const svc = new RewriteCacheService(redis);
    const count = await svc.incrementWithExpiry('key', 60);
    expect(count).toBe(2);
    expect(redis.expire).not.toHaveBeenCalled();
  });

  it('incrementWithExpiry() returns 0 when Redis errors (open posture)', async () => {
    const svc = new RewriteCacheService(
      makeRedisStub({ incr: jest.fn().mockRejectedValue(new Error('redis down')) }),
    );
    expect(await svc.incrementWithExpiry('key', 60)).toBe(0);
  });
});
