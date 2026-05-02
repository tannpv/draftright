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
