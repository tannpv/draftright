import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { PaymentController } from './payment.controller';
import { PaymentService } from './payment.service';
import { StripeStrategy } from './strategies/stripe.strategy';
import { VietQRStrategy } from './strategies/vietqr.strategy';
import { LemonSqueezyStrategy } from './strategies/lemonsqueezy.strategy';
import { Payment } from './entities/payment.entity';
import { User } from '../users/entities/user.entity';
import { AppSettings } from '../admin/entities/app-settings.entity';
import { PlansModule } from '../plans/plans.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [
    /**
     * AppSettings is registered here so each strategy can read live admin-set
     * credentials (sepay_api_key, stripe_secret_key, etc.) instead of forcing
     * a backend redeploy when the admin rotates a key from the Settings UI.
     * User is registered so PaymentService can read/write stripe_customer_id.
     */
    TypeOrmModule.forFeature([Payment, User, AppSettings]),
    PlansModule,
    SubscriptionsModule,
  ],
  controllers: [PaymentController],
  providers: [PaymentService, StripeStrategy, VietQRStrategy, LemonSqueezyStrategy],
  exports: [PaymentService],
})
export class PaymentModule {}
