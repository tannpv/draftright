import { Entity, PrimaryColumn, Column, UpdateDateColumn } from 'typeorm';

/**
 * One row per platform we publish on. Single source of truth for the
 * /updates/latest endpoint that the desktop apps poll. Updated by
 * `scripts/release-publish.sh` via the admin API — no backend rebuild
 * needed per release.
 *
 * Platform values: 'mac' | 'windows' | 'linux' | 'android' | 'ios'
 */
@Entity('app_releases')
export class AppRelease {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  platform: string;

  @Column({ type: 'varchar', length: 50 })
  version: string;

  @Column({ type: 'text', name: 'download_url' })
  download_url: string;

  @Column({ type: 'text', default: '', name: 'release_notes' })
  release_notes: string;

  @Column({ type: 'boolean', default: false })
  required: boolean;

  @UpdateDateColumn()
  updated_at: Date;
}
