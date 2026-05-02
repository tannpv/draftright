import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { PaymentController } from './payment.controller';
import { PaymentService } from './payment.service';
import { StripeStrategy } from './strategies/stripe.strategy';
import { PayPalStrategy } from './strategies/paypal.strategy';
import { VietQRStrategy } from './strategies/vietqr.strategy';
import { MomoStrategy } from './strategies/momo.strategy';
import { Payment } from './entities/payment.entity';
import { PlansModule } from '../plans/plans.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([Payment]),
    PlansModule,
    SubscriptionsModule,
  ],
  controllers: [PaymentController],
  providers: [PaymentService, StripeStrategy, PayPalStrategy, VietQRStrategy, MomoStrategy],
  exports: [PaymentService],
})
export class PaymentModule {}
