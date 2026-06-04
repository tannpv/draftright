import { Column, CreateDateColumn, Entity, PrimaryColumn } from 'typeorm';

/**
 * Addresses we must stop emailing — a hard bounce (the mailbox doesn't
 * exist) or a spam complaint. Populated by the Resend delivery webhook
 * and checked on every send to protect sender reputation.
 */
@Entity('email_suppressions')
export class EmailSuppression {
  /** Lower-cased recipient address. */
  @PrimaryColumn({ type: 'varchar', length: 255 })
  email: string;

  /** 'bounced' (hard) or 'complained'. */
  @Column({ type: 'varchar', length: 16 })
  reason: string;

  @CreateDateColumn()
  created_at: Date;
}
