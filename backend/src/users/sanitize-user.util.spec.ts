import { stripUserSecrets } from './sanitize-user.util';

describe('stripUserSecrets', () => {
  it('removes secret columns but keeps profile columns', () => {
    const user: any = {
      id: 'u1', email: 'a@b.com', name: 'A', is_active: true, role: 'user',
      password_hash: '$2b$10$x', email_verification_code: '123456',
      email_verification_expires: new Date(), password_reset_code: '654321',
      password_reset_expires: new Date(), password_reset_attempts: 2,
    };
    const out = stripUserSecrets(user);
    for (const k of [
      'password_hash', 'email_verification_code', 'email_verification_expires',
      'password_reset_code', 'password_reset_expires', 'password_reset_attempts',
    ]) {
      expect(out).not.toHaveProperty(k);
    }
    expect(out).toMatchObject({ id: 'u1', email: 'a@b.com', name: 'A', is_active: true });
  });

  it('returns null for null input (missing user is 200 with user:null)', () => {
    expect(stripUserSecrets(null)).toBeNull();
  });
});
