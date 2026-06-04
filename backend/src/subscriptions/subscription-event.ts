export enum SubscriptionEvent {
  UPGRADED = 'upgraded',
  PLAN_CHANGED = 'plan_changed',
  EXPIRING_SOON = 'expiring_soon',
  EXPIRED = 'expired',
}

export interface SubscriptionEventPayload {
  email: string;
  name: string;
  planName: string;
  expiresAt?: Date;
  currency?: string;
  amountCents?: number;
}
