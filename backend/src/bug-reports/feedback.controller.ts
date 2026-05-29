import {
  Controller, Post, Get, Body, Param, Query, Req, HttpCode, UnauthorizedException, Logger,
} from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { Throttle } from '@nestjs/throttler';
import { Request } from 'express';
import { BugReportsService } from './bug-reports.service';
import { CreateFeedbackDto } from './dto/create-feedback.dto';
import { decodeOptionalUserId } from './jwt-user';
import { formatDisplayNumber, ReportKind } from '../common/display-number';

/**
 * Public feedback API powering the upvote board at draftright.info/feedback
 * and the native "Suggest a feature" forms in every client.
 *
 *   POST /feedback            create a bug or feature request (JWT optional → user_id)
 *   GET  /feedback            public board feed (kind=feature, is_public), votes desc
 *   POST /feedback/:id/vote   toggle the caller's upvote (JWT REQUIRED)
 *
 * The legacy multipart `POST /bug-reports` route (screenshots) is untouched.
 */
@ApiTags('feedback')
@Controller('feedback')
export class FeedbackController {
  private readonly logger = new Logger(FeedbackController.name);
  constructor(private readonly feedback: BugReportsService) {}

  // See BugReportsController for the rationale: 5/min and 30/hour per IP is
  // tight enough to kill scripted spam while letting a frustrated user file a
  // run of legit reports.
  @Throttle({
    minute: { limit: 5, ttl: 60_000 },
    hour:   { limit: 30, ttl: 3_600_000 },
  })
  @Post()
  @HttpCode(201)
  @ApiOperation({ summary: 'Submit a bug report or feature request' })
  async create(@Body() dto: CreateFeedbackDto, @Req() req: Request) {
    // Honeypot — see CreateBugReportDto.website.
    if (dto.website && dto.website.trim().length > 0) {
      this.logger.warn(`Honeypot triggered (IP=${req.ip}, source=${dto.source}) — dropping submission`);
      return { id: null, message: 'Received. Thanks!' };
    }
    const row = await this.feedback.createFeedback(dto, decodeOptionalUserId(req));
    const ref = formatDisplayNumber(dto.kind as ReportKind, row.display_no);
    const noun = dto.kind === 'feature' ? 'Feature request' : 'Bug report';
    return {
      id: row.id,
      ref,
      message: ref ? `${noun} received. Thanks! Reference: ${ref}` : `${noun} received. Thanks!`,
    };
  }

  @Get()
  @ApiOperation({ summary: 'Public feature-request board feed (sorted by votes)' })
  async list(
    @Query() q: { page?: string; limit?: string; status?: string; target_platform?: string },
    @Req() req: Request,
  ) {
    return this.feedback.listPublicFeatures(q as any, decodeOptionalUserId(req));
  }

  @Post(':id/vote')
  @HttpCode(200)
  @ApiOperation({ summary: "Toggle the signed-in user's upvote on a feature request" })
  async vote(@Param('id') id: string, @Req() req: Request) {
    const userId = decodeOptionalUserId(req);
    if (!userId) throw new UnauthorizedException('sign in to vote');
    return this.feedback.toggleVote(id, userId);
  }
}
