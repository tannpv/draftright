import { BadRequestException } from '@nestjs/common';
import { PayPalStrategy } from './paypal.strategy';

/**
 * PayPal strategy — checkout creation + webhook trust boundary.
 * All PayPal HTTP is mocked; no network. Covers PAYPAL-001..007, 017.
 */

const SANDBOX = 'https://api-m.sandbox.paypal.com';
const LIVE = 'https://api-m.paypal.com';

const okJson = (body: any) => ({ ok: true, status: 200, json: async () => body, text: async () => JSON.stringify(body) });
const noContent = () => ({ ok: true, status: 204, json: async () => ({}), text: async () => '' });
const httpErr = (status: number, body: any = {}) => ({ ok: false, status, json: async () => body, text: async () => JSON.stringify(body) });

const baseSettings = {
  paypal_client_id: 'CID',
  paypal_client_secret: 'SECRET',
  paypal_webhook_id: 'WH-123',
  paypal_plan_monthly: 'P-MONTHLY',
  paypal_plan_yearly: 'P-YEARLY',
  paypal_mode: 'sandbox',
};

function makeStrategy(settings: any = baseSettings) {
  const settingsRepo = { findOne: jest.fn(async () => settings) } as any;
  const cfg = { get: () => undefined } as any;
  return new PayPalStrategy(settingsRepo, cfg);
}

/** Install a global.fetch that dispatches on URL + method. `calls` records requests. */
function installFetch(routes: {
  token?: any;
  createSub?: any;
  verify?: any;
  getSub?: any;
}, calls: any[] = []) {
  const fetchMock = jest.fn(async (url: string, init?: any) => {
    const u = String(url);
    const method = (init?.method || 'GET').toUpperCase();
    // Token request is form-encoded, not JSON — only parse JSON bodies.
    let body: any;
    if (typeof init?.body === 'string' && init.body.trim().startsWith('{')) {
      try { body = JSON.parse(init.body); } catch { body = init.body; }
    } else {
      body = init?.body;
    }
    calls.push({ url: u, method, body });
    if (u.endsWith('/v1/oauth2/token')) return routes.token ?? okJson({ access_token: 'TOKEN', expires_in: 3600 });
    if (u.endsWith('/v1/billing/subscriptions') && method === 'POST') return routes.createSub ?? okJson({ id: 'I-SUB', links: [{ rel: 'approve', href: 'https://paypal.com/approve/xyz' }] });
    if (u.endsWith('/v1/notifications/verify-webhook-signature')) return routes.verify ?? okJson({ verification_status: 'SUCCESS' });
    if (u.includes('/v1/billing/subscriptions/') && method === 'GET') return routes.getSub ?? okJson({ billing_info: { next_billing_time: '2026-09-01T00:00:00Z' } });
    if (u.includes('/cancel')) return routes.token /* reuse noContent via override */ ?? noContent();
    return httpErr(404);
  });
  (global as any).fetch = fetchMock;
  return fetchMock;
}

afterEach(() => { jest.restoreAllMocks(); });

