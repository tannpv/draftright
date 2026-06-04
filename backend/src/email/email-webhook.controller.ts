import {
  BadRequestException,
  Controller,
  Headers,
  HttpCode,
  HttpStatus,
  Post,
  RawBodyRequest,
  Req,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { createHmac, timingSafeEqual } from 'node:crypto';
import type { Request } from 'express';
import { EmailService } from './email.service';

/**
 * Receives Resend delivery events (Svix-signed) and reflects them onto
 * email_logs.status + the suppression list. Hard bounces and spam
 * complaints stop us from emailing that address again.
 *
 * Register the endpoint URL `https://<api host>/webhooks/resend` in the
 * Resend dashboard and put its signing secret in RESEND_WEBHOOK_SECRET.
 */
@Controller('webhooks/resend')
export class EmailWebhookController {
  constructor(
    private readonly email: EmailService,
    private readonly cfg: ConfigService,
  ) {}

  @Post()
  @HttpCode(HttpStatus.OK)
  async handle(
    @Req() req: RawBodyRequest<Request>,
    @Headers() headers: Record<string, string>,
  ): Promise<{ received: boolean }> {
    const secret = this.cfg.get<string>('RESEND_WEBHOOK_SECRET');
    const raw = req.rawBody?.toString('utf8') ?? '';
    if (!secret || !this.verify(secret, headers, raw)) {
      throw new BadRequestException('Invalid webhook signature');
    }

    let event: any;
    try {
      event = JSON.parse(raw);
    } catch {
      throw new BadRequestException('Invalid payload');
    }

    const id: string | undefined = event?.data?.email_id;
    const to: string | undefined = Array.isArray(event?.data?.to) ? event.data.to[0] : event?.data?.to;

    switch (event?.type) {
      case 'email.delivered':
        if (id) await this.email.markByProviderId(id, 'delivered', null);
        break;
      case 'email.bounced': {
        const reason = event?.data?.bounce?.message || event?.data?.reason || 'bounced';
        if (id) await this.email.markByProviderId(id, 'bounced', reason);
        // Only PERMANENT/hard bounces suppress — a transient bounce
        // (full mailbox, greylisting) must not lock a real user out.
        const kind = `${event?.data?.bounce?.type ?? ''} ${event?.data?.bounce?.subType ?? ''}`.toLowerCase();
        if (to && (kind.includes('permanent') || kind.includes('hard'))) {
          await this.email.suppress(to, 'bounced');
        }
        break;
      }
      case 'email.complained':
        if (id) await this.email.markByProviderId(id, 'complained', 'Recipient marked as spam');
        if (to) await this.email.suppress(to, 'complained');
        break;
      default:
        break; // ignore sent / opened / clicked / delivery_delayed
    }
    return { received: true };
  }

  /** Verify a Svix signature (Resend's webhook signing). */
  private verify(secret: string, headers: Record<string, string>, body: string): boolean {
    const id = headers['svix-id'];
    const ts = headers['svix-timestamp'];
    const sigHeader = headers['svix-signature'];
    if (!id || !ts || !sigHeader) return false;
    const key = Buffer.from(secret.replace(/^whsec_/, ''), 'base64');
    const expected = createHmac('sha256', key).update(`${id}.${ts}.${body}`).digest('base64');
    const expBuf = Buffer.from(expected);
    // The header is a space-separated list of `v1,<base64sig>` entries.
    return sigHeader.split(' ').some((part) => {
      const sig = part.split(',')[1];
      if (!sig) return false;
      const sigBuf = Buffer.from(sig);
      return sigBuf.length === expBuf.length && timingSafeEqual(sigBuf, expBuf);
    });
  }
}
