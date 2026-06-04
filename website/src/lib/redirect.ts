/**
 * Navigate to the `next` query-param destination when it is a safe
 * same-origin path, otherwise to [fallback]. Only absolute paths are
 * allowed, and protocol-relative (`//host`) or backslash (`/\`) forms are
 * rejected so a crafted `next` can't open-redirect to another origin.
 */
export function goToNext(fallback = '/account'): void {
  const next = new URLSearchParams(window.location.search).get('next');
  const safe = !!next && next.startsWith('/') && !next.startsWith('//') && !next.startsWith('/\\');
  window.location.href = safe ? next! : fallback;
}
