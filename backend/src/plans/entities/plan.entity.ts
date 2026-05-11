import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

export enum BillingPeriod {
  NONE = 'none',
  MONTHLY = 'monthly',
  YEARLY = 'yearly',
}

@Entity('plans')
export class Plan {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 100 })
  name: string;

  @Column({ type: 'int' })
  daily_limit: number;

  /**
   * Smallest Stripe-native amount unit for the row's currency.
   * USD: cents (×100). VND: whole VND (no divisor).
   * Column name kept for back-compat; Phase 3c will rename to price_minor_units.
   */
  @Column({ type: 'int', default: 0 })
  price_cents: number;

  /** ISO 4217 currency code, e.g. 'USD' / 'VND'. NULL on the free tier. */
  @Column({ type: 'char', length: 3, nullable: true })
  currency: string | null;

  /** Stripe Price ID — populated after Stage 2 by sync_stripe_prices script. */
  @Column({ type: 'varchar', length: 255, nullable: true })
  stripe_price_id: string | null;

  /** Free trial length in days. 0 = no trial. Admin-changeable per plan. */
  @Column({ type: 'int', default: 30 })
  trial_days: number;

  @Column({ type: 'enum', enum: BillingPeriod, default: BillingPeriod.NONE })
  billing_period: BillingPeriod;

  @Column({ type: 'boolean', default: true })
  is_active: boolean;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
