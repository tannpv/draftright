import {
  Controller, Post, Get, Body, Param, Query, Req, HttpCode, UnauthorizedException,
} from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { Request } from 'express';
import * as jwt from 'jsonwebtoken';
import { BugReportsService } from './bug-reports.service';
import { CreateFeedbackDto } from './dto/create-feedback.dto';

/** Decodes a Bearer JWT from the request; returns the user id or null. */
function userIdFrom(req: Request): string | null {
  const h = req.headers['authorization'];
  if (typeof h !== 'string' || !h.startsWith('Bearer ')) return null;
  try {
    const decoded = jwt.verify(h.slice(7), process.env.JWT_SECRET || 'change_me') as {
      sub?: string; user_id?: string;
    };
    return decoded.sub || decoded.user_id || null;
  } catch {
    return null;
  }
}

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
  constructor(private readonly feedback: BugReportsService) {}

  @Post()
  @HttpCode(201)
  @ApiOperation({ summary: 'Submit a bug report or feature request' })
  async create(@Body() dto: CreateFeedbackDto, @Req() req: Request) {
    const row = await this.feedback.createFeedback(dto, userIdFrom(req));
    return { id: row.id, message: dto.kind === 'feature' ? 'Feature request received. Thanks!' : 'Bug report received. Thanks!' };
  }

  @Get()
  @ApiOperation({ summary: 'Public feature-request board feed (sorted by votes)' })
  async list(
    @Query() q: { page?: string; limit?: string; status?: string; target_platform?: string },
    @Req() req: Request,
  ) {
    return this.feedback.listPublicFeatures(q as any, userIdFrom(req));
  }

  @Post(':id/vote')
  @HttpCode(200)
  @ApiOperation({ summary: "Toggle the signed-in user's upvote on a feature request" })
  async vote(@Param('id') id: string, @Req() req: Request) {
    const userId = userIdFrom(req);
    if (!userId) throw new UnauthorizedException('sign in to vote');
    return this.feedback.toggleVote(id, userId);
  }
}
