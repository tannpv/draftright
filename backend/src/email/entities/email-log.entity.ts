import { Column, CreateDateColumn, Entity, Index, PrimaryGeneratedColumn } from 'typeorm';

export type EmailStatus = 'sent' | 'failed' | 'skipped';

/**
 * One row per attempted email send. Written by EmailService.deliver() so the
 * admin can see exactly what went out (or didn't, and why) — independent of
 * the provider dashboard.
 */
@Entity('email_logs')
export class EmailLog {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Index()
  @Column({ type: 'varchar', length: 255 })
  to_email: string;

  /** The deliver() label, e.g. 'subscription-activated', 'verification email'. */
  @Index()
  @Column({ type: 'varchar', length: 64 })
  email_type: string;

  @Column({ type: 'varchar', length: 255 })
  subject: string;

  /** sent = provider accepted · failed = provider error · skipped = Resend not configured. */
  @Index()
  @Column({ type: 'varchar', length: 16 })
  status: EmailStatus;

  /** Resend message id when sent. */
  @Column({ type: 'varchar', length: 255, nullable: true })
  provider_id: string | null;

  @Column({ type: 'text', nullable: true })
  error: string | null;

  @CreateDateColumn()
  created_at: Date;
}
