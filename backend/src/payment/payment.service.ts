import { Injectable, BadRequestException, NotFoundException, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, MoreThanOrEqual } from 'typeorm';
import { Payment, PaymentMethod, PaymentStatus } from './entities/payment.entity';
import { PaymentStrategy, CheckoutResult, WebhookAction } from './strategies/payment-strategy.interface';
import { StripeStrategy } from './strategies/stripe.strategy';
import { PayPalStrategy } from './strategies/paypal.strategy';
import { VietQRStrategy } from './strategies/vietqr.strategy';
import { MomoStrategy } from './strategies/momo.strategy';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { User } from '../users/entities/user.entity';
import { randomBytes } from 'crypto';

@Injectable()
export class PaymentService {
  private strategies: Map<string, PaymentStrategy>;

  /**
   * Methods that are publicly exposed. Controlled via env var
   * `PAYMENT_ENABLED_METHODS=stripe,vietqr` (comma-separated).
   *
   * Phase 3a default = "stripe" only. VietQR/Casso will be added in Phase 3b
   * once the Vietnamese LLC is registered (Casso requires business docs).
   * PayPal + Momo stay implemented but disabled — webhooks return 404,
   * checkout requests rejected — until they're explicitly enabled.
   *
   * `bank_transfer` is an alias for `vietqr` — only enabled if `vietqr` is.
   */
  private readonly enabledMethods: Set<string>;

  private readonly logger = new Logger(PaymentService.name);

  constructor(
    @InjectRepository(Payment)
    private readonly paymentRepo: Repository<Payment>,
    @InjectRepository(User)
    private readonly userRepo: Repository<User>,
    private readonly plansService: PlansService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly stripeStrategy: StripeStrategy,
    private readonly paypalStrategy: PayPalStrategy,
    private readonly vietqrStrategy: VietQRStrategy,
    private readonly momoStrategy: MomoStrategy,
  ) {
    this.strategies = new Map<string, PaymentStrategy>([
      ['stripe', this.stripeStrategy],
      ['paypal', this.paypalStrategy],
      ['momo', this.momoStrategy],
      ['vietqr', this.vietqrStrategy],
      ['bank_transfer', this.vietqrStrategy],
    ]);

    const raw = (process.env.PAYMENT_ENABLED_METHODS || 'stripe').toLowerCase();
    this.enabledMethods = new Set(raw.split(',').map((s) => s.trim()).filter(Boolean));
    // bank_transfer is implicitly enabled iff vietqr is enabled
    if (this.enabledMethods.has('vietqr')) this.enabledMethods.add('bank_transfer');
  }

  // --- Generic: get strategy by method ---

  private getStrategy(method: string): PaymentStrategy {
    if (!this.enabledMethods.has(method)) {
      throw new NotFoundException(`Payment method '${method}' is not enabled.`);
    }
    const strategy = this.strategies.get(method);
    if (!strategy) throw new BadRequestException(`Unsupported payment method: ${method}`);
    return strategy;
  }

  // --- Generic: generate unique reference code ---

  private generateReferenceCode(): string {
    const rand = randomBytes(4).toString('hex').toUpperCase();
    return `DR-PRO-${rand}`;
  }

  // --- Generic: create checkout ---

