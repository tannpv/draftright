/**
 * Built-in defaults for every transactional email. The admin can override a
 * template's subject/html in the DB (EmailTemplate); these are the fallback +
 * the seed for the editor. Bodies use {{variable}} placeholders substituted at
 * send time. `variables` is shown in the admin editor as the available tokens.
 */
export interface EmailTemplateDef {
  key: string;
  label: string;
  subject: string;
  html: string;
  variables: string[];
}

const shell = (title: string, body: string) => `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">${title}</h1>
    ${body}
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`;

export const EMAIL_TEMPLATES: EmailTemplateDef[] = [
  {
    key: 'verification',
    label: 'Email verification',
    subject: 'Verify your DraftRight email',
    variables: ['name', 'code'],
    html: shell('Welcome to DraftRight, {{name}}',
      `<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your verification code is:</p>
    <p style="font-size:28px;font-weight:700;letter-spacing:4px;color:#5b3df6;margin:0 0 16px;">{{code}}</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Enter it in the app to finish setting up your account. It expires in 15 minutes.</p>`),
  },
  {
    key: 'password-reset',
    label: 'Password reset',
    subject: 'Reset your DraftRight password',
    variables: ['name', 'code'],
    html: shell('Reset your password, {{name}}',
      `<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your password reset code is:</p>
    <p style="font-size:28px;font-weight:700;letter-spacing:4px;color:#5b3df6;margin:0 0 16px;">{{code}}</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Enter it on the reset page to choose a new password. It expires in 15 minutes. If you didn't request this, you can ignore this email.</p>`),
  },
  {
    key: 'subscription-activated',
    label: 'Subscription activated (payment success)',
    subject: 'Your DraftRight {{plan}} subscription is active',
    variables: ['name', 'plan', 'amount', 'expires'],
    html: shell("You're all set, {{name}} 🎉",
      `<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your payment of <strong>{{amount}}</strong> was received and your DraftRight <strong>{{plan}}</strong> subscription is now active.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Active until <strong>{{expires}}</strong>. Enjoy unlimited rewrites across all your devices.</p>`),
  },
  {
    key: 'subscription-expired',
    label: 'Subscription expired',
    subject: 'Your DraftRight {{plan}} subscription has expired',
    variables: ['name', 'plan'],
    html: shell('Your subscription has expired',
      `<p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi {{name}} — your {{plan}} plan has ended. You're now on the Free plan with 10 rewrites per day. Restore Pro anytime to go unlimited.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;"><a href="https://draftright.info/account" style="color:#5b3df6;">draftright.info/account</a></p>`),
  },
  {
    key: 'renewal-reminder',
    label: 'Renewal reminder',
    subject: 'DraftRight {{plan}} renews on {{expires}}',
    variables: ['name', 'plan', 'expires', 'amount'],
    html: shell('Heads up, {{name}}',
      `<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your DraftRight {{plan}} subscription renews on <strong>{{expires}}</strong>. We'll charge {{amount}} to your saved payment method.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">No action needed if everything looks right. To update your card or cancel, visit your account settings.</p>`),
  },
  {
    key: 'payment-failed',
    label: 'Payment failed',
    subject: 'Action needed: renewal payment failed for DraftRight {{plan}}',
    variables: ['name', 'plan'],
    html: shell("Payment didn't go through",
      `<p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi {{name}} — we tried to charge your saved card to renew your DraftRight {{plan}} subscription, but the charge failed.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">We'll automatically retry over the next few days. You can update your payment method any time to fix this faster.</p>`),
  },
];

export const EMAIL_TEMPLATE_MAP: Record<string, EmailTemplateDef> = Object.fromEntries(
  EMAIL_TEMPLATES.map((t) => [t.key, t]),
);
