import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';
import * as fs from 'fs';
import * as path from 'path';

@ApiTags('updates')
@Controller('updates')
export class UpdatesController {
  @Get('latest')
  @ApiOperation({ summary: 'Get latest app version info' })
  getLatest() {
    const configPath = path.join(__dirname, 'update-config.json');
    const raw = fs.readFileSync(configPath, 'utf-8');
    return JSON.parse(raw);
  }
}
