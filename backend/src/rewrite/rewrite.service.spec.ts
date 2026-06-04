import { HttpException, Logger } from '@nestjs/common';
import { RewriteService, PROVIDER_UNAVAILABLE_MESSAGE } from './rewrite.service';
import { EntitlementTier } from '../subscriptions/entitlement';

/**
 * Regression guard for the provider-error leak (2026-06-04): an OpenAI
 * 401 used to surface the raw upstream body — including the API key
 * prefix and request id — straight to the keyboard. callAI must log the
 * detail server-side but return a generic, secret-free body.
 */
describe('RewriteService — provider error sanitization', () => {
  const RAW_LEAK =
    'AI provider error (401): {"error":{"message":"Incorrect API key provided: cb4272f5************VPg1.","code":"invalid_api_key"}}';

  function buildService(callProvider: () => Promise<never>): RewriteService {
    const aiProviders: any = {
      findDefault: async () => ({ model: 'gpt-4o-mini', type: 'openai' }),
      callProvider,
    };
    // Only aiProviders is reached before the throw; other deps are unused.
    return new RewriteService(
      undefined as any, // subscriptions
      undefined as any, // usage
      aiProviders,
      undefined as any, // cache
      undefined as any, // rewriteLog
      undefined as any, // metrics
    );
  }

  it('returns a generic 502 body and never echoes the raw provider message', async () => {
    jest.spyOn(Logger.prototype, 'error').mockImplementation(() => undefined);
    const service = buildService(() => Promise.reject(new Error(RAW_LEAK)));

    let caught: HttpException | undefined;
    try {
      await (service as any).callAI('hello', 'polished');
    } catch (e) {
      caught = e as HttpException;
    }

    expect(caught).toBeInstanceOf(HttpException);
    expect(caught!.getStatus()).toBe(502);
    const body = caught!.getResponse() as Record<string, unknown>;
    expect(body.error).toBe(PROVIDER_UNAVAILABLE_MESSAGE);
    expect(body.code).toBe('provider-failed');
    // The crux: no provider internals reach the client.
    const serialized = JSON.stringify(body);
    expect(serialized).not.toContain('cb4272f5');
    expect(serialized).not.toContain('invalid_api_key');
    expect(serialized).not.toContain('Incorrect API key');
  });

  it('logs the raw provider detail server-side for debugging', async () => {
    const spy = jest
      .spyOn(Logger.prototype, 'error')
      .mockImplementation(() => undefined);
    const service = buildService(() => Promise.reject(new Error(RAW_LEAK)));

    await (service as any).callAI('hello', 'polished').catch(() => undefined);

    expect(spy).toHaveBeenCalled();
    const logged = spy.mock.calls.map((c) => String(c[0])).join('\n');
    expect(logged).toContain('cb4272f5');
  });
});

describe('RewriteService — entitlement gating', () => {
  function build(entitlement: any, usageToday: number, callProvider: () => Promise<any>) {
    const subscriptions: any = { resolveEntitlement: async () => entitlement };
    const usage: any = { countTodayByUser: async () => usageToday, log: async () => undefined };
    const aiProviders: any = {
      findDefault: async () => ({ model: 'm', type: 'openai' }),
      callProvider,
    };
    const cache: any = {
      get: async () => null,
      set: async () => undefined,
      isBatchStarted: async () => true, // skip batch prefetch in this path

      markBatchStarted: async () => undefined,
    };
    const rewriteLog: any = { log: async () => undefined };
    const metrics: any = { observe: () => undefined };
    return new RewriteService(
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
