import { Module } from '@nestjs/common';
import { AdminController } from './admin.controller';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';

@Module({
  imports: [UsersModule, PlansModule, AiProvidersModule, SubscriptionsModule, UsageModule],
  controllers: [AdminController],
})
export class AdminModule {}
