import {
  Entity, PrimaryGeneratedColumn, Column, Index,
  CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

/**
 * One row per user-submitted bug report. Submitted via the public
 * `POST /bug-reports` endpoint (multipart/form-data with optional
 * screenshot). Distinct from auto-collected error reports.
 *
 * Status values:
 *   'new'        — just arrived, awaiting triage
 *   'reviewing'  — admin is investigating
 *   'resolved'   — fix shipped or otherwise addressed
 *   'wont_fix'   — out of scope / by design
 */
@Entity('bug_reports')
@Index('bug_reports_status_idx', ['status', 'created_at'])
@Index('bug_reports_source_idx', ['source'])
export class BugReport {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 50 })
  source: string;

  @Column({ type: 'text' })
  description: string;

  @Column({ type: 'varchar', length: 500, nullable: true, name: 'screenshot_path' })
  screenshot_path: string | null;

  @Column({ type: 'varchar', length: 200, nullable: true, name: 'screenshot_filename' })
  screenshot_filename: string | null;

  @Column({ type: 'varchar', length: 50, nullable: true, name: 'app_version' })
  app_version: string | null;

  @Column({ type: 'varchar', length: 100, nullable: true, name: 'os_info' })
  os_info: string | null;

  @Column({ type: 'uuid', nullable: true, name: 'user_id' })
  user_id: string | null;

  @Column({ type: 'varchar', length: 255, nullable: true, name: 'user_email' })
  user_email: string | null;

  @Column({ type: 'jsonb', nullable: true })
  context: Record<string, any> | null;

  @Column({ type: 'varchar', length: 20, default: 'new' })
  status: string;

  @Column({ type: 'text', nullable: true, name: 'admin_notes' })
  admin_notes: string | null;

  @Column({ type: 'text', nullable: true, name: 'ai_fix_proposal' })
  ai_fix_proposal: string | null;

  @Column({ type: 'timestamptz', nullable: true, name: 'ai_fix_proposed_at' })
  ai_fix_proposed_at: Date | null;

  @CreateDateColumn({ name: 'created_at' })
  created_at: Date;

  @UpdateDateColumn({ name: 'updated_at' })
  updated_at: Date;
}
