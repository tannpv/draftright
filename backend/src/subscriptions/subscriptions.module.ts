import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ScheduleModule } from '@nestjs/schedule';
import { Subscription } from './entities/subscription.entity';
import { SubscriptionsService } from './subscriptions.service';
import { SubscriptionsController } from './subscriptions.controller';
import { SubscriptionsCron } from './subscriptions.cron';
import { UsageModule } from '../usage/usage.module';
import { PlansModule } from '../plans/plans.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([Subscription]),
    ScheduleModule.forRoot(), // safe to call multiple times across modules
    UsageModule,
    PlansModule,
    // EmailService comes from the global EmailModule registered in app.module.
  ],
  controllers: [SubscriptionsController],
  providers: [SubscriptionsService, SubscriptionsCron],
  exports: [SubscriptionsService],
})
export class SubscriptionsModule {}
