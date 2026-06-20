// U+2026 HORIZONTAL ELLIPSIS — the single marker maskSecret emits and the write
// path keys on. Real API keys/secrets are ASCII and never contain it.
export const MASK_MARKER = '…';

// Mirror of Go internal/shared.MaskSecret. Both MUST agree byte-for-byte or the
// Go-vs-Node shadow gate fails. Empty stays empty; <16 code points reveal only
// the marker; longer reveal first3 + marker + last4. Array.from gives
// code-point iteration (not UTF-16 units) to match Go []rune.
export function maskSecret(s: string): string {
  if (!s) return '';
  const r = Array.from(s);
  if (r.length < 16) return MASK_MARKER;
  return r.slice(0, 3).join('') + MASK_MARKER + r.slice(-4).join('');
}

export function containsMaskMarker(s: unknown): boolean {
  return typeof s === 'string' && s.includes(MASK_MARKER);
}
