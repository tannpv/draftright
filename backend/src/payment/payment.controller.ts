import {
  Controller, Post, Get, Delete, Body, Param, Query, Req, UseGuards, RawBodyRequest, HttpCode,
} from '@nestjs/common';
import { Request } from 'express';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { PaymentService } from './payment.service';
import { CreateCheckoutDto } from './dto/create-checkout.dto';
import { PaymentMethod } from './entities/payment.entity';

@ApiTags('payment')
@Controller('payment')
export class PaymentController {
  constructor(private readonly paymentService: PaymentService) {}

  // --- Public: which payment methods the storefront should show ---

  @Get('methods')
  async enabledMethods() {
    return { methods: await this.paymentService.getEnabledMethods() };
  }

  // --- Authenticated: create checkout ---

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Post('checkout')
  async createCheckout(@Req() req: any, @Body() dto: CreateCheckoutDto) {
    return this.paymentService.createCheckout(
      req.user.id,
      dto.plan_id,
      dto.method,
      { success_url: dto.success_url, cancel_url: dto.cancel_url },
    );
  }

  // --- Public: check payment status (by reference code) ---

  @Get('status/:ref')
  async getStatus(@Param('ref') ref: string) {
    const payment = await this.paymentService.getStatus(ref);
    if (!payment) return { status: 'not_found' };
    return {
      status: payment.status,
      method: payment.method,
      amount: payment.amount,
      currency: payment.currency,
      reference_code: payment.reference_code,
      plan_name: payment.plan?.name,
      completed_at: payment.completed_at,
      expires_at: payment.expires_at,
    };
  }

  // --- Authenticated: user's payment history ---

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Get('history')
  async getHistory(@Req() req: any) {
    return this.paymentService.findByUser(req.user.id);
  }

  // --- Authenticated: unified customer portal ---
  //
  // Replaces the provider-specific /lemonsqueezy/portal route.  Looks
  // up the user's active subscription and dispatches to the right
  // provider's portal API.  Clients (mobile + 3 desktops) should
  // call this instead of /lemonsqueezy/portal; the LS-specific route
  // is kept for backward compat until those clients ship.

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Get('portal')
  async getPortal(@Req() req: any) {
    const url = await this.paymentService.getCustomerPortalUrl(req.user.id);
    return { url };
  }

  // --- Authenticated: in-app cancel ---
  //
  // Replaces the trip-to-LS-portal flow for the most common manage
  // action (cancel).  Card-update + plan-change still go via the
  // portal endpoint above.  See [[project_session_20260602_apple_signin]]
  // for the in-app cancel rationale.
  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @Delete('subscription')
  @HttpCode(200)
  async cancelSubscription(@Req() req: any) {
    return this.paymentService.cancelActiveSubscription(req.user.id);
  }

  // --- Public: webhooks from payment providers ---

  @Post('webhook/stripe')
  async stripeWebhook(@Req() req: RawBodyRequest<Request>) {
    return this.paymentService.handleWebhook(PaymentMethod.STRIPE, req.rawBody, req.headers);
  }

  @Post('webhook/vietqr')
  async vietqrWebhook(@Body() body: any, @Req() req: Request) {
    return this.paymentService.handleWebhook(PaymentMethod.VIETQR, body, req.headers);
  }

  // Casso + SePay both feed into the VIETQR strategy (same statement-line
  // schema). Route names are kept for legacy webhook URLs already registered
  // with each provider; the strategy dispatch is the canonical key.
  @Post('webhook/casso')
  async cassoWebhook(@Body() body: any, @Req() req: Request) {
    return this.paymentService.handleWebhook(PaymentMethod.VIETQR, body, req.headers);
  }

  @Post('webhook/sepay')
  async sepayWebhook(@Body() body: any, @Req() req: Request) {
    return this.paymentService.handleWebhook(PaymentMethod.VIETQR, body, req.headers);
  }

  @Post('webhook/lemonsqueezy')
  async lemonSqueezyWebhook(@Req() req: RawBodyRequest<Request>) {
    return this.paymentService.handleWebhook(PaymentMethod.LEMONSQUEEZY, req.rawBody, req.headers);
  }
}
