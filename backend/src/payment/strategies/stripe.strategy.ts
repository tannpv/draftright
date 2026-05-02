import { Injectable } from '@nestjs/common';
import { PaymentStrategy, CheckoutResult } from './payment-strategy.interface';
import { Payment } from '../entities/payment.entity';

@Injectable()
export class StripeStrategy implements PaymentStrategy {
  private get secretKey() { return process.env.STRIPE_SECRET_KEY || ''; }
  private get webhookSecret() { return process.env.STRIPE_WEBHOOK_SECRET || ''; }

  async createCheckout(payment: Payment, options?: { success_url?: string; cancel_url?: string }): Promise<CheckoutResult> {
    if (!this.secretKey) throw new Error('Credit card payments are not available yet. Please use VietQR or Bank Transfer.');

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(this.secretKey);

    const session = await stripe.checkout.sessions.create({
      payment_method_types: ['card'],
      line_items: [{
        price_data: {
          currency: payment.currency.toLowerCase(),
          product_data: { name: `DraftRight Pro` },
          unit_amount: payment.amount,
        },
        quantity: 1,
      }],
      mode: 'payment',
      metadata: { reference_code: payment.reference_code, payment_id: payment.id },
      success_url: options?.success_url || `${process.env.WEBSITE_URL || 'http://localhost:4000'}/payment/success?ref=${payment.reference_code}`,
      cancel_url: options?.cancel_url || `${process.env.WEBSITE_URL || 'http://localhost:4000'}/payment/cancel`,
    });

    return {
      payment,
      redirect_url: session.url || undefined,
    };
  }

  async verifyWebhook(payload: any, headers: any): Promise<{ reference_code: string; status: 'completed' | 'failed' } | null> {
    if (!this.secretKey || !this.webhookSecret) return null;

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(this.secretKey);

    const sig = headers['stripe-signature'];
    let event;
    try {
      event = stripe.webhooks.constructEvent(payload, sig, this.webhookSecret);
    } catch {
      return null;
    }

    if (event.type === 'checkout.session.completed') {
      const session = event.data.object as any;
      return {
        reference_code: session.metadata?.reference_code,
        status: 'completed',
      };
    }

    return null;
  }
}
