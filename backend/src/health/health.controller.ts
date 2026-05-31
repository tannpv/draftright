import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { SkipThrottle } from '@nestjs/throttler';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AppSettings } from '../admin/entities/app-settings.entity';

@ApiTags('health')
@Controller('health')
// External monitors (Caddy probe, container healthcheck, uptime
// services) hit /health every 10-30 s.  At the default per-IP limit
// (200/min) a single sustained monitor would lock other consumers
// out of /health.  Skip the throttler for liveness probes — the
// endpoint is internally cached and DB-tolerant, so flooding it is
// safe (also a perf-test prerequisite, 2026-05-31).
@SkipThrottle()
export class HealthController {
  // Clients poll /health every ~30s for liveness, so cache the log level
  // briefly to keep the endpoint a near-zero-cost read. An admin change in
  // the portal propagates to clients within this TTL.
  private cachedLevel = 'info';
  private cachedAt = 0;
  private static readonly TTL_MS = 30_000;

  constructor(
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
  ) {}

  @Get()
  @ApiOperation({ summary: 'Health check with app identity' })
  async getHealth() {
    return {
      app: 'draftright',
      version: '2.0.0',
      status: 'ok',
      // Minimum severity clients should log: 'off' | 'errors' | 'warnings' | 'info'.
      // Older clients simply ignore this field.
      client_log_level: await this.getClientLogLevel(),
    };
  }

  private async getClientLogLevel(): Promise<string> {
    const now = Date.now();
    if (now - this.cachedAt < HealthController.TTL_MS) return this.cachedLevel;
    try {
      const settings = await this.settingsRepo.findOne({ where: {} });
      this.cachedLevel = settings?.client_log_level || 'info';
    } catch {
      // A DB hiccup must never make /health fail — clients use it for
      // liveness. Keep serving the last known level (or the default).
      this.cachedLevel = this.cachedLevel || 'info';
    }
    this.cachedAt = now;
    return this.cachedLevel;
  }
}
