import { Controller, Get, Query } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiQuery } from '@nestjs/swagger';
import { ReleasesService } from './releases.service';

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
    return {
      version: mac?.version ?? '',
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
