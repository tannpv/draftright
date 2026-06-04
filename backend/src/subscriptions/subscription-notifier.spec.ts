import { SubscriptionNotifier } from './subscription-notifier';
import { SubscriptionEvent } from './subscription-event';

describe('SubscriptionNotifier', () => {
  function build() {
    const email: any = {
      sendSubscriptionActivated: jest.fn().mockResolvedValue(undefined),
      sendRenewalReminder: jest.fn().mockResolvedValue(undefined),
      sendSubscriptionExpired: jest.fn().mockResolvedValue(undefined),
    };
    return { notifier: new SubscriptionNotifier(email), email };
  }

  it('UPGRADED → sendSubscriptionActivated', async () => {
    const { notifier, email } = build();
    await notifier.notify(SubscriptionEvent.UPGRADED, {
      email: 'a@b.com', name: 'A', planName: 'Pro',
      expiresAt: new Date(), currency: 'USD', amountCents: 499,
    });
    expect(email.sendSubscriptionActivated).toHaveBeenCalledTimes(1);
  });

  it('a send failure is swallowed (never throws)', async () => {
    const { notifier, email } = build();
    email.sendSubscriptionExpired.mockRejectedValue(new Error('smtp down'));
    await expect(
      notifier.notify(SubscriptionEvent.EXPIRED, { email: 'a@b.com', name: 'A', planName: 'Pro' }),
    ).resolves.toBeUndefined();
  });
});
