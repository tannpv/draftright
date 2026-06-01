import { PaymentMethod } from './entities/payment.entity';
import { StoreType } from '../subscriptions/entities/subscription.entity';
import { storeTypeForMethod } from './store-type-mapping';

describe('storeTypeForMethod', () => {
  it('maps every PaymentMethod to its matching StoreType', () => {
    expect(storeTypeForMethod(PaymentMethod.STRIPE)).toBe(StoreType.STRIPE);
    expect(storeTypeForMethod(PaymentMethod.LEMONSQUEEZY)).toBe(StoreType.LEMONSQUEEZY);
    expect(storeTypeForMethod(PaymentMethod.VIETQR)).toBe(StoreType.VIETQR);
    expect(storeTypeForMethod(PaymentMethod.BANK_TRANSFER)).toBe(StoreType.BANK_TRANSFER);
    expect(storeTypeForMethod(PaymentMethod.PAYPAL)).toBe(StoreType.PAYPAL);
  });

  it('every active PaymentMethod has a non-ADMIN_GRANTED mapping (no silent drops)', () => {
    // MoMo is intentionally mapped to ADMIN_GRANTED (legacy enum
    // value, no longer an active payment method).  Every OTHER
    // method must carry its own store_type for analytics accuracy.
    const active = Object.values(PaymentMethod).filter((m) => m !== PaymentMethod.MOMO);
    for (const m of active) {
      expect(storeTypeForMethod(m as PaymentMethod)).not.toBe(StoreType.ADMIN_GRANTED);
    }
  });

  it('returns ADMIN_GRANTED as a defensive fallback for unknown values', () => {
    // Cast through `unknown` since TypeScript prevents this at compile time;
    // covers the runtime branch where a new PaymentMethod is added without
    // updating the switch.
    expect(storeTypeForMethod('future-method' as unknown as PaymentMethod))
      .toBe(StoreType.ADMIN_GRANTED);
  });
});
