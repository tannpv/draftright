/** Shared display formatters. One source of truth for money + relative time. */

/**
 * Format a stored amount for display. Convention: USD (and other cent-based
 * currencies) are stored in minor units (499 → $4.99); VND is stored whole
 * (99000 → 99.000 ₫).
 */
export function formatCurrency(amount: number, currency: string | null = 'USD'): string {
  const c = (currency || 'USD').toUpperCase();
  if (c === 'VND') return `${amount.toLocaleString('en-US')} ₫`;
  if (c === 'USD') return `$${(amount / 100).toFixed(2)}`;
  return `${amount.toLocaleString('en-US')} ${c}`;
}

/** Compact relative time, e.g. "just now", "5m ago", "3d ago", "2y ago". */
export function timeAgo(iso: string): string {
  if (!iso) return '—';
  const diff = Date.now() - new Date(iso).getTime();
  const s = Math.floor(diff / 1000);
  if (s < 60) return 'just now';
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d}d ago`;
  const mo = Math.floor(d / 30);
  if (mo < 12) return `${mo}mo ago`;
  const y = Math.floor(d / 365);
  return `${y}y ago`;
}
