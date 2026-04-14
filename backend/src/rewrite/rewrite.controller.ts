import { Controller, Post, Body, UseGuards, Req } from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { Request } from 'express';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { RewriteService } from './rewrite.service';
import { RewriteDto } from './dto/rewrite.dto';

@ApiTags('rewrite')
@Controller('rewrite')
export class RewriteController {
  constructor(private readonly rewriteService: RewriteService) {}

  @UseGuards(JwtAuthGuard)
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
