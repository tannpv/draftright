import { maskSecret, containsMaskMarker } from '../common/mask-secret.util';

// The 11 secret-bearing app_settings columns. Apple team/key IDs, public
// client IDs, *_mode, vietqr_*, email_from, lemonsqueezy store/variant are NOT
// secrets. Mirrors the Go set masked in AppSettings.MarshalJSON (spec §3).
export const SETTINGS_SECRET_COLUMNS = [
  'stripe_secret_key',
  'stripe_webhook_secret',
  'paypal_client_secret',
  'momo_access_key',
  'momo_secret_key',
  'casso_api_key',
  'sepay_api_key',
  'resend_api_key',
  'google_client_secret',
  'lemonsqueezy_api_key',
  'lemonsqueezy_webhook_secret',
] as const;

// Columns encrypted AT REST (#50). Superset of the masked set: paypal_webhook_id
// is a stored PayPal identifier — encrypted alongside the real credentials, but
// not masked on read (it is not a bearer secret, so the API may echo it). This
// is the single source of truth for scripts/encrypt-secrets.ts and MUST stay in
// sync with the `secretTransformer` columns on the AppSettings entity — add a
// new secret column in both places.
export const SETTINGS_ENCRYPTED_COLUMNS = [
  ...SETTINGS_SECRET_COLUMNS,
  'paypal_webhook_id',
] as const;

// Returns a masked COPY for the response. Never mutate the entity (payment
// strategies read the real keys off the loaded settings object).
export function maskSettings<T extends Record<string, any> | null>(s: T): T {
  if (!s) return s;
  const copy: Record<string, any> = { ...s };
  for (const k of SETTINGS_SECRET_COLUMNS) {
    if (k in copy) copy[k] = maskSecret(copy[k] ?? '');
  }
  return copy as T;
}

// Mutates the inbound PATCH body in place: drops any secret key whose value is
// a masked echo, so a portal re-save can't overwrite the stored secret.
export function stripMaskedSecretsFromBody(body: Record<string, any>): void {
  if (!body) return;
  for (const k of SETTINGS_SECRET_COLUMNS) {
    if (k in body && containsMaskMarker(body[k])) delete body[k];
  }
}
