/**
 * Return the `next` query-param destination when it is a safe same-origin
 * path, otherwise null.
 *
 * The browser strips tab/newline/CR from URLs before resolving them, so a
 * raw `next` like `/\t/evil.com` would pass naive prefix checks yet navigate
 * to `//evil.com`. We therefore resolve `next` against our own origin and
 * require the result to stay on it (no credentials, no host change) before
 * returning a path-only string the caller can safely navigate to.
 */
export function safeNextPath(): string | null {
  const next = new URLSearchParams(window.location.search).get('next');
  if (!next || !next.startsWith('/')) return null;
  // Characters the URL parser strips can move the host boundary — reject them.
  if (/[\t\n\r]/.test(next)) return null;
  try {
    const url = new URL(next, window.location.origin);
    if (url.origin !== window.location.origin) return null;
    if (url.username || url.password) return null;
    return url.pathname + url.search + url.hash;
  } catch {
    return null;
  }
}

/**
 * Navigate to the safe `next` destination (see [safeNextPath]), otherwise
 * to [fallback].
 */
export function goToNext(fallback = '/account'): void {
  window.location.href = safeNextPath() ?? fallback;
}
