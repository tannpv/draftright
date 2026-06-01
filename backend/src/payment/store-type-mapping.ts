import { PaymentMethod } from './entities/payment.entity';
import { StoreType } from '../subscriptions/entities/subscription.entity';

/**
 * Map an incoming payment method to the StoreType the resulting
 * subscription row should carry.  Centralising this keeps the
 * Subscription audit field accurate (used by analytics + by
 * `PaymentService.getCustomerPortalUrl` which dispatches per
 * store_type), and gives every payment strategy a single
 * one-line entry to add when a new method ships.
 *
 * Rule #1: extending = one new switch arm, no other touches.
 */
export function storeTypeForMethod(method: PaymentMethod): StoreType {
  switch (method) {
    case PaymentMethod.STRIPE:        return StoreType.STRIPE;
    case PaymentMethod.LEMONSQUEEZY:  return StoreType.LEMONSQUEEZY;
    case PaymentMethod.VIETQR:        return StoreType.VIETQR;
    case PaymentMethod.BANK_TRANSFER: return StoreType.BANK_TRANSFER;
    case PaymentMethod.PAYPAL:        return StoreType.PAYPAL;
    // MoMo was removed from the active method set in 2026-05-x but
    // the enum value stays for historical payment rows.  Map to
    // ADMIN_GRANTED so legacy rows still activate without crashing.
    case PaymentMethod.MOMO:          return StoreType.ADMIN_GRANTED;
    default:                          return StoreType.ADMIN_GRANTED;
  }
}
