import { encryptSecret, decryptSecret, isEncrypted, SECRET_PREFIX } from './secret-cipher';

// Fixed 32-byte test key (0x07 repeated), base64 — matches the Go interop test.
const TEST_KEY = Buffer.alloc(32, 7).toString('base64');

// A ciphertext produced by the GO backend (internal/platform/secretcipher) under
// TEST_KEY for plaintext "s3cret-value". Proves Node can decrypt what Go wrote —
// the two share one DB (#50).
const GO_CIPHERTEXT = 'enc:v1:Ju0s7YxvBZYKfu4rBWL47u7sc42wg9/1nxVhVccrjmbNAKWbpth6uA==';

describe('secret-cipher', () => {
  const orig = process.env.SECRETS_ENCRYPTION_KEY;
  afterEach(() => {
    if (orig === undefined) delete process.env.SECRETS_ENCRYPTION_KEY;
    else process.env.SECRETS_ENCRYPTION_KEY = orig;
  });

  it('decrypts a Go-produced ciphertext (cross-backend interop)', () => {
    process.env.SECRETS_ENCRYPTION_KEY = TEST_KEY;
    expect(decryptSecret(GO_CIPHERTEXT)).toBe('s3cret-value');
  });

  it('round-trips various plaintexts', () => {
    process.env.SECRETS_ENCRYPTION_KEY = TEST_KEY;
    for (const p of ['sk-abc123', 'unicode ✓ résumé', 'with:colons enc:v1:lookalike']) {
      const enc = encryptSecret(p);
      expect(isEncrypted(enc)).toBe(true);
      expect(enc.startsWith(SECRET_PREFIX)).toBe(true);
      expect(decryptSecret(enc)).toBe(p);
    }
  });

  it('is a no-op without a key (plaintext passthrough)', () => {
    delete process.env.SECRETS_ENCRYPTION_KEY;
    expect(encryptSecret('sk-plain')).toBe('sk-plain');
    expect(decryptSecret('sk-plain')).toBe('sk-plain');
  });

  it('never encrypts an empty string', () => {
    process.env.SECRETS_ENCRYPTION_KEY = TEST_KEY;
    expect(encryptSecret('')).toBe('');
  });

  it('is idempotent (re-encrypting ciphertext is a no-op)', () => {
    process.env.SECRETS_ENCRYPTION_KEY = TEST_KEY;
    const once = encryptSecret('sk-abc');
    expect(encryptSecret(once)).toBe(once);
  });

  it('throws decrypting ciphertext when no key is set', () => {
    delete process.env.SECRETS_ENCRYPTION_KEY;
    expect(() => decryptSecret(GO_CIPHERTEXT)).toThrow();
  });

  it('rejects a non-32-byte key', () => {
    process.env.SECRETS_ENCRYPTION_KEY = Buffer.from('short').toString('base64');
    expect(() => encryptSecret('x')).toThrow();
  });
});
