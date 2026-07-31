import { createCipheriv, createDecipheriv, randomBytes } from 'crypto';

/**
 * At-rest envelope encryption for secret columns (`ai_providers.api_key`,
 * `app_settings` payment/SMTP secrets). Issue #50.
 *
 * Format: `enc:v1:<base64(nonce[12] || ciphertext || gcmTag[16])>`, AES-256-GCM,
 * no AAD. The `enc:v1:` prefix makes reads transparent to legacy plaintext rows
 * (gradual migration + safe rollback) and lets the Go backend decrypt Node's
 * ciphertext and vice-versa — the two share the same DB, so the wire format and
 * key MUST match `internal/platform/secretcipher` byte-for-byte.
 *
 * Key-gated: when `SECRETS_ENCRYPTION_KEY` is unset, encrypt() is a no-op
 * (returns plaintext) and decrypt() passes plaintext through — so deploying this
 * code changes NOTHING until the key is provisioned and the migration runs.
 */

export const SECRET_PREFIX = 'enc:v1:';
const NONCE_BYTES = 12;
const TAG_BYTES = 16;
const KEY_ENV = 'SECRETS_ENCRYPTION_KEY';

/** The 32-byte AES key, or null when no key is configured. */
function loadKey(): Buffer | null {
  const raw = process.env[KEY_ENV];
  if (!raw) return null;
  const key = Buffer.from(raw, 'base64');
  if (key.length !== 32) {
    throw new Error(`${KEY_ENV} must be base64 of exactly 32 bytes (got ${key.length})`);
  }
  return key;
}

/** True when a value is already ciphertext this module produced. */
export function isEncrypted(value: string | null | undefined): boolean {
  return typeof value === 'string' && value.startsWith(SECRET_PREFIX);
}

/**
 * Encrypt a secret for storage. No-op (returns the input) when no key is set or
 * the value is empty / already encrypted — so callers can encrypt idempotently.
 */
export function encryptSecret(plain: string): string {
  const key = loadKey();
  if (!key || plain === '' || isEncrypted(plain)) return plain;
  const nonce = randomBytes(NONCE_BYTES);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  const ct = Buffer.concat([cipher.update(plain, 'utf8'), cipher.final()]);
  const tag = cipher.getAuthTag();
  return SECRET_PREFIX + Buffer.concat([nonce, ct, tag]).toString('base64');
}

/**
 * Decrypt a stored secret. Plaintext (no prefix) passes through unchanged.
 * Throws if ciphertext is present but no key is configured, or on tamper.
 */
export function decryptSecret(stored: string | null | undefined): string {
  if (stored == null) return '';
  if (!isEncrypted(stored)) return stored;
  const key = loadKey();
  if (!key) {
    throw new Error(`encrypted secret present but ${KEY_ENV} is not set`);
  }
  const raw = Buffer.from(stored.slice(SECRET_PREFIX.length), 'base64');
  if (raw.length < NONCE_BYTES + TAG_BYTES) {
    throw new Error('malformed encrypted secret');
  }
  const nonce = raw.subarray(0, NONCE_BYTES);
  const tag = raw.subarray(raw.length - TAG_BYTES);
  const ct = raw.subarray(NONCE_BYTES, raw.length - TAG_BYTES);
  const decipher = createDecipheriv('aes-256-gcm', key, nonce);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(ct), decipher.final()]).toString('utf8');
}
