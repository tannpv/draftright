import { createHmac } from 'crypto';
import { LemonSqueezyStrategy } from './lemonsqueezy.strategy';

/**
 * Webhook security + dispatch. The signature check is the trust boundary —
 * a forged body must never activate a subscription.
 */
describe('LemonSqueezyStrategy.verifyWebhook', () => {
  const SECRET = 'a55655bf8ed5fbeb408c50500fae5ad263417446';
  const settingsRepo = {
    findOne: jest.fn(async () => ({ lemonsqueezy_webhook_secret: SECRET })),
  } as any;

  let strat: LemonSqueezyStrategy;
  beforeEach(() => {
    jest.clearAllMocks();
    strat = new LemonSqueezyStrategy(settingsRepo);
  });

  const sign = (raw: string) => createHmac('sha256', SECRET).update(raw).digest('hex');
  const event = (name: string, ref?: string) =>
    JSON.stringify({ meta: { event_name: name, custom_data: ref ? { reference_code: ref } : {} } });

  it('activates on subscription_payment_success with valid signature', async () => {
    const raw = event('subscription_payment_success', 'DR-PRO-ABC');
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'payment_completed', reference_code: 'DR-PRO-ABC' });
  });

  it('rejects a forged/invalid signature', async () => {
    const raw = event('subscription_payment_success', 'DR-PRO-ABC');
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': 'deadbeef' });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('ignores when signing secret is unset', async () => {
    settingsRepo.findOne.mockResolvedValueOnce({ lemonsqueezy_webhook_secret: '' });
    const raw = event('subscription_payment_success', 'DR-PRO-ABC');
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('ignores payment_success that lacks a reference_code', async () => {
    const raw = event('subscription_payment_success');
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('ignores cancel/expire/update events (lapse-at-expiry for now)', async () => {
    for (const name of ['subscription_cancelled', 'subscription_expired', 'subscription_updated']) {
      const raw = event(name, 'DR-PRO-ABC');
      const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
      expect(action).toEqual({ type: 'ignored' });
    }
  });
});
