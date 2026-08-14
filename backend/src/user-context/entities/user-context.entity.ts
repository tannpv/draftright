import { Entity, PrimaryColumn, Column, UpdateDateColumn } from 'typeorm';
import { secretTransformer } from '../../common/crypto/secret-transformer';
import { PROFILE_FIELD_MAX_LEN } from '../user-context.constants';

/**
 * Per-user rewrite personalization context (#173) — one row per user, keyed by
 * the user id (not a surrogate PK, so the row IS the user's context and cannot
 * duplicate). Injected into the rewrite prompt when `enabled`.
 *
 * `style_notes` is free text the user writes, so it is encrypted at rest via
 * the shared `secretTransformer` (#50) — same posture as app_settings secrets.
 * The structured fields (job/industry/audience) are low-sensitivity labels and
 * stay plaintext for admin/debug legibility.
 *
 * Phase 2 (learned_summary + learned_summary_at) is NOT added here — those
 * columns land with the learning pipeline so the schema never carries unused
 * columns ahead of their consumer.
 */
@Entity('user_contexts')
export class UserContext {
  /** FK to users.id. One context per user. */
  @PrimaryColumn({ type: 'uuid' })
  user_id: string;

  /** Opt-in. No context is injected until the user turns this on. */
  @Column({ type: 'boolean', default: false })
  enabled: boolean;

  @Column({ type: 'varchar', length: PROFILE_FIELD_MAX_LEN, default: '' })
  job_title: string;

  @Column({ type: 'varchar', length: PROFILE_FIELD_MAX_LEN, default: '' })
  industry: string;

  @Column({ type: 'varchar', length: PROFILE_FIELD_MAX_LEN, default: '' })
  audience: string;

  /** Free-text style guidance. Encrypted at rest (#50). */
  @Column({ type: 'text', default: '', transformer: secretTransformer })
  style_notes: string;

  @UpdateDateColumn({ type: 'timestamptz' })
  updated_at: Date;
}
