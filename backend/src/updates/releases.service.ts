import { Injectable, BadRequestException, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AppRelease } from './entities/app-release.entity';
import { AppReleasePolicy } from './entities/app-release-policy.entity';

export type Platform = 'mac' | 'windows' | 'linux' | 'android' | 'ios';
export type Channel = 'direct' | 'store';

const PLATFORMS: Platform[] = ['mac', 'windows', 'linux', 'android', 'ios'];
const CHANNELS: Channel[] = ['direct', 'store'];

@Injectable()
export class ReleasesService {
  constructor(
    @InjectRepository(AppRelease)
    private readonly releaseRepo: Repository<AppRelease>,
    @InjectRepository(AppReleasePolicy)
    private readonly policyRepo: Repository<AppReleasePolicy>,
  ) {}

  /**
   * Returns every (platform, channel) row + every policy. Shape designed
   * for the admin Versions page — one card per platform, two channels
   * each, plus the policy toggle/status per card.
   */
  async listAll() {
    const [rows, policies] = await Promise.all([
      this.releaseRepo.find(),
      this.policyRepo.find(),
    ]);
    const byPlatform: Record<string, { policy: AppReleasePolicy | null; channels: Record<string, AppRelease | null> }> = {};
    for (const p of PLATFORMS) {
      byPlatform[p] = {
        policy: null,
        channels: { direct: null, store: null },
      };
    }
    for (const row of rows) {
      if (!byPlatform[row.platform]) continue;
      byPlatform[row.platform].channels[row.channel] = row;
    }
    for (const policy of policies) {
      if (!byPlatform[policy.platform]) continue;
      byPlatform[policy.platform].policy = policy;
    }
    return byPlatform;
  }

  /**
   * Returns the row that `/updates/latest` should surface for one platform.
   * Reads the platform's `preferred` channel from the policy row, falls
   * back to whatever channel exists if preferred is missing, returns null
   * if neither channel has an enabled row.
   *
   * Override hint comes from clients via `?channel=store|direct` — that
   * just gets passed through here.
   */
  async getEffective(platform: string, override?: string | null): Promise<AppRelease | null> {
    const policy = await this.policyRepo.findOne({ where: { platform } });
    const desired = (override && CHANNELS.includes(override as Channel))
      ? override
      : (policy?.preferred ?? 'direct');

    const preferredRow = await this.releaseRepo.findOne({
      where: { platform, channel: desired, enabled: true },
    });
    if (preferredRow) return preferredRow;

    // Fallback: if preferred isn't available, use the other channel.
    const other = desired === 'direct' ? 'store' : 'direct';
    const fallback = await this.releaseRepo.findOne({
      where: { platform, channel: other, enabled: true },
    });
    return fallback ?? null;
  }

  /** Insert or update one (platform, channel) row. */
  async upsertChannel(input: {
    platform: string;
    channel: string;
    version: string;
    download_url: string;
    sha256?: string;
    release_notes?: string;
    required?: boolean;
    enabled?: boolean;
  }): Promise<AppRelease> {
    if (!PLATFORMS.includes(input.platform as Platform)) {
      throw new BadRequestException(`platform must be one of: ${PLATFORMS.join(', ')}`);
    }
    if (!CHANNELS.includes(input.channel as Channel)) {
      throw new BadRequestException(`channel must be one of: ${CHANNELS.join(', ')}`);
    }
    if (!input.version || !input.download_url) {
      throw new BadRequestException('version and download_url are required');
    }
    if (input.sha256 !== undefined && input.sha256 !== '' && !/^[0-9a-f]{64}$/i.test(input.sha256)) {
      throw new BadRequestException('sha256 must be a 64-char hex string (or empty)');
    }
    const sha256 = input.sha256?.toLowerCase();
    const existing = await this.releaseRepo.findOne({
      where: { platform: input.platform, channel: input.channel },
    });
    if (existing) {
      existing.version = input.version;
      existing.download_url = input.download_url;
      // A new artifact replaces the hash; if the publisher didn't supply one,
      // clear the stale hash so we never verify a new file against an old hash.
      existing.sha256 = sha256 ?? '';
      if (input.release_notes !== undefined) existing.release_notes = input.release_notes;
      if (input.required !== undefined) existing.required = input.required;
      if (input.enabled !== undefined) existing.enabled = input.enabled;
      return this.releaseRepo.save(existing);
    }
    const row = this.releaseRepo.create({
      platform: input.platform,
      channel: input.channel,
      version: input.version,
      download_url: input.download_url,
      sha256: sha256 ?? '',
      release_notes: input.release_notes ?? '',
      required: input.required ?? false,
      enabled: input.enabled ?? true,
    });
    return this.releaseRepo.save(row);
  }

  async deleteChannel(platform: string, channel: string): Promise<void> {
    const result = await this.releaseRepo.delete({ platform, channel });
    if (!result.affected) {
      throw new NotFoundException(`No release row for ${platform}/${channel}`);
    }
  }

  /** Backwards-compat: existing callers (release-publish.sh) hit upsert without channel. */
  async upsert(input: {
    platform: string;
    version: string;
    download_url: string;
    release_notes?: string;
    required?: boolean;
  }): Promise<AppRelease> {
    return this.upsertChannel({ ...input, channel: 'direct' });
  }

  /** Returns the per-platform map used by the legacy /updates/latest payload. */
  async listEffective(override?: string | null): Promise<Record<Platform, AppRelease | null>> {
    const out: Record<string, AppRelease | null> = {};
    for (const p of PLATFORMS) {
      out[p] = await this.getEffective(p, override);
    }
    return out as Record<Platform, AppRelease | null>;
  }
}
