import { randomBytes } from 'crypto';

/**
 * Single source of truth for the payment reference code (`DR-PRO-XXXXXXXX`).
 * Customers put this in the bank-transfer memo; the SePay/Casso/MB webhook
 * matches it back to the pending payment. The generator and the matcher MUST
 * agree on the format — keep both here.
 */
export const PAYMENT_REF_PREFIX = 'DR-PRO-';

/** Matches any DraftRight reference (`DR-<SECTION>-<ALNUM>`) in a transfer memo. */
export const PAYMENT_REF_REGEX = /DR-[A-Z]+-[A-Z0-9]+/;

export function generatePaymentReference(): string {
  return `${PAYMENT_REF_PREFIX}${randomBytes(4).toString('hex').toUpperCase()}`;
}

/** Extract a reference from free-text transfer content (case-insensitive). */
export function extractPaymentReference(text: string | null | undefined): string | null {
  const match = (text || '').toUpperCase().match(PAYMENT_REF_REGEX);
  return match ? match[0] : null;
}
