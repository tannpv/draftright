// Columns that must never reach an admin client. Dropped (not masked) because
// no client ever writes them back — see spec §3 (#31). Mirrored by the Go
// port: internal/user/admin_domain.go MarshalJSON omits the same keys, so both
// backends emit byte-identical JSON and the shadow gate stays green.
const USER_SECRET_COLUMNS = [
  'password_hash',
  'email_verification_code',
  'email_verification_expires',
  'password_reset_code',
  'password_reset_expires',
  'password_reset_attempts',
] as const;

export function stripUserSecrets<T extends object | null>(user: T): T {
  if (!user) return user;
  const copy: Record<string, unknown> = { ...user };
  for (const k of USER_SECRET_COLUMNS) delete copy[k];
  return copy as T;
}
