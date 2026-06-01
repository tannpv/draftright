import { Body, Controller, Get, Headers, Post, Req, UseGuards } from '@nestjs/common';
import { ApiTags } from '@nestjs/swagger';
import type { RawBodyRequest } from '@nestjs/common';
import type { Request } from 'express';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { LemonsqueezyService } from './lemonsqueezy.service';

@ApiTags('lemonsqueezy')
@Controller('lemonsqueezy')
export class LemonsqueezyController {
  constructor(private readonly svc: LemonsqueezyService) {}

  @UseGuards(JwtAuthGuard)
  @Post('checkout')
  async createCheckout(@Req() req: any) {
    const url = await this.svc.createCheckoutUrl(req.user.id);
    return { url };
  }

  /**
   * @deprecated Use the unified `GET /payment/portal` instead — it
   * dispatches to the right provider per the user's active
   * subscription (LS or Stripe).  Kept here for backward compat with
   * older clients (pre-2026-06-01); remove after all clients ship
   * the new path.
   */
  @UseGuards(JwtAuthGuard)
  @Get('portal')
  async getPortal(@Req() req: any) {
    const url = await this.svc.createCustomerPortalUrl(req.user.id);
    return { url };
  }

  @Post('webhook')
  async webhook(
    @Req() req: RawBodyRequest<Request>,
    @Headers('x-signature') signature: string | undefined,
    @Body() body: any,
  ) {
    if (!req.rawBody) {
      // rawBody is required for HMAC verification — refuse anything else.
      return { error: 'Missing raw body' };
    }
    this.svc.verifyWebhookSignature(req.rawBody, signature);
    await this.svc.handleWebhook(body);
    return { received: true };
  }
}
