import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  Post,
  Req,
  UseGuards,
} from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from './jwt-auth.guard';
import { MintExtensionTokenDto } from './dto/mint-extension-token.dto';
import { ExtensionTokenService } from './extension-token.service';

@ApiTags('auth')
@ApiBearerAuth()
@UseGuards(JwtAuthGuard)
@Controller('auth/extension-tokens')
export class ExtensionTokenController {
  constructor(private readonly service: ExtensionTokenService) {}

  @Post()
  // 200 instead of NestJS's default 201, matching feedback_nest_post_status.md.
  @HttpCode(200)
  async mint(@Req() req: any, @Body() dto: MintExtensionTokenDto) {
    return this.service.mint(req.user.id, dto.device_id, dto.device_name);
  }

  @Get()
  async list(@Req() req: any) {
    const rows = await this.service.list(req.user.id);
    // Strip server-only fields. Plaintext is never stored — only the
    // sha256 hash — but neither it nor the user_id should leak via this
    // endpoint.
    return rows.map(({ token_hash, user_id, ...rest }) => rest);
  }

  @Delete(':id')
  @HttpCode(204)
  async revoke(@Req() req: any, @Param('id') id: string) {
    await this.service.revoke(req.user.id, id);
  }
}
