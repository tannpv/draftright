/**
 * Return the `next` query-param destination when it is a safe same-origin
 * path, otherwise null. Only absolute paths are allowed, and
 * protocol-relative (`//host`) or backslash (`/\`) forms are rejected so a
 * crafted `next` can't open-redirect to another origin.
 */
export function safeNextPath(): string | null {
  const next = new URLSearchParams(window.location.search).get('next');
  const safe = !!next && next.startsWith('/') && !next.startsWith('//') && !next.startsWith('/\\');
  return safe ? next : null;
}

/**
 * Navigate to the safe `next` destination (see [safeNextPath]), otherwise
 * to [fallback].
 */
export function goToNext(fallback = '/account'): void {
  window.location.href = safeNextPath() ?? fallback;
}
