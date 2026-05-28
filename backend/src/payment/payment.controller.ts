import {
  Controller, Post, Get, Body, Param, Query, Req, UseGuards, RawBodyRequest,
} from '@nestjs/common';
import { Request } from 'express';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { PaymentService } from './payment.service';
import { CreateCheckoutDto } from './dto/create-checkout.dto';

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

  // --- Public: webhooks from payment providers ---

  @Post('webhook/stripe')
  async stripeWebhook(@Req() req: RawBodyRequest<Request>) {
    return this.paymentService.handleWebhook('stripe', req.rawBody, req.headers);
  }

  @Post('webhook/vietqr')
  async vietqrWebhook(@Body() body: any, @Req() req: Request) {
    return this.paymentService.handleWebhook('vietqr', body, req.headers);
  }

  @Post('webhook/casso')
  async cassoWebhook(@Body() body: any, @Req() req: Request) {
    return this.paymentService.handleWebhook('vietqr', body, req.headers);
  }

  @Post('webhook/sepay')
  async sepayWebhook(@Body() body: any, @Req() req: Request) {
    return this.paymentService.handleWebhook('vietqr', body, req.headers);
  }

  @Post('webhook/lemonsqueezy')
  async lemonSqueezyWebhook(@Req() req: RawBodyRequest<Request>) {
    return this.paymentService.handleWebhook('lemonsqueezy', req.rawBody, req.headers);
  }
}
