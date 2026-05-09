import { Injectable, Logger, InternalServerErrorException } from '@nestjs/common';
import { Resend } from 'resend';

@Injectable()
export class EmailService {
  private readonly logger = new Logger(EmailService.name);
  private readonly from = process.env.EMAIL_FROM ?? 'DraftRight <noreply@draftright.info>';
  private _client: Resend | null = null;

  private get client(): Resend {
    // Lazy-init: Resend constructor throws on empty key, so defer until first send.
    if (!this._client) {
      this._client = new Resend(process.env.RESEND_API_KEY!);
    }
    return this._client;
  }

  async sendVerificationEmail(toEmail: string, name: string, code: string): Promise<void> {
    if (!process.env.RESEND_API_KEY) {
      this.logger.warn(`RESEND_API_KEY not set — would send verification code ${code} to ${toEmail}`);
      return;
    }

    const html = this.renderVerification(name, code);
    const result = await this.client.emails.send({
      from: this.from,
      to: toEmail,
      subject: 'Verify your DraftRight email',
      html,
    });

    if (result.error) {
      this.logger.error(`Resend error sending to ${toEmail}: ${result.error.message}`);
      throw new InternalServerErrorException(`Email send failed: ${result.error.message}`);
    }
    this.logger.log(`Verification email sent to ${toEmail} (id=${result.data?.id})`);
  }

  /**
   * Reminds the user that their subscription renews in N days.
   * Fires from SubscriptionsCron when expires_at is in the 3-day window.
   */
  async sendRenewalReminder(toEmail: string, name: string, planName: string, expiresAt: Date, currency: string, amount: number): Promise<void> {
    if (!process.env.RESEND_API_KEY) {
      this.logger.warn(`RESEND_API_KEY not set — would send renewal reminder to ${toEmail}`);
      return;
    }
    const html = this.renderRenewalReminder(name, planName, expiresAt, currency, amount);
    const result = await this.client.emails.send({
      from: this.from,
      to: toEmail,
      subject: `DraftRight ${planName} renews on ${expiresAt.toDateString()}`,
      html,
    });
    if (result.error) {
      this.logger.error(`Resend error sending renewal reminder to ${toEmail}: ${result.error.message}`);
      return; // best-effort — don't block cron
    }
    this.logger.log(`Renewal reminder sent to ${toEmail} (id=${result.data?.id})`);
  }

  /**
   * Notifies the user that a renewal charge failed. Stripe Smart Retries will
   * try 3 more times automatically; this email gives the user a chance to
   * update their card before the subscription is cancelled.
   */
  async sendPaymentFailed(toEmail: string, name: string, planName: string): Promise<void> {
    if (!process.env.RESEND_API_KEY) {
      this.logger.warn(`RESEND_API_KEY not set — would send payment-failed to ${toEmail}`);
      return;
    }
    const html = this.renderPaymentFailed(name, planName);
    const result = await this.client.emails.send({
      from: this.from,
      to: toEmail,
      subject: `Action needed: renewal payment failed for DraftRight ${planName}`,
      html,
    });
    if (result.error) {
      this.logger.error(`Resend error sending payment-failed to ${toEmail}: ${result.error.message}`);
      return;
    }
    this.logger.log(`Payment-failed email sent to ${toEmail} (id=${result.data?.id})`);
  }

  private renderRenewalReminder(name: string, planName: string, expiresAt: Date, currency: string, amount: number): string {
    const safeName = this.escapeHtml(name || 'there');
    const safePlan = this.escapeHtml(planName);
    const dateStr = expiresAt.toDateString();
    const amountStr = currency === 'USD'
      ? `$${(amount / 100).toFixed(2)}`
      : `${amount.toLocaleString('en-US')} ${currency}`;
    return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">Heads up, ${safeName}</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Your DraftRight ${safePlan} subscription renews on <strong>${dateStr}</strong>. We'll charge ${amountStr} to your saved payment method.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">No action needed if everything looks right. To update your card or cancel, visit your account settings.</p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`;
  }

  private renderPaymentFailed(name: string, planName: string): string {
    const safeName = this.escapeHtml(name || 'there');
    const safePlan = this.escapeHtml(planName);
    return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">Payment didn't go through</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi ${safeName} — we tried to charge your saved card to renew your DraftRight ${safePlan} subscription, but the charge failed.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">We'll automatically retry over the next few days. You can update your payment method any time to fix this faster.</p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`;
  }

  private renderVerification(name: string, code: string): string {
    const safeName = this.escapeHtml(name);
    return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">Welcome to DraftRight, ${safeName}</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Confirm your email by entering this 6-digit code on the verification page:</p>
    <div style="font-size:32px;font-weight:600;letter-spacing:8px;text-align:center;background:#f0f0f0;padding:16px;border-radius:8px;margin:24px 0;color:#111;">${code}</div>
    <p style="color:#888;font-size:13px;margin:0;">This code expires in 15 minutes. If you didn't sign up, you can safely ignore this email.</p>
  </div>
</body></html>`;
  }

  private escapeHtml(s: string): string {
    return s.replace(/[&<>"']/g, (c) => (
      { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!
    ));
  }
}
