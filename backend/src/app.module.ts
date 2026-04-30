import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { databaseConfig } from './config/database.config';
import { HealthModule } from './health/health.module';
import { UsersModule } from './users/users.module';
import { PlansModule } from './plans/plans.module';
import { SubscriptionsModule } from './subscriptions/subscriptions.module';
import { AiProvidersModule } from './ai-providers/ai-providers.module';
import { AuthModule } from './auth/auth.module';
import { UsageModule } from './usage/usage.module';
import { RewriteModule } from './rewrite/rewrite.module';
import { AdminModule } from './admin/admin.module';
import { PaymentModule } from './payment/payment.module';
import { UpdatesModule } from './updates/updates.module';
import { EmailModule } from './email/email.module';
import { LemonsqueezyModule } from './lemonsqueezy/lemonsqueezy.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    EmailModule,
    HealthModule,
    UsersModule,
    PlansModule,
    SubscriptionsModule,
    AiProvidersModule,
    AuthModule,
    UsageModule,
    RewriteModule,
    AdminModule,
    PaymentModule,
    UpdatesModule,
    LemonsqueezyModule,
  ],
})
export class AppModule {}
