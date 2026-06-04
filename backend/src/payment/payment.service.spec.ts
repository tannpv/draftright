import { NotFoundException } from '@nestjs/common';
import { PaymentService } from './payment.service';

/**
 * Covers today's payment-method enable/disable feature:
 *  - getEnabledMethods (settings CSV → env fallback → default; vietqr⇒bank_transfer)
 *  - createCheckout rejects a disabled method (assertEnabled path)
 */
describe('PaymentService — enabled methods', () => {
  const findOneSettings = jest.fn();
  const settingsRepo = { findOne: findOneSettings } as any;

  // Minimal stubs — only what the tested paths touch.
  const plansService = { findById: jest.fn() } as any;
  const paymentRepo = { create: jest.fn((x) => x), save: jest.fn(async (x) => x) } as any;
  const userRepo = { findOne: jest.fn(async () => ({ id: 'u1', email: 'a@b.c' })) } as any;
  const subsService = {} as any;
  const stripeStrategy = { createCheckout: jest.fn(), verifyWebhook: jest.fn() } as any;
  const vietqrStrategy = { createCheckout: jest.fn(), verifyWebhook: jest.fn() } as any;
  const emailService = { sendSubscriptionActivated: jest.fn(), sendSubscriptionExpired: jest.fn() } as any;

  let svc: PaymentService;
  const ORIGINAL_ENV = process.env.PAYMENT_ENABLED_METHODS;

  beforeEach(() => {
    jest.clearAllMocks();
    delete process.env.PAYMENT_ENABLED_METHODS;
    const lemonSqueezyStrategy = {} as any;
    // ConfigService stub: returns undefined for any env key so the
    // CSV ends up taking the DEFAULT_PAYMENT_METHOD path when no DB
    // value is set. Specific tests override findOneSettings.
    const cfg = { get: () => undefined } as any;
    const notifier = { notify: jest.fn().mockResolvedValue(undefined) } as any;
    svc = new PaymentService(
      paymentRepo, userRepo, settingsRepo, plansService, subsService, stripeStrategy, vietqrStrategy, lemonSqueezyStrategy, emailService, cfg, notifier,
    );
  });
  afterAll(() => {
    if (ORIGINAL_ENV === undefined) delete process.env.PAYMENT_ENABLED_METHODS;
    else process.env.PAYMENT_ENABLED_METHODS = ORIGINAL_ENV;
  });

  it('reads the admin CSV; vietqr implies bank_transfer', async () => {
    findOneSettings.mockResolvedValue({ payment_methods_enabled: 'vietqr' });
    expect((await svc.getEnabledMethods()).sort()).toEqual(['bank_transfer', 'vietqr']);
  });

  it('supports multiple methods from the CSV', async () => {
    findOneSettings.mockResolvedValue({ payment_methods_enabled: 'stripe,vietqr' });
    expect((await svc.getEnabledMethods()).sort()).toEqual(['bank_transfer', 'stripe', 'vietqr']);
  });

  it('falls back to the env var when the setting is empty', async () => {
    findOneSettings.mockResolvedValue({ payment_methods_enabled: '' });
    process.env.PAYMENT_ENABLED_METHODS = 'stripe';
    expect(await svc.getEnabledMethods()).toEqual(['stripe']);
  });

  it('defaults to stripe when neither setting nor env is set', async () => {
    findOneSettings.mockResolvedValue(null);
    expect(await svc.getEnabledMethods()).toEqual(['stripe']);
  });

  it('createCheckout rejects a method that is not enabled', async () => {
    findOneSettings.mockResolvedValue({ payment_methods_enabled: 'vietqr' });
    plansService.findById.mockResolvedValue({ id: 'p1', price_cents: 99000, currency: 'VND' });
    await expect(svc.createCheckout('u1', 'p1', 'stripe')).rejects.toBeInstanceOf(NotFoundException);
    expect(stripeStrategy.createCheckout).not.toHaveBeenCalled();
  });

  it('createCheckout allows an enabled method', async () => {
    findOneSettings.mockResolvedValue({ payment_methods_enabled: 'vietqr' });
    plansService.findById.mockResolvedValue({ id: 'p1', price_cents: 99000, currency: 'VND' });
    vietqrStrategy.createCheckout.mockResolvedValue({ payment: {}, qr_data: 'x' });
    await svc.createCheckout('u1', 'p1', 'vietqr');
    expect(vietqrStrategy.createCheckout).toHaveBeenCalled();
  });
});

