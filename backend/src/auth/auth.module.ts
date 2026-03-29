import { Module } from '@nestjs/common';
import { JwtModule } from '@nestjs/jwt';
import { PassportModule } from '@nestjs/passport';
import { AuthService } from './auth.service';
import { AuthController } from './auth.controller';
import { JwtStrategy } from './jwt.strategy';
import { UsersModule } from '../users/users.module';
import { PlansModule } from '../plans/plans.module';
import { SubscriptionsModule } from '../subscriptions/subscriptions.module';

@Module({
  imports: [
    PassportModule,
    JwtModule.register({}),
    UsersModule,
    PlansModule,
    SubscriptionsModule,
  ],
  controllers: [AuthController],
  providers: [AuthService, JwtStrategy],
  exports: [AuthService, JwtStrategy],
})
export class AuthModule {}
