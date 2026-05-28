import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
  ManyToOne, JoinColumn, Index,
} from 'typeorm';
import { User } from '../../users/entities/user.entity';
import { Plan } from '../../plans/entities/plan.entity';

export enum PaymentMethod {
  STRIPE = 'stripe',
  PAYPAL = 'paypal',
  MOMO = 'momo',
  VIETQR = 'vietqr',
  BANK_TRANSFER = 'bank_transfer',
  LEMONSQUEEZY = 'lemonsqueezy',
}

export enum PaymentStatus {
  PENDING = 'pending',
  COMPLETED = 'completed',
  FAILED = 'failed',
  EXPIRED = 'expired',
  REFUNDED = 'refunded',
}

@Entity('payments')
@Index(['user_id', 'created_at'])
@Index(['status', 'created_at'])
export class Payment {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'uuid' })
  user_id: string;

  @ManyToOne(() => User)
  @JoinColumn({ name: 'user_id' })
  user: User;

  @Column({ type: 'uuid' })
  plan_id: string;

  @ManyToOne(() => Plan)
  @JoinColumn({ name: 'plan_id' })
  plan: Plan;

  @Column({ type: 'int' })
  amount: number; // in smallest currency unit (VND dong, USD cents)

  @Column({ type: 'varchar', length: 10, default: 'VND' })
  currency: string;

  @Column({ type: 'varchar', length: 20 })
  method: PaymentMethod;

  @Column({ type: 'varchar', length: 20, default: PaymentStatus.PENDING })
  status: PaymentStatus;

  @Column({ type: 'varchar', length: 255, nullable: true })
  provider_ref: string; // Stripe payment_intent, PayPal order_id, bank ref

  @Column({ type: 'varchar', length: 50, unique: true })
  reference_code: string; // DR-PRO-xxxxx — used in bank transfer description

  @Column({ type: 'text', nullable: true })
  qr_data: string; // VietQR payload or QR image URL

  @Column({ type: 'text', nullable: true })
  notes: string; // admin notes, error details

  @Column({ type: 'timestamp', nullable: true })
  expires_at: Date; // QR/pending payment expiry (30 min)

  @Column({ type: 'timestamp', nullable: true })
  completed_at: Date;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
