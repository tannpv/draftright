import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ScheduleModule } from '@nestjs/schedule';
import { Subscription } from './entities/subscription.entity';
import { SubscriptionsService } from './subscriptions.service';
import { SubscriptionsController } from './subscriptions.controller';
import { SubscriptionsCron } from './subscriptions.cron';
import { UsageModule } from '../usage/usage.module';
import { EmailService } from '../email/email.service';

@Module({
  imports: [
    TypeOrmModule.forFeature([Subscription]),
    ScheduleModule.forRoot(), // safe to call multiple times across modules
    UsageModule,
  ],
  controllers: [SubscriptionsController],
  // EmailService is light enough to construct here without a full EmailModule import.
  // If/when EmailModule grows (templates, queues), refactor to import the module.
  providers: [SubscriptionsService, SubscriptionsCron, EmailService],
  exports: [SubscriptionsService],
})
export class SubscriptionsModule {}
