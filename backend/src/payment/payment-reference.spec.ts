import {
  PAYMENT_REF_PREFIX,
  generatePaymentReference,
  extractPaymentReference,
} from './payment-reference';

describe('payment-reference', () => {
  it('generates a DR-PRO- prefixed reference', () => {
    const ref = generatePaymentReference();
    expect(ref.startsWith(PAYMENT_REF_PREFIX)).toBe(true);
    expect(ref).toMatch(/^DR-PRO-[A-F0-9]{8}$/);
  });

  it('extracts a reference from a transfer memo (case-insensitive)', () => {
    expect(extractPaymentReference('CT DEN:DR-PRO-AB12CD34 thanh toan')).toBe('DR-PRO-AB12CD34');
    expect(extractPaymentReference('dr-pro-ab12cd34')).toBe('DR-PRO-AB12CD34');
  });

  it('round-trips: a generated reference is found in a memo', () => {
    const ref = generatePaymentReference();
    expect(extractPaymentReference(`thanh toan ${ref} cho draftright`)).toBe(ref);
  });

  it('returns null when no reference present', () => {
    expect(extractPaymentReference('random transfer')).toBeNull();
    expect(extractPaymentReference(null)).toBeNull();
    expect(extractPaymentReference('')).toBeNull();
  });
});