  async createCheckout(
    userId: string,
    planId: string,
    method: string,
    options?: { success_url?: string; cancel_url?: string },
  ): Promise<CheckoutResult> {
    const plan = await this.plansService.findById(planId);
    if (!plan) throw new NotFoundException('Plan not found');
    if (plan.price_cents === 0) throw new BadRequestException('Cannot purchase a free plan');

    const strategy = this.getStrategy(method);

    // Load user for Stripe customer reuse + email pre-fill
    const user = await this.userRepo.findOne({ where: { id: userId } });
    if (!user) throw new NotFoundException('User not found');

    // Create payment record. Currency is taken from the plan now (multi-currency).
    const payment = this.paymentRepo.create({
      user_id: userId,
      plan_id: planId,
      amount: plan.price_cents,
      currency: plan.currency || 'USD',
      method: method as PaymentMethod,
      status: PaymentStatus.PENDING,
      reference_code: this.generateReferenceCode(),
      expires_at: new Date(Date.now() + 30 * 60 * 1000), // 30 min expiry
    });
    // Eagerly attach the plan so the strategy can read it without a re-fetch
    (payment as any).plan = plan;
    await this.paymentRepo.save(payment);

    // Delegate to strategy
    let result: CheckoutResult;
    try {
      result = await strategy.createCheckout(payment, {
        ...options,
        stripe_customer_id: user.stripe_customer_id,
        user_email: user.email,
      });
    } catch (err: any) {
      payment.status = PaymentStatus.FAILED;
      payment.notes = err.message;
      await this.paymentRepo.save(payment);
      throw new BadRequestException(err.message || 'Payment provider error');
    }

    // Save QR data if generated
    if (result.qr_data) {
      payment.qr_data = result.qr_data;
      await this.paymentRepo.save(payment);
    }

    return result;
  }

  // --- Generic: handle webhook from any provider ---

  async handleWebhook(method: string, payload: any, headers: any): Promise<{ success: boolean; reference_code?: string }> {
    const strategy = this.getStrategy(method);
    const action: WebhookAction = await strategy.verifyWebhook(payload, headers);

    switch (action.type) {
      case 'payment_completed': {
        // Stripe: capture the new Customer ID for reuse on next checkout
        if (action.stripe_customer_id) {
          const payment = await this.paymentRepo.findOne({ where: { reference_code: action.reference_code } });
          if (payment?.user_id) {
            await this.userRepo.update(payment.user_id, { stripe_customer_id: action.stripe_customer_id });
            this.logger.log(`Saved stripe_customer_id ${action.stripe_customer_id} for user ${payment.user_id}`);
          }
        }
        // Stripe: stamp subscription_id on the granted subscription via store_transaction_id
        const completion = await this.completePayment(action.reference_code, 'completed');
        if (action.stripe_subscription_id && completion.success) {
          await this.subscriptionsService.stampStripeSubscription(
            action.reference_code,
            action.stripe_subscription_id,
          );
        }
        return completion;
      }

      case 'payment_failed':
        return this.completePayment(action.reference_code, 'failed');

      case 'subscription_renewed':
        await this.handleSubscriptionRenewed(action.stripe_subscription_id, action.current_period_end);
        return { success: true };

      case 'subscription_canceled':
        await this.handleSubscriptionCanceled(action.stripe_subscription_id);
        return { success: true };

      case 'dispute_created':
        this.logger.warn(`Stripe dispute on charge ${action.stripe_charge_id}, amount ${action.amount}. Manual review required.`);
        return { success: true };

      case 'ignored':
      default:
        return { success: false };
    }
  }

  /**
   * Stripe `invoice.payment_succeeded` — extend the subscription's expires_at
   * to the new period end. Idempotent: setting same expiry twice is safe.
   */
  private async handleSubscriptionRenewed(stripeSubId: string, periodEndUnixSec: number): Promise<void> {
    const expiresAt = new Date(periodEndUnixSec * 1000);
    const updated = await this.subscriptionsService.extendByStripeSubId(stripeSubId, expiresAt);
    this.logger.log(`Renewed Stripe sub ${stripeSubId} → expires ${expiresAt.toISOString()} (rows=${updated})`);
  }

  /**
   * Stripe `customer.subscription.deleted` — mark the matching subscription
   * cancelled. Whether it was customer-initiated, dunning failure, or admin
   * doesn't change our action: stop renewing, let access lapse at expires_at.
   */
  private async handleSubscriptionCanceled(stripeSubId: string): Promise<void> {
    const updated = await this.subscriptionsService.cancelByStripeSubId(stripeSubId);
    this.logger.log(`Cancelled Stripe sub ${stripeSubId} (rows=${updated})`);
  }

  // --- Generic: complete/fail a payment ---

