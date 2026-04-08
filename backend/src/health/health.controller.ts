import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation } from '@nestjs/swagger';

@ApiTags('health')
@Controller('health')
export class HealthController {
  @Get()
  @ApiOperation({ summary: 'Health check with app identity' })
  getHealth() {
    return {
      app: 'draftright',
      version: '2.0.0',
      status: 'ok',
    };
  }
}
