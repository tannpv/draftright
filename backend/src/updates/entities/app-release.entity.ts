import { Entity, PrimaryColumn, Column, UpdateDateColumn } from 'typeorm';

/**
 * One row per (platform, channel) we publish on. Two channels exist:
 *   - 'direct' — sideload / website download (DMG, APK, EXE, tarball)
 *   - 'store'  — Apple App Store, Google Play, Microsoft Store deep link
 *
 * Which one /updates/latest returns is controlled by the per-platform row
 * in `app_release_policies.preferred`. Both channels can coexist; only one
 * is exposed to clients at a time. A channel can be temporarily hidden by
 * setting `enabled = false`.
 *
 * Updated by `scripts/release-publish.sh` (always writes the 'direct' row)
 * and the admin portal Versions page (writes either channel). No backend
 * rebuild needed per release.
 *
 * Platform values: 'mac' | 'windows' | 'linux' | 'android' | 'ios'
 */
@Entity('app_releases')
export class AppRelease {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  platform: string;

  @PrimaryColumn({ type: 'varchar', length: 20, default: 'direct' })
  channel: string;

  @Column({ type: 'varchar', length: 50 })
  version: string;

  @Column({ type: 'text', name: 'download_url' })
  download_url: string;

  /**
   * Hex SHA-256 of the artifact at `download_url`. Empty when the publisher
   * didn't record one (older rows). Desktop updaters verify the downloaded
   * file against this before executing it; an empty value means "unverified"
   * (the client logs a warning but still installs, for backwards-compat).
   */
  @Column({ type: 'varchar', length: 64, default: '', name: 'sha256' })
  sha256: string;

  @Column({ type: 'text', default: '', name: 'release_notes' })
  release_notes: string;

  @Column({ type: 'boolean', default: false })
  required: boolean;

  @Column({ type: 'boolean', default: true })
  enabled: boolean;

  @UpdateDateColumn()
  updated_at: Date;
}