  async completePayment(referenceCode: string, status: 'completed' | 'failed'): Promise<{ success: boolean; reference_code: string }> {
    const payment = await this.paymentRepo.findOne({
      where: { reference_code: referenceCode },
      relations: ['plan'],
    });

    if (!payment) return { success: false, reference_code: referenceCode };
    if (payment.status !== PaymentStatus.PENDING) return { success: true, reference_code: referenceCode };

    if (status === 'completed') {
      payment.status = PaymentStatus.COMPLETED;
      payment.completed_at = new Date();
      await this.paymentRepo.save(payment);

      // Activate subscription
      await this.activateSubscription(payment);
    } else {
      payment.status = PaymentStatus.FAILED;
      await this.paymentRepo.save(payment);
    }

    return { success: true, reference_code: referenceCode };
  }

  // --- Generic: activate subscription after successful payment ---

  private async activateSubscription(payment: Payment): Promise<void> {
    if (!payment.plan) {
      const fullPayment = await this.paymentRepo.findOne({
        where: { id: payment.id },
        relations: ['plan'],
      });
      payment = fullPayment || payment;
    }

    const billingPeriod = payment.plan?.billing_period || 'monthly';
    const expiresAt = new Date();
    if (billingPeriod === 'yearly') {
      expiresAt.setFullYear(expiresAt.getFullYear() + 1);
    } else {
      expiresAt.setMonth(expiresAt.getMonth() + 1);
    }

    await this.subscriptionsService.grant(payment.user_id, payment.plan_id, expiresAt);
  }

  // --- Generic: get payment status ---

  async getStatus(referenceCode: string): Promise<Payment | null> {
    return this.paymentRepo.findOne({
      where: { reference_code: referenceCode },
      relations: ['plan'],
    });
  }

  // --- Generic: admin confirm (for bank transfers) ---

  async adminConfirm(paymentId: string, adminNotes?: string): Promise<Payment> {
    const payment = await this.paymentRepo.findOne({
      where: { id: paymentId },
      relations: ['plan'],
    });
    if (!payment) throw new NotFoundException('Payment not found');
    if (payment.status !== PaymentStatus.PENDING) throw new BadRequestException('Payment is not pending');

    payment.status = PaymentStatus.COMPLETED;
    payment.completed_at = new Date();
    payment.notes = adminNotes || 'Manually confirmed by admin';
    await this.paymentRepo.save(payment);

    await this.activateSubscription(payment);
    return payment;
  }

  // --- Generic: list payments (admin) ---

  async findAll(options: { page?: number; limit?: number; status?: string }): Promise<{ payments: Payment[]; total: number }> {
    const { page = 1, limit = 20, status } = options;
    const where: any = {};
    if (status) where.status = status;

    const [payments, total] = await this.paymentRepo.findAndCount({
      where,
      relations: ['user', 'plan'],
      order: { created_at: 'DESC' },
      skip: (page - 1) * limit,
      take: limit,
    });
    return { payments, total };
  }

  // --- Generic: user's payment history ---

  async findByUser(userId: string): Promise<Payment[]> {
    return this.paymentRepo.find({
      where: { user_id: userId },
      relations: ['plan'],
      order: { created_at: 'DESC' },
      take: 20,
    });
  }

  // --- Generic: expire stale pending payments ---

  async expireStalePayments(): Promise<number> {
    const result = await this.paymentRepo
      .createQueryBuilder()
      .update(Payment)
      .set({ status: PaymentStatus.EXPIRED })
      .where('status = :status AND expires_at < NOW()', { status: PaymentStatus.PENDING })
      .execute();
    return result.affected || 0;
  }

  // --- Stats ---

  async getStats(): Promise<{ total: number; completed: number; pending: number; revenue: number }> {
    const [total, completed, pending] = await Promise.all([
      this.paymentRepo.count(),
      this.paymentRepo.count({ where: { status: PaymentStatus.COMPLETED } }),
      this.paymentRepo.count({ where: { status: PaymentStatus.PENDING } }),
    ]);

    const revenueResult = await this.paymentRepo
      .createQueryBuilder('p')
      .select('COALESCE(SUM(p.amount), 0)', 'total')
      .where('p.status = :status', { status: PaymentStatus.COMPLETED })
      .getRawOne();

    return { total, completed, pending, revenue: parseInt(revenueResult?.total || '0') };
  }
}
