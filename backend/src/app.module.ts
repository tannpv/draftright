import { Module } from '@nestjs/common';
import { APP_GUARD } from '@nestjs/core';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ThrottlerGuard, ThrottlerModule } from '@nestjs/throttler';
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
import { ErrorsModule } from './errors/errors.module';
import { BugReportsModule } from './bug-reports/bug-reports.module';
import { ExtractionModule } from './extraction/extraction.module';

@Module({
  imports: [
    TypeOrmModule.forRoot(databaseConfig()),
    // Global per-IP rate limit, applied via APP_GUARD below. The anon endpoints
    // (/bug-reports, /feedback, /errors) further tighten this with @Throttle()
    // decorators. Multi-window definition lets us catch both burst spam
    // ("60 in 10 seconds") and slow drip spam ("1000 in an hour").
    ThrottlerModule.forRoot([
      { name: 'short',  ttl: 10_000,   limit: 60   }, // 60 req / 10 s
      { name: 'medium', ttl: 60_000,   limit: 200  }, // 200 req / min
      { name: 'long',   ttl: 3_600_000, limit: 2000 }, // 2000 req / hour
    ]),
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
    ErrorsModule,
    BugReportsModule,
    ExtractionModule,
  ],
  providers: [
    // Global guard so ThrottlerModule's limits apply to every controller
    // without per-controller @UseGuards. Overridden per-route by @Throttle()
    // on the anon endpoints, and bypassed by @SkipThrottle() on health.
    { provide: APP_GUARD, useClass: ThrottlerGuard },
  ],
})
export class AppModule {}
