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
@Index('bug_reports_kind_public_votes_idx', ['kind', 'is_public', 'vote_count'])
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

  /** Discriminates bug reports ('bug', the legacy default) from feature requests ('feature'). */
  @Column({ type: 'varchar', length: 20, default: 'bug' })
  kind: string;

  /** Feature requests only: short one-line title (1-80 chars). Null for bugs. */
  @Column({ type: 'varchar', length: 80, nullable: true })
  title: string | null;

  /**
   * Feature requests only: which platform the request is *for*, picked by the
   * user in the submit dropdown. One of: playground | mobile | windows | mac | linux.
   * Distinct from `source` (where the request was submitted from). Null for bugs.
   */
  @Column({ type: 'varchar', length: 20, nullable: true, name: 'target_platform' })
  target_platform: string | null;

  /** Feature requests only: upvote count, derived — always recomputed as COUNT(feature_votes). */
  @Column({ type: 'int', default: 0, name: 'vote_count' })
  vote_count: number;

  /** Feature requests only: false hides the row from the public board (admin spam/dedupe lever). */
  @Column({ type: 'boolean', default: true, name: 'is_public' })
  is_public: boolean;

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
