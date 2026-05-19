import { Controller, Get, Query } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiQuery } from '@nestjs/swagger';
import { AppRelease } from './entities/app-release.entity';
import { Platform, ReleasesService } from './releases.service';

const PLATFORMS: readonly Platform[] = ['mac', 'windows', 'linux', 'android', 'ios'] as const;

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

function isPlatform(value: string | undefined): value is Platform {
  return !!value && (PLATFORMS as readonly string[]).includes(value);
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
  @ApiQuery({ name: 'platform', required: false, enum: PLATFORMS as unknown as string[] })
  async getLatest(
    @Query('channel') channelOverride?: string,
    @Query('platform') platformOverride?: string,
  ) {
    const all = await this.releases.listEffective(channelOverride);
    const anchor = isPlatform(platformOverride) ? all[platformOverride] : null;
    const envelope = buildEnvelope(all, anchor);
    return {
      version: envelope.version,
      mac_url: all.mac?.download_url ?? '',
      windows_url: all.windows?.download_url ?? '',
      linux_url: all.linux?.download_url ?? '',
      android_url: all.android?.download_url ?? '',
      ios_url: all.ios?.download_url ?? '',
      release_notes: envelope.release_notes,
      required: envelope.required,
      // Per-platform expansion — always present so smart clients can pin to
      // the row matching their `*_url` and never get fooled by an envelope
      // computed across other platforms.
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

/**
 * Picks the (version, release_notes, required) triple that fills the legacy
 * top-level envelope of `/updates/latest`. With a platform anchor (caller
 * passed `?platform=windows`) the envelope is taken straight from that row,
 * so `version` is guaranteed to match the matching `<platform>_url` — the
 * mismatch was the root cause of the "current 2.2.10, install 2.3.1, still
 * 2.2.10" loop (top-level reported the cross-platform max, but `windows_url`
 * still pointed at the older Windows installer). With no anchor we keep the
 * legacy "max across all platforms" semantics so genuinely old clients still
 * *see* that something newer exists; smart clients should read
 * `platforms.<their-platform>` directly instead of trusting the envelope.
 */
function buildEnvelope(
  all: Record<Platform, AppRelease | null>,
  anchor: AppRelease | null,
): { version: string; release_notes: string; required: boolean } {
  if (anchor) {
    return {
      version: anchor.version,
      release_notes: anchor.release_notes ?? '',
      required: anchor.required ?? false,
    };
  }
  const mac = all.mac;
  const topVersion = maxVersion(
    Object.values(all).map((r) => r?.version).filter((v): v is string => !!v),
  );
  return {
    version: topVersion || mac?.version || '',
    release_notes: mac?.release_notes ?? '',
    required: mac?.required ?? false,
  };
}
