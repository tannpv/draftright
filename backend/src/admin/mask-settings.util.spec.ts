import { maskSettings, stripMaskedSecretsFromBody } from './mask-settings.util';

describe('maskSettings', () => {
  it('masks the 11 secret columns, leaves IDs/modes alone', () => {
    const s: any = {
      stripe_secret_key: 'sk_live_abcdefghijklmnop',
      resend_api_key: 're_abcdefghijklmnop',
      apple_team_id: 'ABCDE12345',
      paypal_client_id: 'paypal-public',
      stripe_mode: 'live',
    };
    const out = maskSettings(s);
    expect(out.stripe_secret_key).toBe('sk_…mnop');
    expect(out.resend_api_key).toBe('re_…mnop');
    expect(out.apple_team_id).toBe('ABCDE12345');
    expect(out.paypal_client_id).toBe('paypal-public');
    expect(out.stripe_mode).toBe('live');
  });
});

describe('stripMaskedSecretsFromBody', () => {
  it('deletes only masked secret keys, keeps real ones + non-secrets', () => {
    const body: any = {
      stripe_secret_key: 'sk_…mnop', // masked echo → drop
      resend_api_key: 're_brandnewkey12345', // real → keep
      email_from: 'x@y.com', // non-secret → keep
    };
    stripMaskedSecretsFromBody(body);
    expect(body).not.toHaveProperty('stripe_secret_key');
    expect(body.resend_api_key).toBe('re_brandnewkey12345');
    expect(body.email_from).toBe('x@y.com');
  });
});