describe('PaymentService — getCustomerPortalUrl dispatch', () => {
  const settingsRepo = { findOne: jest.fn() } as any;
  const plansService = { findById: jest.fn() } as any;
  const paymentRepo  = { create: jest.fn(), save: jest.fn() } as any;
  const userRepo     = { findOne: jest.fn() } as any;
  const subsService  = { findActiveByUserId: jest.fn() } as any;
  const emailService = {} as any;

  // Strategy stubs include the new getCustomerPortalUrl hook.
  const stripeStrategy       = { createCheckout: jest.fn(), verifyWebhook: jest.fn(), getCustomerPortalUrl: jest.fn() } as any;
  const vietqrStrategy       = { createCheckout: jest.fn(), verifyWebhook: jest.fn(), getCustomerPortalUrl: jest.fn().mockResolvedValue(null) } as any;
  const lemonSqueezyStrategy = { createCheckout: jest.fn(), verifyWebhook: jest.fn(), getCustomerPortalUrl: jest.fn() } as any;

  let svc: PaymentService;
  beforeEach(() => {
    jest.clearAllMocks();
    const cfg = { get: () => undefined } as any;
    const notifier = { notify: jest.fn().mockResolvedValue(undefined) } as any;
    svc = new PaymentService(
      paymentRepo, userRepo, settingsRepo, plansService, subsService,
      stripeStrategy, vietqrStrategy, lemonSqueezyStrategy, emailService, cfg, notifier,
    );
  });

  it('dispatches to LemonSqueezyStrategy for LS-sourced subscriptions', async () => {
    userRepo.findOne.mockResolvedValue({ id: 'u1', lemonsqueezy_customer_id: 'cus_1' });
    subsService.findActiveByUserId.mockResolvedValue({ store_type: 'lemonsqueezy' });
    lemonSqueezyStrategy.getCustomerPortalUrl.mockResolvedValue('https://ls.example/portal');
    expect(await svc.getCustomerPortalUrl('u1')).toBe('https://ls.example/portal');
    expect(lemonSqueezyStrategy.getCustomerPortalUrl).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'u1' }),
    );
    expect(stripeStrategy.getCustomerPortalUrl).not.toHaveBeenCalled();
  });

  it('dispatches to StripeStrategy for Stripe-sourced subscriptions', async () => {
    userRepo.findOne.mockResolvedValue({ id: 'u2', stripe_customer_id: 'cus_2' });
    subsService.findActiveByUserId.mockResolvedValue({ store_type: 'stripe' });
    stripeStrategy.getCustomerPortalUrl.mockResolvedValue('https://billing.stripe.com/x');
    expect(await svc.getCustomerPortalUrl('u2')).toBe('https://billing.stripe.com/x');
  });

  it('throws NotFound when no active subscription', async () => {
    userRepo.findOne.mockResolvedValue({ id: 'u3' });
    subsService.findActiveByUserId.mockResolvedValue(null);
    await expect(svc.getCustomerPortalUrl('u3')).rejects.toBeInstanceOf(NotFoundException);
  });

  it('throws NotFound when subscription is admin_granted (no portal)', async () => {
    userRepo.findOne.mockResolvedValue({ id: 'u4' });
    subsService.findActiveByUserId.mockResolvedValue({ store_type: 'admin_granted' });
    await expect(svc.getCustomerPortalUrl('u4')).rejects.toBeInstanceOf(NotFoundException);
  });

  it('throws NotFound when strategy returns null (e.g. missing customer id)', async () => {
    userRepo.findOne.mockResolvedValue({ id: 'u5', lemonsqueezy_customer_id: null });
    subsService.findActiveByUserId.mockResolvedValue({ store_type: 'lemonsqueezy' });
    lemonSqueezyStrategy.getCustomerPortalUrl.mockResolvedValue(null);
    await expect(svc.getCustomerPortalUrl('u5')).rejects.toBeInstanceOf(NotFoundException);
  });

  it('throws NotFound when user does not exist', async () => {
    userRepo.findOne.mockResolvedValue(null);
    await expect(svc.getCustomerPortalUrl('ghost')).rejects.toBeInstanceOf(NotFoundException);
  });
});
