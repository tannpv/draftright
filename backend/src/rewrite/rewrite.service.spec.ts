import { HttpException, Logger } from '@nestjs/common';
import { RewriteService, PROVIDER_UNAVAILABLE_MESSAGE } from './rewrite.service';

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
