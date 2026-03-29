import { Controller, Get, Post, Body, UseGuards, Request } from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { SubscriptionsService } from './subscriptions.service';

@ApiTags('subscription')
@Controller('subscription')
export class SubscriptionsController {
  constructor(
    private readonly subscriptionsService: SubscriptionsService,
  ) {}

  @Get()
  async getMySubscription(@Request() req: any) {
    const sub = await this.subscriptionsService.findActiveByUserId(req.user.id);
    return {
      plan: sub?.plan ? {
        name: sub.plan.name,
        daily_limit: sub.plan.daily_limit,
        billing_period: sub.plan.billing_period,
      } : null,
      status: sub?.status || null,
      expires_at: sub?.expires_at || null,
    };
  }

  @Post('verify-receipt')
  async verifyReceipt(@Request() req: any, @Body() body: { store_type: string; receipt_data: string; product_id: string }) {
    const sub = await this.subscriptionsService.findActiveByUserId(req.user.id);
    return {
      subscription: sub ? { plan: sub.plan?.name, status: sub.status, expires_at: sub.expires_at } : null,
    };
  }
}
