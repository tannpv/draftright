import * as bcrypt from 'bcryptjs';

/**
 * Single source of truth for password hashing across the backend (customer
 * auth, admin auth, admin user CRUD, seed). Centralising the algorithm and
 * cost factor means a future bump is one edit instead of six.
 */
export const BCRYPT_ROUNDS = 10;

/** Hash a plaintext password for storage. */
export function hashPassword(plain: string): Promise<string> {
  return bcrypt.hash(plain, BCRYPT_ROUNDS);
}

/** Constant-time compare a plaintext password against a stored hash. */
export function verifyPassword(plain: string, hash: string): Promise<boolean> {
  return bcrypt.compare(plain, hash);
}
