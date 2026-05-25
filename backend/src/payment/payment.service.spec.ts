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

  let svc: PaymentService;
  const ORIGINAL_ENV = process.env.PAYMENT_ENABLED_METHODS;

  beforeEach(() => {
    jest.clearAllMocks();
    delete process.env.PAYMENT_ENABLED_METHODS;
    svc = new PaymentService(
      paymentRepo, userRepo, settingsRepo, plansService, subsService, stripeStrategy, vietqrStrategy,
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
