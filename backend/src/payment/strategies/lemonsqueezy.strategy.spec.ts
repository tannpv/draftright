import { createHmac } from 'crypto';
import { BadRequestException } from '@nestjs/common';
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

  // Strategy now takes ConfigService for env-fallback credentials.
  // Spec only exercises DB-resident secrets, so the stub returns ''
  // for any env key — settingsRepo's signing secret is what gets used.
  const cfg = { get: () => '' } as any;

  let strat: LemonSqueezyStrategy;
  beforeEach(() => {
    jest.clearAllMocks();
    strat = new LemonSqueezyStrategy(settingsRepo, cfg);
  });

  const sign = (raw: string) => createHmac('sha256', SECRET).update(raw).digest('hex');
  const event = (name: string, opts: {
    ref?: string;
    subId?: string;
    renewsAt?: string;
    customerId?: number;
  } = {}) =>
    JSON.stringify({
      meta: {
        event_name: name,
        custom_data: opts.ref ? { reference_code: opts.ref } : {},
      },
      data: {
        id: opts.subId ?? 'sub_TEST_1',
        attributes: {
          renews_at: opts.renewsAt ?? '2026-07-01T00:00:00.000Z',
          customer_id: opts.customerId,
        },
      },
    });

  it('emits lemonsqueezy_payment_success on a valid first-charge event', async () => {
    const raw = event('subscription_payment_success', {
      ref: 'DR-PRO-ABC',
      subId: 'sub_123',
      customerId: 999,
    });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({
      type: 'lemonsqueezy_payment_success',
      reference_code: 'DR-PRO-ABC',
      lemonsqueezy_subscription_id: 'sub_123',
      lemonsqueezy_customer_id: '999',
      current_period_end: Math.floor(new Date('2026-07-01T00:00:00.000Z').getTime() / 1000),
    });
  });

  it('emits lemonsqueezy_payment_failed on payment_failed events', async () => {
    const raw = event('subscription_payment_failed', { ref: 'DR-PRO-ABC', subId: 'sub_888' });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({
      type: 'lemonsqueezy_payment_failed',
      lemonsqueezy_subscription_id: 'sub_888',
    });
  });

  it('ignores payment_failed without data.id', async () => {
    const raw = JSON.stringify({
      meta: { event_name: 'subscription_payment_failed' },
      data: {},
    });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('emits lemonsqueezy_subscription_canceled on cancel events', async () => {
    const raw = event('subscription_cancelled', { ref: 'DR-PRO-ABC', subId: 'sub_999' });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({
      type: 'lemonsqueezy_subscription_canceled',
      lemonsqueezy_subscription_id: 'sub_999',
    });
  });

  it('emits lemonsqueezy_subscription_expired on expired events', async () => {
    const raw = event('subscription_expired', { ref: 'DR-PRO-ABC', subId: 'sub_999' });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({
      type: 'lemonsqueezy_subscription_expired',
      lemonsqueezy_subscription_id: 'sub_999',
    });
  });

  it('rejects a forged/invalid signature with a 400', async () => {
    const raw = event('subscription_payment_success', { ref: 'DR-PRO-ABC', subId: 'sub_123' });
    await expect(
      strat.verifyWebhook(Buffer.from(raw), { 'x-signature': 'deadbeef' }),
    ).rejects.toBeInstanceOf(BadRequestException);
  });

  it('ignores when signing secret is unset', async () => {
    settingsRepo.findOne.mockResolvedValueOnce({ lemonsqueezy_webhook_secret: '' });
    const raw = event('subscription_payment_success', { ref: 'DR-PRO-ABC', subId: 'sub_123' });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('ignores payment_success that lacks a reference_code', async () => {
    const raw = event('subscription_payment_success', { subId: 'sub_123' });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('ignores payment_success that lacks data.id (subscription)', async () => {
    const raw = JSON.stringify({
      meta: { event_name: 'subscription_payment_success', custom_data: { reference_code: 'DR-PRO-ABC' } },
      data: {}, // no id
    });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });

  it('ignores subscription_updated (no-op event)', async () => {
    const raw = event('subscription_updated', { ref: 'DR-PRO-ABC', subId: 'sub_123' });
    const action = await strat.verifyWebhook(Buffer.from(raw), { 'x-signature': sign(raw) });
    expect(action).toEqual({ type: 'ignored' });
  });
});
