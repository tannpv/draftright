import 'reflect-metadata';
import { getMetadataArgsStorage } from 'typeorm';
import { AppSettings } from './entities/app-settings.entity';
import {
  maskSettings,
  stripMaskedSecretsFromBody,
  SETTINGS_SECRET_COLUMNS,
  SETTINGS_ENCRYPTED_COLUMNS,
} from './mask-settings.util';

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

/**
 * #50 guard: the columns actually encrypted at rest (those carrying a
 * ValueTransformer on the AppSettings entity) must exactly match
 * SETTINGS_ENCRYPTED_COLUMNS — the backfill script's source of truth. If a
 * secret column is added to the entity but not the constant (or vice-versa),
 * this fails instead of silently leaving a column unencrypted / unbackfilled.
 */
describe('AppSettings at-rest encryption column set', () => {
  const entityEncryptedColumns = getMetadataArgsStorage()
    .columns.filter(
      (c) => c.target === AppSettings && Boolean((c.options as any)?.transformer),
    )
    .map((c) => c.propertyName)
    .sort();

  it('matches SETTINGS_ENCRYPTED_COLUMNS exactly', () => {
    expect(entityEncryptedColumns).toEqual([...SETTINGS_ENCRYPTED_COLUMNS].sort());
  });

  it('encrypts at least every masked column (superset of the masked set)', () => {
    for (const col of SETTINGS_SECRET_COLUMNS) {
      expect(SETTINGS_ENCRYPTED_COLUMNS).toContain(col);
    }
  });
});
