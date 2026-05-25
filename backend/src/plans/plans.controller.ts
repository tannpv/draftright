import { Controller, Get } from '@nestjs/common';
import { ApiOperation, ApiTags } from '@nestjs/swagger';
import { PlansService } from './plans.service';

/**
 * Public, unauthenticated plan catalog for the website + app storefront.
 * One source of truth so clients never hard-code (and never go stale on)
 * plan IDs or prices.
 */
@ApiTags('plans')
@Controller('plans')
export class PlansController {
  constructor(private readonly plansService: PlansService) {}

  @Get()
  @ApiOperation({ summary: 'List active subscription plans (public)' })
  list() {
    return this.plansService.findPublic();
  }
}
