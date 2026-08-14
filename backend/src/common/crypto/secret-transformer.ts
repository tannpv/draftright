import { ValueTransformer } from 'typeorm';
import { encryptSecret, decryptSecret } from './secret-cipher';

/**
 * At-rest encryption transformer for secret `text` columns (#50). `to` encrypts
 * on write, `from` decrypts on read — transparent to every consumer. Null-safe
 * and a no-op without `SECRETS_ENCRYPTION_KEY`, so legacy plaintext rows keep
 * working. Columns carrying it must be `text` (ciphertext is longer than the
 * plaintext and would overflow a narrow varchar).
 *
 * ONE shared transformer, imported by every entity that needs it (AppSettings,
 * UserContext, …) rather than copied per file — two copies are two sources of
 * truth and would drift (RULE #1). Extracted from app-settings.entity.ts when
 * UserContext (#173) needed the same behaviour for its free-text columns.
 */
export const secretTransformer: ValueTransformer = {
  to: (value?: string | null) => (value == null ? value : encryptSecret(value)),
  from: (value?: string | null) => (value == null ? value : decryptSecret(value)),
};
