import { Injectable, Logger, InternalServerErrorException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Resend } from 'resend';
import { AppSettings } from '../admin/entities/app-settings.entity';

@Injectable()
export class EmailService {
  private readonly logger = new Logger(EmailService.name);
  private _client: Resend | null = null;
  private _clientKey = '';

  constructor(
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
  ) {}

  /**
   * Read API key + from address from AppSettings (admin portal) and fall back
   * to env vars. Cached Resend client is invalidated when the key rotates.
   */
  private async getCredentials(): Promise<{ apiKey: string; from: string }> {
    const settings = await this.settingsRepo.findOne({ where: {} });
    return {
      apiKey: settings?.resend_api_key || process.env.RESEND_API_KEY || '',
      from: settings?.email_from || process.env.EMAIL_FROM || 'DraftRight <noreply@draftright.info>',
    };
  }

  private async getClient(): Promise<{ client: Resend; from: string } | null> {
    const { apiKey, from } = await this.getCredentials();
    if (!apiKey) return null;
    if (!this._client || this._clientKey !== apiKey) {
      this._client = new Resend(apiKey);
      this._clientKey = apiKey;
    }
    return { client: this._client, from };
  }

  /**
   * Single send path for every email: resolves the Resend client, sends, and
   * logs. `throwOnError` distinguishes user-facing flows (verify/test — surface
   * the failure) from best-effort notifications (cron/transactional — never
   * block on email). Every public method is a thin wrapper over this.
   */
  private async deliver(to: string, subject: string, html: string, label: string, throwOnError = false): Promise<void> {
    const c = await this.getClient();
    if (!c) {
      this.logger.warn(`Resend not configured — would send ${label} to ${to}`);
      if (throwOnError) throw new InternalServerErrorException('Resend API key not configured');
      return;
    }
    const result = await c.client.emails.send({ from: c.from, to, subject, html });
    if (result.error) {
      this.logger.error(`Resend error sending ${label} to ${to}: ${result.error.message}`);
      if (throwOnError) throw new InternalServerErrorException(`Email send failed: ${result.error.message}`);
      return;
    }
    this.logger.log(`${label} sent to ${to} (id=${result.data?.id})`);
  }

  async sendVerificationEmail(toEmail: string, name: string, code: string): Promise<void> {
    await this.deliver(toEmail, 'Verify your DraftRight email', this.renderVerification(name, code), 'verification email', true);
  }

  /**
   * Admin-triggered "Send test email" — verifies Resend creds + DNS are wired.
   * Throws on any error so the admin sees the failure reason in their toast.
   */
  async sendTestEmail(toEmail: string): Promise<void> {
    const html = `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">It works.</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">If you can read this, your Resend API key + sender domain are set up correctly. Renewal reminders, verification codes, and payment notices will all flow through this configuration.</p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight admin test, sent ${new Date().toISOString()}</p>
  </div>
</body></html>`;
    await this.deliver(toEmail, 'DraftRight test email', html, 'test email', true);
  }

  /**
   * Reminds the user that their subscription renews in N days.
   * Fires from SubscriptionsCron when expires_at is in the 3-day window.
   */
  async sendRenewalReminder(toEmail: string, name: string, planName: string, expiresAt: Date, currency: string, amount: number): Promise<void> {
    await this.deliver(
      toEmail,
      `DraftRight ${planName} renews on ${expiresAt.toDateString()}`,
      this.renderRenewalReminder(name, planName, expiresAt, currency, amount),
      'renewal reminder',
    );
  }

  /**
   * Notifies the user that a renewal charge failed. Stripe Smart Retries will
   * try 3 more times automatically; this email gives the user a chance to
   * update their card before the subscription is cancelled.
   */
  async sendPaymentFailed(toEmail: string, name: string, planName: string): Promise<void> {
    await this.deliver(
      toEmail,
      `Action needed: renewal payment failed for DraftRight ${planName}`,
      this.renderPaymentFailed(name, planName),
      'payment-failed',
    );
  }

  /** Confirms a successful payment — the subscription is now active. */
  async sendSubscriptionActivated(toEmail: string, name: string, planName: string, expiresAt: Date, currency: string, amount: number): Promise<void> {
    await this.deliver(
      toEmail,
      `Your DraftRight ${planName} subscription is active`,
      this.renderSubscriptionActivated(name, planName, expiresAt, currency, amount),
      'subscription-activated',
    );
  }

  /** Notifies the user that their subscription has lapsed (renew to restore). */
  async sendSubscriptionExpired(toEmail: string, name: string, planName: string): Promise<void> {
    await this.deliver(
      toEmail,
      `Your DraftRight ${planName} subscription has expired`,
      this.renderSubscriptionExpired(name, planName),
      'subscription-expired',
    );
  }

  private formatAmount(currency: string, amount: number): string {
    return currency === 'USD' ? `$${(amount / 100).toFixed(2)}` : `${amount.toLocaleString('en-US')} ${currency}`;
  }

  private renderSubscriptionActivated(name: string, planName: string, expiresAt: Date, currency: string, amount: number): string {
    const safeName = this.escapeHtml(name || 'there');
    const safePlan = this.escapeHtml(planName);
    return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">You're all set, ${safeName} 🎉</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Your payment of <strong>${this.formatAmount(currency, amount)}</strong> was received and your DraftRight <strong>${safePlan}</strong> subscription is now active.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Active until <strong>${expiresAt.toDateString()}</strong>. Enjoy unlimited rewrites across all your devices.</p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`;
  }

  private renderSubscriptionExpired(name: string, planName: string): string {
    const safeName = this.escapeHtml(name || 'there');
    const safePlan = this.escapeHtml(planName);
    return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">Your subscription has expired</h1>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi ${safeName} — your DraftRight <strong>${safePlan}</strong> subscription has ended, and your account is back on the free plan.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Renew any time to restore unlimited rewrites: <a href="https://draftright.info/account" style="color:#5b3df6;">draftright.info/account</a></p>
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`;
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
