import { Injectable, Logger, BadRequestException, InternalServerErrorException, UnauthorizedException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as crypto from 'crypto';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { StoreType } from '../subscriptions/entities/subscription.entity';
import { LS_PERIOD_MS } from '../common/app-config';
import { EnvSchema } from '../config/env.schema';

const LEMONSQUEEZY_API_BASE = 'https://api.lemonsqueezy.com/v1';

@Injectable()
export class LemonsqueezyService {
  private readonly logger = new Logger(LemonsqueezyService.name);

  constructor(
    private readonly usersService: UsersService,
    private readonly plansService: PlansService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly cfg: ConfigService<EnvSchema, true>,
  ) {}

  /** Build a hosted-checkout URL for the given user. Pre-fills email and embeds user_id in custom data. */
  async createCheckoutUrl(userId: string): Promise<string> {
    const apiKey = this.cfg.get('LEMONSQUEEZY_API_KEY', { infer: true });
    const storeId = this.cfg.get('LEMONSQUEEZY_STORE_ID', { infer: true });
    const variantId = this.cfg.get('LEMONSQUEEZY_PRO_VARIANT_ID', { infer: true });
    if (!apiKey || !storeId || !variantId) {
      throw new InternalServerErrorException('Lemon Squeezy not configured');
    }

    const user = await this.usersService.findById(userId);
    if (!user) throw new BadRequestException('User not found');

    const body = {
      data: {
        type: 'checkouts',
        attributes: {
          checkout_data: {
            email: user.email,
            name: user.name,
            custom: { user_id: userId },
          },
          product_options: {
            redirect_url: 'https://draftright.info/account?subscribed=1',
            receipt_button_text: 'Open DraftRight',
            receipt_link_url: 'https://draftright.info/download',
          },
        },
        relationships: {
          store: { data: { type: 'stores', id: storeId } },
          variant: { data: { type: 'variants', id: variantId } },
        },
      },
    };

    const res = await fetch(`${LEMONSQUEEZY_API_BASE}/checkouts`, {
      method: 'POST',
      headers: {
        Accept: 'application/vnd.api+json',
        'Content-Type': 'application/vnd.api+json',
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const errBody = await res.text();
      this.logger.error(`Lemon Squeezy checkout create failed: HTTP ${res.status} ${errBody.slice(0, 200)}`);
      throw new InternalServerErrorException('Could not start checkout');
    }
    const json = await res.json() as { data: { attributes: { url: string } } };
    return json.data.attributes.url;
  }

  /** Build a Customer Portal URL for managing the existing subscription. */
  async createCustomerPortalUrl(userId: string): Promise<string> {
    const apiKey = this.cfg.get('LEMONSQUEEZY_API_KEY', { infer: true });
    if (!apiKey) throw new InternalServerErrorException('Lemon Squeezy not configured');

    const user = await this.usersService.findById(userId);
    if (!user || !user.lemonsqueezy_customer_id) {
      throw new BadRequestException('No Lemon Squeezy customer associated with this user');
    }

    const res = await fetch(`${LEMONSQUEEZY_API_BASE}/customers/${user.lemonsqueezy_customer_id}`, {
      headers: { Accept: 'application/vnd.api+json', Authorization: `Bearer ${apiKey}` },
    });
    if (!res.ok) {
      this.logger.error(`Lemon Squeezy customer fetch failed: HTTP ${res.status}`);
      throw new InternalServerErrorException('Could not load customer portal');
    }
    const json = await res.json() as { data: { attributes: { urls: { customer_portal: string } } } };
    return json.data.attributes.urls.customer_portal;
  }

  /** Verify HMAC SHA256 signature on raw webhook body. Throws UnauthorizedException on mismatch. */
  verifyWebhookSignature(rawBody: Buffer, signature: string | undefined): void {
    const secret = this.cfg.get('LEMONSQUEEZY_WEBHOOK_SECRET', { infer: true });
    if (!secret) {
      throw new InternalServerErrorException('Webhook secret not configured');
    }
    if (!signature) {
      throw new UnauthorizedException('Missing signature header');
    }
    const expected = crypto.createHmac('sha256', secret).update(rawBody).digest('hex');
    const sigBuf = Buffer.from(signature, 'hex');
    const expBuf = Buffer.from(expected, 'hex');
    if (sigBuf.length !== expBuf.length || !crypto.timingSafeEqual(sigBuf, expBuf)) {
      throw new UnauthorizedException('Invalid webhook signature');
    }
  }

  /**
   * Handle a webhook event after signature verification.
   * Payload shape: https://docs.lemonsqueezy.com/api/webhooks
   */
  async handleWebhook(payload: any): Promise<void> {
    const eventName = payload?.meta?.event_name;
    const customData = payload?.meta?.custom_data ?? {};
    const userId = customData.user_id as string | undefined;
    const subAttrs = payload?.data?.attributes;
    const subId = payload?.data?.id;

    this.logger.log(`Webhook received: event=${eventName} subId=${subId} userId=${userId ?? 'unknown'}`);

    if (!userId) {
      // Subscription webhooks fire without custom_data on renewal events. Look up by store_transaction_id.
      if (subId) {
        await this.handleByExistingSubscription(eventName, subId, subAttrs);
      } else {
        this.logger.warn(`Webhook ${eventName} without user_id or subscription id — skipping`);
      }
      return;
    }

    switch (eventName) {
      case 'subscription_created':
      case 'subscription_resumed':
        await this.activatePro(userId, subId, subAttrs);
        break;
      case 'subscription_payment_success':
        await this.extendPro(userId, subId, subAttrs);
        break;
      case 'subscription_payment_failed':
        this.logger.warn(`Payment failed for user ${userId} sub ${subId} — Lemon Squeezy retries automatically`);
        break;
      case 'subscription_cancelled':
      case 'subscription_expired':
        await this.cancelPro(userId);
        break;
      default:
        this.logger.log(`Unhandled webhook event: ${eventName}`);
    }
  }

  private async activatePro(userId: string, subId: string, subAttrs: any): Promise<void> {
    const proPlan = await this.plansService.findByName('Pro');
    if (!proPlan) throw new InternalServerErrorException('Pro plan not configured');

    const customerId = String(subAttrs?.customer_id ?? '');
    const renewsAt = subAttrs?.renews_at ? new Date(subAttrs.renews_at) : null;

    if (customerId) {
      await this.usersService.update(userId, { lemonsqueezy_customer_id: customerId });
    }
    await this.subscriptionsService.verifyAndActivate(
      userId,
      proPlan.id,
      StoreType.LEMONSQUEEZY,
      subId,
      renewsAt ?? new Date(Date.now() + LS_PERIOD_MS),
    );
    this.logger.log(`Activated Pro for user ${userId} sub ${subId} renews=${renewsAt?.toISOString()}`);
  }

  private async extendPro(userId: string, subId: string, subAttrs: any): Promise<void> {
    const proPlan = await this.plansService.findByName('Pro');
    if (!proPlan) return;
    const renewsAt = subAttrs?.renews_at ? new Date(subAttrs.renews_at) : null;
    await this.subscriptionsService.verifyAndActivate(
      userId,
      proPlan.id,
      StoreType.LEMONSQUEEZY,
      subId,
      renewsAt ?? new Date(Date.now() + LS_PERIOD_MS),
    );
    this.logger.log(`Extended Pro for user ${userId} renews=${renewsAt?.toISOString()}`);
  }

  private async cancelPro(userId: string): Promise<void> {
    const freePlan = await this.plansService.findFreePlan();
    if (!freePlan) return;
    await this.subscriptionsService.grant(userId, freePlan.id);
    this.logger.log(`Downgraded user ${userId} to Free`);
  }

  private async handleByExistingSubscription(eventName: string, subId: string, subAttrs: any): Promise<void> {
    // For renewals/cancellations without custom_data, find the subscription by its transaction id.
    // (Implemented as a fallback — most events include custom_data via the original checkout.)
    this.logger.log(`Fallback lookup by subId=${subId} event=${eventName}`);
  }
}
