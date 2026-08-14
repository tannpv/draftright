import { Controller, Get, Put, Delete, Body, UseGuards, Request, HttpCode } from '@nestjs/common';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { UserContextService } from './user-context.service';
import { UpdateUserContextDto } from './dto/update-user-context.dto';
import { UserContext } from './entities/user-context.entity';

/** The client-facing shape — never exposes server-only columns. */
function toView(ctx: UserContext | null) {
  return {
    enabled: ctx?.enabled ?? false,
    job_title: ctx?.job_title ?? '',
    industry: ctx?.industry ?? '',
    audience: ctx?.audience ?? '',
    style_notes: ctx?.style_notes ?? '',
  };
}

/**
 * The caller's own personalization context. Always scoped to `req.user.id` —
 * a user can only ever read/write/delete their own row, never another's.
 */
@Controller('me/context')
@UseGuards(JwtAuthGuard)
export class UserContextController {
  constructor(private readonly service: UserContextService) {}

  /** The caller's profile, or empty defaults if they have never set one. */
  @Get()
  async get(@Request() req: any) {
    return toView(await this.service.get(req.user.id));
  }

  @Put()
  @HttpCode(200)
  async update(@Request() req: any, @Body() dto: UpdateUserContextDto) {
    return toView(await this.service.upsert(req.user.id, dto));
  }

  /** GDPR erasure — removes the row entirely. */
  @Delete()
  @HttpCode(204)
  async clear(@Request() req: any): Promise<void> {
    await this.service.clear(req.user.id);
  }
}
