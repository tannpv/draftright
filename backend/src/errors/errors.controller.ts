import { Controller, Post, Body, Req, Logger } from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { Throttle } from '@nestjs/throttler';
import { Request } from 'express';
import { ErrorsService } from './errors.service';
import { CreateErrorReportDto } from './dto/create-error-report.dto';
import * as jwt from 'jsonwebtoken';

/**
 * Public endpoint that any client (mobile, desktop, web) POSTs to when
 * it catches an unhandled error. Authenticated users get their user_id
 * stamped on the report (best-effort — the endpoint accepts anonymous
 * reports too so we don't lose telemetry from logged-out crashes).
 */
@ApiTags('errors')
@Controller('errors')
export class ErrorsController {
  private readonly logger = new Logger(ErrorsController.name);
  constructor(private readonly errors: ErrorsService) {}

  // Crash reports are noisier than bug reports (an app crashlooping can fire
  // many in a row), so the cap is higher: 30/min, 300/hour per IP.
  @Throttle({
    minute: { limit: 30, ttl: 60_000 },
    hour:   { limit: 300, ttl: 3_600_000 },
  })
  @Post()
  @ApiOperation({ summary: 'Submit an error/crash report from a client' })
  async create(@Body() dto: CreateErrorReportDto, @Req() req: Request) {
    // Honeypot — see CreateBugReportDto.website.
    if (dto.website && dto.website.trim().length > 0) {
      this.logger.warn(`Honeypot triggered (IP=${req.ip}, platform=${dto.platform}) — dropping submission`);
      return { ok: true, id: null, fingerprint: null, count: 0, first_seen_at: null };
    }
    let userId: string | null = null;
    const authHeader = req.headers['authorization'];
    if (typeof authHeader === 'string' && authHeader.startsWith('Bearer ')) {
      const token = authHeader.slice(7);
      try {
        const decoded = jwt.verify(
          token,
          process.env.JWT_SECRET || 'change_me',
        ) as { sub?: string; user_id?: string };
        userId = decoded.sub || decoded.user_id || null;
      } catch {
        // Bad token — fall through, treat as anonymous
      }
    }
    const row = await this.errors.ingest(dto, userId);
    return {
      ok: true,
      id: row.id,
      fingerprint: row.fingerprint,
      count: row.count,
      first_seen_at: row.first_seen_at,
    };
  }
}
