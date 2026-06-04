import { SubscriptionsCron } from './subscriptions.cron';
import { SubscriptionEvent } from './subscription-event';

describe('SubscriptionsCron — emits events via notifier', () => {
  it('expired lapse routes through notifier with EXPIRED', async () => {
    const lapsed = [{ id: 's1', user: { email: 'a@b.com', name: 'A' }, plan: { name: 'Pro' }, expires_at: new Date() }];
    const subsRepo: any = {
      find: jest.fn()
        .mockResolvedValueOnce([])      // renewal reminders window
        .mockResolvedValueOnce(lapsed), // lapsed query
      createQueryBuilder: () => ({
        update: () => ({ set: () => ({ whereInIds: () => ({ execute: async () => undefined }) }) }),
      }),
    };
    const notifier: any = { notify: jest.fn().mockResolvedValue(undefined) };
    const cron = new SubscriptionsCron(subsRepo, notifier);
    await cron.runDailyMaintenance();
    expect(notifier.notify).toHaveBeenCalledWith(
      SubscriptionEvent.EXPIRED,
      expect.objectContaining({ email: 'a@b.com', planName: 'Pro' }),
    );
  });
});
