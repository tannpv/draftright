import { Controller, Get, Query } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiQuery } from '@nestjs/swagger';
import { ReleasesService } from './releases.service';

/** Compare two dotted numeric version strings ("2.2.10" > "2.2.9"). */
function compareVersions(a: string, b: string): number {
  const pa = a.split('.').map((n) => parseInt(n, 10) || 0);
  const pb = b.split('.').map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

/** Highest version in the list, or '' if the list is empty. */
function maxVersion(versions: string[]): string {
  return versions.reduce((max, v) => (compareVersions(v, max) > 0 ? v : max), '');
}

/**
 * Public endpoint polled by every desktop app's "Check for Updates" flow.
 *
 * Reads each platform's `preferred` channel from `app_release_policies`
 * and returns the corresponding (platform, channel) row from
 * `app_releases`. Clients can override with `?channel=store|direct` for
 * testing — e.g. previewing the store URL before flipping the toggle.
 *
 * Response shape preserved from 2.1.x clients.
 *
 * Updates flow: scripts/release-publish.sh → POST /admin/releases (with
 * admin JWT) → DB row updated → next /updates/latest call reflects it.
 * Or admin Versions page → flip preferred → channel switches instantly.
 */
@ApiTags('updates')
@Controller('updates')
export class UpdatesController {
  constructor(private readonly releases: ReleasesService) {}

  @Get('latest')
  @ApiOperation({ summary: 'Get latest app version info per platform' })
  @ApiQuery({ name: 'channel', required: false, enum: ['direct', 'store'] })
  async getLatest(@Query('channel') channelOverride?: string) {
    const all = await this.releases.listEffective(channelOverride);
    // Use mac's release_notes + required as the "envelope" notes/required
    // since the 2.1.x clients only show notes for their own platform anyway.
    const mac = all.mac;
    // The legacy top-level `version` is what older clients compare against to
    // decide "is there an update?" — but they then download their *own*
    // platform's `*_url`. If it were just mac's version, a Windows-only
    // release would be invisible to Windows clients until macOS also bumped.
    // Report the highest version across all platforms so every client at
    // least *sees* that something newer exists; the per-platform `platforms`
    // map (and the `*_url` fields) carry the actual target.
    const topVersion = maxVersion(
      Object.values(all).map((r) => r?.version).filter((v): v is string => !!v),
    );
    return {
      version: topVersion || mac?.version || '',
      mac_url: mac?.download_url ?? '',
      windows_url: all.windows?.download_url ?? '',
      linux_url: all.linux?.download_url ?? '',
      android_url: all.android?.download_url ?? '',
      ios_url: all.ios?.download_url ?? '',
      release_notes: mac?.release_notes ?? '',
      required: mac?.required ?? false,
      // Per-platform expansion for newer clients that want the full picture:
      platforms: Object.fromEntries(
        Object.entries(all)
          .filter(([_, v]) => v !== null)
          .map(([k, v]) => [k, {
            version: v!.version,
            url: v!.download_url,
            notes: v!.release_notes,
            required: v!.required,
            channel: v!.channel,
            updated_at: v!.updated_at,
          }]),
      ),
    };
  }
}
