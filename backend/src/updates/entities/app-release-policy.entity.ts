import { Entity, PrimaryColumn, Column, UpdateDateColumn } from 'typeorm';

/**
 * One row per platform. Decides which channel `/updates/latest` returns
 * by default + tracks the store submission state for admin visibility.
 *
 * `preferred` flips between 'direct' and 'store' — the moment a store
 * approves us, the admin opens the Versions page, flips this toggle,
 * and every running app's auto-updater immediately surfaces the store
 * URL on its next poll. No client rebuild, no backend deploy.
 *
 * `store_status` is informational — drives a status badge in the admin
 * UI ('not_submitted' | 'in_review' | 'approved' | 'rejected' | 'n/a').
 */
@Entity('app_release_policies')
export class AppReleasePolicy {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  platform: string;

  @Column({ type: 'varchar', length: 20, default: 'direct' })
  preferred: string;

  @Column({ type: 'varchar', length: 30, default: 'not_submitted', name: 'store_status' })
  store_status: string;

  @Column({ type: 'text', default: '' })
  notes: string;

  @UpdateDateColumn()
  updated_at: Date;
}
