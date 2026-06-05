import { BadRequestException } from '@nestjs/common';
import { createHmac } from 'node:crypto';
import { EmailWebhookController } from './email-webhook.controller';

describe('EmailWebhookController', () => {
  const SECRET = 'whsec_' + Buffer.from('supersecretkey').toString('base64');

  function build() {
    const email: any = {
      markByProviderId: jest.fn().mockResolvedValue(undefined),
      suppress: jest.fn().mockResolvedValue(undefined),
    };
    const cfg: any = { get: () => SECRET };
    return { ctrl: new EmailWebhookController(email, cfg), email };
  }

  // Produce a valid Svix-signed request for `payload`.
  function signed(payload: object) {
    const body = JSON.stringify(payload);
    const id = 'msg_1';
    const ts = '1700000000';
    const key = Buffer.from(SECRET.replace(/^whsec_/, ''), 'base64');
    const sig = createHmac('sha256', key).update(`${id}.${ts}.${body}`).digest('base64');
    const headers = { 'svix-id': id, 'svix-timestamp': ts, 'svix-signature': `v1,${sig}` };
    const req: any = { rawBody: Buffer.from(body, 'utf8') };
    return { req, headers };
  }

  it('rejects a bad signature', async () => {
    const { ctrl } = build();
    const req: any = { rawBody: Buffer.from('{}') };
    await expect(
      ctrl.handle(req, { 'svix-id': 'x', 'svix-timestamp': '1', 'svix-signature': 'v1,bogus' }),
    ).rejects.toBeInstanceOf(BadRequestException);
  });

  it('marks delivered', async () => {
    const { ctrl, email } = build();
    const { req, headers } = signed({ type: 'email.delivered', data: { email_id: 'e1', to: ['a@b.com'] } });
    await ctrl.handle(req, headers);
    expect(email.markByProviderId).toHaveBeenCalledWith('e1', 'delivered', null);
    expect(email.suppress).not.toHaveBeenCalled();
  });

  it('suppresses on a permanent bounce', async () => {
    const { ctrl, email } = build();
    const { req, headers } = signed({
      type: 'email.bounced',
      data: { email_id: 'e2', to: ['bad@b.com'], bounce: { type: 'Permanent', message: 'no mailbox' } },
    });
    await ctrl.handle(req, headers);
    expect(email.markByProviderId).toHaveBeenCalledWith('e2', 'bounced', 'no mailbox');
    expect(email.suppress).toHaveBeenCalledWith('bad@b.com', 'bounced');
  });

  it('does NOT suppress on a transient bounce', async () => {
    const { ctrl, email } = build();
    const { req, headers } = signed({
      type: 'email.bounced',
      data: { email_id: 'e3', to: ['x@b.com'], bounce: { type: 'Transient', message: 'mailbox full' } },
    });
    await ctrl.handle(req, headers);
    expect(email.markByProviderId).toHaveBeenCalledWith('e3', 'bounced', 'mailbox full');
    expect(email.suppress).not.toHaveBeenCalled();
  });

  it('suppresses on a complaint', async () => {
    const { ctrl, email } = build();
    const { req, headers } = signed({ type: 'email.complained', data: { email_id: 'e4', to: ['c@b.com'] } });
    await ctrl.handle(req, headers);
    expect(email.suppress).toHaveBeenCalledWith('c@b.com', 'complained');
  });
});
