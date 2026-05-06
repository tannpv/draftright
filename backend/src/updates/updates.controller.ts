import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { ReleasesService } from './releases.service';

/**
 * Public endpoint polled by every desktop app's "Check for Updates" flow.
 * Aggregates the per-platform `app_releases` rows into the legacy response
 * shape so existing 2.1.x clients keep working.
 *
 * Updates flow: scripts/release-publish.sh → POST /admin/releases (with
 * admin JWT) → DB row updated → next /updates/latest call reflects it.
 * No backend rebuild, no container restart.
 */
@ApiTags('updates')
@Controller('updates')
export class UpdatesController {
  constructor(private readonly releases: ReleasesService) {}

  @Get('latest')
  @ApiOperation({ summary: 'Get latest app version info per platform' })
  async getLatest() {
    const all = await this.releases.listAll();
    // Use mac's release_notes + required as the "envelope" notes/required
    // since the 2.1.x clients only show notes for their own platform anyway.
    // Each platform-specific URL falls back to '' if no row.
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
            updated_at: v!.updated_at,
          }]),
      ),
    };
  }
}
