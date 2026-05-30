import { Controller, Post, Body, UseGuards, Request, Get, Delete, HttpCode, HttpStatus } from '@nestjs/common';
import { ApiTags } from '@nestjs/swagger';
import { AuthService } from './auth.service';
import { RegisterDto } from './dto/register.dto';
import { LoginDto } from './dto/login.dto';
import { JwtAuthGuard } from './jwt-auth.guard';
import { FeatureFlagsService } from './feature-flags.service';
import { UsersService } from '../users/users.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';

@ApiTags('auth')
@Controller('auth')
export class AuthController {
  constructor(
    private readonly authService: AuthService,
    private readonly usersService: UsersService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
    private readonly featureFlags: FeatureFlagsService,
  ) {}

  @Post('register')
  async register(@Body() dto: RegisterDto) {
    return this.authService.register(dto.email, dto.password, dto.name);
  }

  @Post('login')
  async login(@Body() dto: LoginDto) {
    return this.authService.login(dto.email, dto.password);
  }

  @Post('refresh')
  @HttpCode(HttpStatus.OK)
  async refresh(@Body() body: { refresh_token: string }) {
    return this.authService.refresh(body.refresh_token);
  }

  @Post('verify-email')
  @HttpCode(HttpStatus.OK)
  async verifyEmail(@Body() body: { email: string; code: string }) {
    return this.authService.verifyEmail(body.email, body.code);
  }

  @Post('resend-verification')
  @HttpCode(HttpStatus.OK)
  async resendVerification(@Body() body: { email: string }) {
    await this.authService.resendVerification(body.email);
    return { success: true };
  }

  @Post('social')
  async socialLogin(@Body() body: { provider: string; id_token: string; name?: string; email?: string; avatar_url?: string }) {
    return this.authService.socialLogin(body.provider, body.id_token, {
      name: body.name,
      email: body.email,
      avatar_url: body.avatar_url,
    });
  }

  @UseGuards(JwtAuthGuard)
  @Post('change-password')
  async changePassword(@Request() req: any, @Body() body: { current_password: string; new_password: string }) {
    return this.authService.changePassword(req.user.id, body.current_password, body.new_password);
  }

  @UseGuards(JwtAuthGuard)
  @Get('me')
  async me(@Request() req: any) {
    // `flags` is the server-controlled rollout state for the calling
    // user. Clients poll this on login + on periodic refresh + react
    // to it without a rebuild — operations bump GO_BACKEND_RAMP_PERCENT
    // and the next /auth/me round-trip picks it up.  See
    // FeatureFlagsService for the bucket algorithm.
    return {
      id: req.user.id,
      email: req.user.email,
      role: req.user.role,
      flags: {
        use_go_backend: this.featureFlags.useGoBackend(req.user.id),
      },
    };
  }

  // Permanent, in-app account deletion (App Store Guideline 5.1.1(v)).
  // Removes the user and all data tied to them; no recovery.
  @UseGuards(JwtAuthGuard)
  @Delete('account')
  @HttpCode(HttpStatus.OK)
  async deleteAccount(@Request() req: any) {
    await this.usersService.deleteAccount(req.user.id);
    return { deleted: true };
  }

  @UseGuards(JwtAuthGuard)
  @Get('account')
  async account(@Request() req: any) {
    const user = await this.usersService.findById(req.user.id);
    if (!user) return null;

    const sub = await this.subscriptionsService.findActiveByUserId(req.user.id);
    const usageToday = await this.usageService.countTodayByUser(req.user.id);

    return {
      id: user.id,
      email: user.email,
      name: user.name,
      email_verified: user.email_verified,
      has_lemonsqueezy_customer: !!user.lemonsqueezy_customer_id,
      subscription: sub
        ? {
            plan_name: sub.plan?.name ?? 'Unknown',
            status: sub.status,
            store_type: sub.store_type,
            started_at: sub.started_at,
            expires_at: sub.expires_at,
            daily_limit: sub.plan?.daily_limit ?? 0,
            usage_today: usageToday,
          }
        : null,
    };
  }
}
