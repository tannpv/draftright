import { Module } from '@nestjs/common';
import { LemonsqueezyService } from './lemonsqueezy.service';
import { LemonsqueezyController } from './lemonsqueezy.controller';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { AuthModule } from '../auth/auth.module';

@Module({
  imports: [UsersModule, PlansModule, SubscriptionsModule, AuthModule],
  controllers: [LemonsqueezyController],
  providers: [LemonsqueezyService],
  exports: [LemonsqueezyService],
})
export class LemonsqueezyModule {}
