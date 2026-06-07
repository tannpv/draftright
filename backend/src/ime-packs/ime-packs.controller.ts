import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import { ImePacksService } from './ime-packs.service';

/**
 * Public, server-driven catalog of keyboard languages. Clients fetch this to
 * show the "Add language" list and to download dictionary packs on demand.
 */
@ApiTags('ime-packs')
@Controller('ime-packs')
export class ImePacksController {
  constructor(private readonly imePacks: ImePacksService) {}

  @Get('manifest')
  @ApiOperation({ summary: 'List available keyboard languages + downloadable packs' })
  manifest() {
    return { languages: this.imePacks.catalog() };
  }
}
