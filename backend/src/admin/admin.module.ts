import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { JwtModule } from '@nestjs/jwt';
import { AdminController } from './admin.controller';
import { AdminAuthController } from './admin-auth.controller';
import { AdminAuthService } from './admin-auth.service';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { AiProvidersModule } from '../ai-providers/ai-providers.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';
import { RewriteModule } from '../rewrite/rewrite.module';
import { AppSettings } from './entities/app-settings.entity';
import { AdminUser } from './entities/admin-user.entity';
import { EmailLog } from '../email/entities/email-log.entity';
import { PaymentModule } from '../payment/payment.module';
import { UpdatesModule } from '../updates/updates.module';
import { ErrorsModule } from '../errors/errors.module';
import { BugReportsModule } from '../bug-reports/bug-reports.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([AppSettings, AdminUser, EmailLog]),
    JwtModule.register({}),
    UsersModule,
    PlansModule,
    AiProvidersModule,
    SubscriptionsModule,
    UsageModule,
    RewriteModule,
    PaymentModule,
    UpdatesModule,
    ErrorsModule,
    BugReportsModule,
  ],
  controllers: [AdminController, AdminAuthController],
  providers: [AdminAuthService],
})
export class AdminModule {}
