import {
  Entity, PrimaryGeneratedColumn, Column, Index,
  CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

/**
 * One row per unique error fingerprint. Repeated occurrences increment
 * `count` and update `last_seen_at` instead of creating new rows.
 *
 * Status flow:
 *   0 NEW            — just arrived
 *   1 TRIAGED        — human acknowledged
 *   2 IN_PROGRESS    — being worked
 *   3 FIX_PROPOSED   — AI proposed a fix, awaiting review
 *   4 RESOLVED       — fix shipped, awaiting confirmation in production
 *   5 CLOSED         — verified fixed
 *   6 WONT_FIX       — out of scope / by design
 *   7 DUPLICATE      — same as another fingerprint
 */
@Entity('error_reports')
@Index(['platform', 'status'])
@Index(['fingerprint'])
@Index(['last_seen_at'])
export class ErrorReport {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 20 })
  platform: string;       // 'ios' | 'android' | 'macos' | 'windows' | 'linux' | 'web'

  @Column({ type: 'varchar', length: 50, nullable: true, name: 'app_version' })
  app_version: string | null;

  @Column({ type: 'varchar', length: 20, default: 'error' })
  severity: string;       // 'fatal' | 'error' | 'warning' | 'info'

  @Column({ type: 'varchar', length: 200, nullable: true, name: 'error_type' })
  error_type: string | null;

  @Column({ type: 'text', nullable: true })
  message: string | null;

  @Column({ type: 'text', nullable: true, name: 'stack_trace' })
  stack_trace: string | null;

  @Column({ type: 'jsonb', nullable: true })
  context: Record<string, any> | null;

  @Column({ type: 'uuid', nullable: true, name: 'user_id' })
  user_id: string | null;

  @Column({ type: 'varchar', length: 100, nullable: true, name: 'device_id' })
  device_id: string | null;

  @Column({ type: 'char', length: 64 })
  fingerprint: string;    // sha256 of error_type + first N stack frames

  @Column({ type: 'integer', default: 1 })
  count: number;

  @Column({ type: 'integer', default: 0 })
  status: number;

  @Column({ type: 'text', nullable: true, name: 'ai_fix_proposal' })
  ai_fix_proposal: string | null;

  @Column({ type: 'varchar', length: 100, nullable: true, name: 'resolved_by' })
  resolved_by: string | null;

  @Column({ type: 'timestamptz', nullable: true, name: 'resolved_at' })
  resolved_at: Date | null;

  @CreateDateColumn({ name: 'first_seen_at' })
  first_seen_at: Date;

  @UpdateDateColumn({ name: 'last_seen_at' })
  last_seen_at: Date;
}