describe('PayPalStrategy.createCheckout', () => {
  it('monthly plan → posts monthly plan_id + custom_id, returns approve URL (PAYPAL-002)', async () => {
    const calls: any[] = [];
    installFetch({}, calls);
    const strat = makeStrategy();
    const payment: any = { reference_code: 'DR-PRO-1', plan: { billing_period: 'monthly' } };
    const res = await strat.createCheckout(payment, { user_email: 'a@b.c' });
    expect(res.redirect_url).toBe('https://paypal.com/approve/xyz');
    const sub = calls.find((c) => c.url.endsWith('/v1/billing/subscriptions'));
    expect(sub.body.plan_id).toBe('P-MONTHLY');
    expect(sub.body.custom_id).toBe('DR-PRO-1');
    // token fetched against sandbox host
    expect(calls[0].url.startsWith(SANDBOX)).toBe(true);
  });

  it('yearly plan → posts yearly plan_id (PAYPAL-003)', async () => {
    const calls: any[] = [];
    installFetch({}, calls);
    const strat = makeStrategy();
    const payment: any = { reference_code: 'DR-PRO-2', plan: { billing_period: 'yearly' } };
    await strat.createCheckout(payment, {});
    const sub = calls.find((c) => c.url.endsWith('/v1/billing/subscriptions'));
    expect(sub.body.plan_id).toBe('P-YEARLY');
  });

  it('live mode → token fetched against live host (PAYPAL-017)', async () => {
    const calls: any[] = [];
    installFetch({}, calls);
    const strat = makeStrategy({ ...baseSettings, paypal_mode: 'live' });
    await strat.createCheckout({ reference_code: 'r', plan: { billing_period: 'monthly' } } as any, {});
    expect(calls[0].url.startsWith(LIVE)).toBe(true);
  });

  it('missing plan id → clear config error, no subscription created (PAYPAL-004)', async () => {
    const calls: any[] = [];
    installFetch({}, calls);
    const strat = makeStrategy({ ...baseSettings, paypal_plan_monthly: '' });
    await expect(
      strat.createCheckout({ reference_code: 'r', plan: { billing_period: 'monthly' } } as any, {}),
    ).rejects.toThrow(/PayPal is temporarily unavailable/);
    expect(calls.some((c) => c.url.endsWith('/v1/billing/subscriptions'))).toBe(false);
  });

  it('missing credentials → not-configured error (PAYPAL-005)', async () => {
    installFetch({});
    const strat = makeStrategy({ ...baseSettings, paypal_client_id: '' });
    await expect(
      strat.createCheckout({ reference_code: 'r', plan: { billing_period: 'monthly' } } as any, {}),
    ).rejects.toThrow(/PayPal is temporarily unavailable/);
  });
});

describe('PayPalStrategy.verifyWebhook', () => {
  const headers = {
    'paypal-auth-algo': 'SHA256withRSA',
    'paypal-cert-url': 'https://api.paypal.com/cert',
    'paypal-transmission-id': 't-1',
    'paypal-transmission-sig': 'sig',
    'paypal-transmission-time': '2026-07-01T00:00:00Z',
  };

  it('ACTIVATED with SUCCESS verify → paypal_payment_success (PAYPAL-008)', async () => {
    installFetch({ verify: okJson({ verification_status: 'SUCCESS' }) });
    const strat = makeStrategy();
    const event = {
      event_type: 'BILLING.SUBSCRIPTION.ACTIVATED',
      resource: { id: 'I-SUB-9', custom_id: 'DR-PRO-9', billing_info: { next_billing_time: '2026-08-01T00:00:00Z' } },
    };
    const action = await strat.verifyWebhook(Buffer.from(JSON.stringify(event)), headers);
    expect(action).toEqual({
      type: 'paypal_payment_success',
      reference_code: 'DR-PRO-9',
      paypal_subscription_id: 'I-SUB-9',
      current_period_end: Math.floor(new Date('2026-08-01T00:00:00Z').getTime() / 1000),
    });
  });

  it('CANCELLED → paypal_subscription_canceled (PAYPAL-011)', async () => {
    installFetch({ verify: okJson({ verification_status: 'SUCCESS' }) });
    const strat = makeStrategy();
    const event = { event_type: 'BILLING.SUBSCRIPTION.CANCELLED', resource: { id: 'I-SUB-C' } };
    const action = await strat.verifyWebhook(Buffer.from(JSON.stringify(event)), headers);
    expect(action).toEqual({ type: 'paypal_subscription_canceled', paypal_subscription_id: 'I-SUB-C' });
  });

  it('FAILURE verification → BadRequestException, nothing accepted (PAYPAL-006)', async () => {
    installFetch({ verify: okJson({ verification_status: 'FAILURE' }) });
    const strat = makeStrategy();
    const event = { event_type: 'BILLING.SUBSCRIPTION.ACTIVATED', resource: { id: 'x', custom_id: 'y' } };
    await expect(strat.verifyWebhook(Buffer.from(JSON.stringify(event)), headers)).rejects.toBeInstanceOf(BadRequestException);
  });

  it('unset webhook ID → ignored, no verify call (PAYPAL-007)', async () => {
    const calls: any[] = [];
    installFetch({}, calls);
    const strat = makeStrategy({ ...baseSettings, paypal_webhook_id: '' });
    const action = await strat.verifyWebhook(Buffer.from(JSON.stringify({ event_type: 'X' })), headers);
    expect(action).toEqual({ type: 'ignored' });
    expect(calls.some((c) => c.url.includes('verify-webhook-signature'))).toBe(false);
  });
});
