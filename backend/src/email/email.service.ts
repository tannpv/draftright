import { Injectable, Logger, InternalServerErrorException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Resend } from 'resend';
import { AppSettings } from '../admin/entities/app-settings.entity';
import { EmailLog, EmailStatus } from './entities/email-log.entity';
import { EmailTemplate } from './entities/email-template.entity';
import { EmailSuppression } from './entities/email-suppression.entity';
import { EMAIL_TEMPLATE_MAP } from './email-templates';

@Injectable()
export class EmailService {
  private readonly logger = new Logger(EmailService.name);
  private _client: Resend | null = null;
  private _clientKey = '';

  constructor(
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
    @InjectRepository(EmailLog)
    private readonly emailLogRepo: Repository<EmailLog>,
    @InjectRepository(EmailTemplate)
    private readonly templateRepo: Repository<EmailTemplate>,
    @InjectRepository(EmailSuppression)
    private readonly suppressionRepo: Repository<EmailSuppression>,
  ) {}

  /** True if we've stopped emailing this address (hard bounce / complaint). */
  async isSuppressed(email: string): Promise<boolean> {
    return (await this.suppressionRepo.count({ where: { email: email.toLowerCase() } })) > 0;
  }

  /** Add an address to the suppression list (idempotent). */
  async suppress(email: string, reason: string): Promise<void> {
    await this.suppressionRepo
      .createQueryBuilder()
      .insert()
      .values({ email: email.toLowerCase(), reason })
      .orIgnore()
      .execute();
  }

  /**
   * Update the delivery status of a previously-sent email by its Resend
   * message id. Called by the delivery webhook. Best-effort.
   */
  async markByProviderId(providerId: string, status: EmailStatus, detail: string | null): Promise<void> {
    await this.emailLogRepo.update({ provider_id: providerId }, { status, ...(detail ? { error: detail } : {}) });
  }

  /** Best-effort audit row for every send attempt (never throws). */
  private async record(to: string, subject: string, type: string, status: EmailStatus, providerId: string | null, error: string | null): Promise<void> {
    try {
      await this.emailLogRepo.save(this.emailLogRepo.create({
        to_email: to, subject, email_type: type, status, provider_id: providerId, error,
      }));
    } catch (e: any) {
      this.logger.warn(`email_logs write failed: ${e?.message}`);
    }
  }

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
    if (await this.isSuppressed(to)) {
      this.logger.warn(`Suppressed recipient — skipping ${label} to ${to}`);
      await this.record(to, subject, label, 'suppressed', null, 'Recipient on suppression list (bounce/complaint)');
      if (throwOnError) throw new InternalServerErrorException('This email address can no longer receive mail.');
      return;
    }
    const c = await this.getClient();
    if (!c) {
      this.logger.warn(`Resend not configured — would send ${label} to ${to}`);
      await this.record(to, subject, label, 'skipped', null, 'Resend not configured');
      if (throwOnError) throw new InternalServerErrorException('Resend API key not configured');
      return;
    }
    const result = await c.client.emails.send({ from: c.from, to, subject, html });
    if (result.error) {
      this.logger.error(`Resend error sending ${label} to ${to}: ${result.error.message}`);
      await this.record(to, subject, label, 'failed', null, result.error.message);
      if (throwOnError) throw new InternalServerErrorException(`Email send failed: ${result.error.message}`);
      return;
    }
    this.logger.log(`${label} sent to ${to} (id=${result.data?.id})`);
    await this.record(to, subject, label, 'sent', result.data?.id ?? null, null);
  }

  async sendVerificationEmail(toEmail: string, name: string, code: string): Promise<void> {
    await this.sendTemplated('verification', toEmail, { name: name || 'there', code }, true);
  }

  async sendPasswordResetEmail(toEmail: string, name: string, code: string): Promise<void> {
    await this.sendTemplated('password-reset', toEmail, { name: name || 'there', code }, true);
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
    await this.sendTemplated('renewal-reminder', toEmail, {
      name: name || 'there', plan: planName, expires: expiresAt.toDateString(), amount: this.formatAmount(currency, amount),
    });
  }

  async sendPaymentFailed(toEmail: string, name: string, planName: string): Promise<void> {
    await this.sendTemplated('payment-failed', toEmail, { name: name || 'there', plan: planName });
  }

  /** Confirms a successful payment — the subscription is now active. */
  async sendSubscriptionActivated(toEmail: string, name: string, planName: string, expiresAt: Date, currency: string, amount: number): Promise<void> {
    await this.sendTemplated('subscription-activated', toEmail, {
      name: name || 'there', plan: planName, expires: expiresAt.toDateString(), amount: this.formatAmount(currency, amount),
    });
  }

  /** Notifies the user that their subscription has lapsed (renew to restore). */
  async sendSubscriptionExpired(toEmail: string, name: string, planName: string): Promise<void> {
    await this.sendTemplated('subscription-expired', toEmail, { name: name || 'there', plan: planName });
  }

  // --- Template rendering: DB override → built-in default → {{var}} substitution ---

  private async sendTemplated(key: string, toEmail: string, vars: Record<string, string>, throwOnError = false): Promise<void> {
    const { subject, html } = await this.renderTemplate(key, vars);
    await this.deliver(toEmail, subject, html, key, throwOnError);
  }

  /** Resolve a template (admin override or built-in default) and fill {{vars}}. */
  async renderTemplate(key: string, vars: Record<string, string>): Promise<{ subject: string; html: string }> {
    const def = EMAIL_TEMPLATE_MAP[key];
    let row: EmailTemplate | null = null;
    try {
      row = await this.templateRepo.findOne({ where: { template_key: key } });
    } catch {
      /* email_templates table optional — fall back to defaults */
    }
    const subjectTpl = row?.subject ?? def?.subject ?? '';
    const htmlTpl = row?.html ?? def?.html ?? '';
    return {
      subject: this.substitute(subjectTpl, vars, false),
      html: this.substitute(htmlTpl, vars, true),
    };
  }

  /** Replace {{token}} with vars[token]; HTML-escape values in html context. */
  private substitute(tpl: string, vars: Record<string, string>, escape: boolean): string {
    return tpl.replace(/\{\{(\w+)\}\}/g, (_m, k) => {
      const v = vars[k] ?? '';
      return escape ? this.escapeHtml(v) : v;
    });
  }

  private formatAmount(currency: string, amount: number): string {
    return currency === 'USD' ? `$${(amount / 100).toFixed(2)}` : `${amount.toLocaleString('en-US')} ${currency}`;
  }

  private escapeHtml(s: string): string {
    return s.replace(/[&<>"']/g, (c) => (
      { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!
    ));
  }
}
