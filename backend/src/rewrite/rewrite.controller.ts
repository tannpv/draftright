import { Controller, Post, Get, Body, UseGuards, Req } from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { Request } from 'express';
import { RewriteAuthGuard } from '../auth/rewrite-auth.guard';
import { RewriteService } from './rewrite.service';
import { RewriteDto } from './dto/rewrite.dto';
import { TONES } from './tones';

@ApiTags('rewrite')
@Controller('rewrite')
export class RewriteController {
  constructor(private readonly rewriteService: RewriteService) {}

  // Public tone catalog. Clients should render from this instead of
  // hardcoding the tone list, so a new tone ships without a client release.
  @Get('tones')
  getTones() {
    return { tones: TONES };
  }

  // Accepts either a regular user JWT or a dr_ext_* extension token with
  // the 'rewrite' scope. See RewriteAuthGuard.
  @UseGuards(RewriteAuthGuard)
  @ApiBearerAuth()
  @Post()
  async rewrite(@Req() req: any, @Body() dto: RewriteDto) {
    return this.rewriteService.rewrite(req.user.id, dto.text, dto.tone, dto.target_language, dto.source_language);
  }

  @Post('trial')
  async trialRewrite(@Req() req: Request, @Body() dto: RewriteDto) {
    const clientIp =
      (req.headers['x-forwarded-for'] as string)?.split(',')[0]?.trim() ||
      req.socket.remoteAddress ||
      'unknown';
    return this.rewriteService.trialRewrite(dto.text, dto.tone, clientIp, dto.target_language, dto.source_language);
  }
}
