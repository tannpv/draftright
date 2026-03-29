import { Controller, Post, Body, UseGuards, Request } from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
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
  async rewrite(@Request() req: any, @Body() dto: RewriteDto) {
    return this.rewriteService.rewrite(req.user.id, dto.text, dto.tone, dto.target_language);
  }
}
