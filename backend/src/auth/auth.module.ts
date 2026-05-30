import { Module } from '@nestjs/common';
import { JwtModule } from '@nestjs/jwt';
import { PassportModule } from '@nestjs/passport';
import { TypeOrmModule } from '@nestjs/typeorm';
import { AuthService } from './auth.service';
import { AuthController } from './auth.controller';
import { JwtStrategy } from './jwt.strategy';
import { JwtAuthGuard } from './jwt-auth.guard';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';
import { UsageModule } from '../usage/usage.module';
import { ExtensionToken } from './extension-token.entity';
import { ExtensionTokenService } from './extension-token.service';
import { ExtensionTokenController } from './extension-token.controller';
import { RewriteAuthGuard } from './rewrite-auth.guard';
import { FeatureFlagsService } from './feature-flags.service';
import { AppSettings } from '../admin/entities/app-settings.entity';

@Module({
  imports: [
    PassportModule,
    JwtModule.register({}),
    TypeOrmModule.forFeature([ExtensionToken, AppSettings]),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
    UsageModule,
  ],
  controllers: [AuthController, ExtensionTokenController],
  providers: [
    AuthService,
    JwtStrategy,
    JwtAuthGuard,
    ExtensionTokenService,
    RewriteAuthGuard,
    FeatureFlagsService,
  ],
  exports: [
    AuthService,
    JwtStrategy,
    JwtAuthGuard,
    ExtensionTokenService,
    RewriteAuthGuard,
    FeatureFlagsService,
  ],
})
export class AuthModule {}
