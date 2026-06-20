import { maskSecret, containsMaskMarker, MASK_MARKER } from './mask-secret.util';

describe('maskSecret (must match Go internal/shared.MaskSecret byte-for-byte)', () => {
  it.each([
    ['', ''],
    ['short', '…'],
    ['0123456789abcde', '…'], // 15 chars
    ['0123456789abcdef', '012…cdef'], // 16 chars
    ['sk-proj-abcd1234wxyz', 'sk-…wxyz'],
  ])('maskSecret(%j) = %j', (input, want) => {
    expect(maskSecret(input)).toBe(want);
  });
  it('marker is U+2026', () => expect(MASK_MARKER).toBe('…'));
  it('containsMaskMarker detects echoes', () => {
    expect(containsMaskMarker('sk-…wxyz')).toBe(true);
    expect(containsMaskMarker('sk-realkey')).toBe(false);
  });
});
