import { Injectable, UnauthorizedException, ConflictException, BadRequestException, Logger } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as bcrypt from 'bcryptjs';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { EmailService } from '../email/email.service';
import { AuthProvider } from '../users/entities/user.entity';
import { AppSettings } from '../admin/entities/app-settings.entity';

@Injectable()
export class AuthService {
  private readonly logger = new Logger(AuthService.name);

  constructor(
    private readonly usersService: UsersService,
    private readonly jwtService: JwtService,
    private readonly plansService: PlansService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly emailService: EmailService,
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
  ) {}

  private async generateTokens(user: { id: string; email: string; role: string }) {
    const payload = { sub: user.id, email: user.email, role: user.role };

    // Token lifetimes are admin-configurable via AppSettings.
    // Defaults match historical behavior: 15min access, 90d refresh.
    const settings = await this.settingsRepo.findOne({ where: {} });
    const accessMinutes = settings?.token_expiry_minutes ?? 15;
    const refreshDays = settings?.refresh_token_expiry_days ?? 90;

    const access_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_SECRET || 'dev-secret',
      expiresIn: `${accessMinutes}m`,
    });

    const refresh_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      expiresIn: `${refreshDays}d`,
    });

    return { access_token, refresh_token };
  }

  async register(email: string, password: string, name: string) {
    const normalizedEmail = email.trim().toLowerCase();
    const existing = await this.usersService.findByEmail(normalizedEmail);
    if (existing) throw new ConflictException('Email already registered');

    const password_hash = await bcrypt.hash(password, 10);
    const code = this.generateVerificationCode();
    const expires = new Date(Date.now() + 15 * 60 * 1000);

    const user = await this.usersService.create({
      email: normalizedEmail,
      password_hash,
      name,
      email_verification_code: code,
      email_verification_expires: expires,
    });

    const freePlan = await this.plansService.findFreePlan();
    await this.subscriptionsService.createFreeSubscription(user.id, freePlan.id);

    // Fire-and-forget: don't block registration on email-provider latency or transient failures.
    this.emailService.sendVerificationEmail(normalizedEmail, name, code).catch((err) => {
      this.logger.error(`Failed to send verification email to ${normalizedEmail}: ${err.message}`);
    });

    const tokens = await this.generateTokens(user);
    return {
      ...tokens,
      user: { id: user.id, email: user.email, name: user.name, email_verified: false },
    };
  }

  async verifyEmail(email: string, code: string): Promise<{ success: true }> {
    const user = await this.usersService.findByEmail(email.trim().toLowerCase());
    if (!user || !user.email_verification_code || !user.email_verification_expires) {
      throw new BadRequestException('Invalid or expired verification code');
    }
    if (user.email_verification_code !== code) {
      throw new BadRequestException('Invalid or expired verification code');
    }
    if (user.email_verification_expires.getTime() < Date.now()) {
      throw new BadRequestException('Invalid or expired verification code');
    }
    await this.usersService.update(user.id, {
      email_verified: true,
      email_verification_code: null,
      email_verification_expires: null,
    });
    return { success: true };
  }

  async resendVerification(email: string): Promise<void> {
    // Silent for unknown emails — don't leak which addresses exist.
    const user = await this.usersService.findByEmail(email.trim().toLowerCase());
    if (!user || user.email_verified) return;

    const code = this.generateVerificationCode();
    const expires = new Date(Date.now() + 15 * 60 * 1000);
    await this.usersService.update(user.id, {
      email_verification_code: code,
      email_verification_expires: expires,
    });
    this.emailService.sendVerificationEmail(user.email, user.name, code).catch((err) => {
      this.logger.error(`Failed to resend verification to ${user.email}: ${err.message}`);
    });
  }

  private generateVerificationCode(): string {
    return Math.floor(100000 + Math.random() * 900000).toString();
  }

  async login(email: string, password: string) {
    const user = await this.usersService.findByEmail(email);
    if (!user) throw new UnauthorizedException('Invalid credentials');

    const valid = await bcrypt.compare(password, user.password_hash);
    if (!valid) throw new UnauthorizedException('Invalid credentials');

    if (!user.is_active) throw new UnauthorizedException('Account disabled');

    const tokens = await this.generateTokens(user);
    return { ...tokens, user: { id: user.id, email: user.email, name: user.name } };
  }

  async refresh(refreshToken: string) {
    try {
      const payload = this.jwtService.verify(refreshToken, {
        secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      });
      const user = await this.usersService.findById(payload.sub);
      if (!user || !user.is_active) throw new UnauthorizedException();
      return await this.generateTokens(user);
    } catch {
      throw new UnauthorizedException('Invalid refresh token');
    }
  }

  async socialLogin(provider: string, idToken: string, profile: { name?: string; email?: string; avatar_url?: string }) {
    const providerEnum = this.toAuthProvider(provider);
    const socialProfile = await this.verifySocialToken(providerEnum, idToken, profile);

    if (!socialProfile.email) {
      throw new BadRequestException('Email is required for social login');
    }

    // Look up by social ID first
    let user = await this.usersService.findBySocialId(providerEnum, socialProfile.socialId);

    if (!user) {
      // Check if email already exists (link accounts)
      user = await this.usersService.findByEmail(socialProfile.email);
      if (user) {
        // Link social ID to existing account
        const socialIdColumn = `${provider}_id`;
        await this.usersService.update(user.id, {
          [socialIdColumn]: socialProfile.socialId,
          avatar_url: socialProfile.avatar_url || user.avatar_url,
        } as any);
        user = await this.usersService.findById(user.id);
      } else {
        // Create new user
        const socialIdColumn = `${provider}_id`;
        user = await this.usersService.create({
          email: socialProfile.email,
          name: socialProfile.name || socialProfile.email.split('@')[0],
          auth_provider: providerEnum,
          [socialIdColumn]: socialProfile.socialId,
          avatar_url: socialProfile.avatar_url,
        } as any);

        // Assign free plan
        const freePlan = await this.plansService.findFreePlan();
        await this.subscriptionsService.createFreeSubscription(user.id, freePlan.id);
      }
    }

    if (!user || !user.is_active) throw new UnauthorizedException('Account disabled');

    const tokens = await this.generateTokens(user!);
    return { ...tokens, user: { id: user!.id, email: user!.email, name: user!.name } };
  }

  private toAuthProvider(provider: string): AuthProvider {
    switch (provider.toLowerCase()) {
      case 'google': return AuthProvider.GOOGLE;
      case 'facebook': return AuthProvider.FACEBOOK;
      case 'tiktok': return AuthProvider.TIKTOK;
      default: throw new BadRequestException(`Unsupported provider: ${provider}`);
    }
  }

  private async verifySocialToken(
    provider: AuthProvider,
    idToken: string,
    profile: { name?: string; email?: string; avatar_url?: string },
  ): Promise<{ socialId: string; email: string; name?: string; avatar_url?: string }> {
    switch (provider) {
      case AuthProvider.GOOGLE:
        return this.verifyGoogleToken(idToken);
      case AuthProvider.FACEBOOK:
        return this.verifyFacebookToken(idToken);
      case AuthProvider.TIKTOK:
        // TikTok doesn't have a simple ID token verification;
        // the Flutter SDK provides user info directly, so we trust the access token + profile
        return {
          socialId: idToken, // TikTok open_id passed as idToken
          email: profile.email || '',
          name: profile.name,
          avatar_url: profile.avatar_url,
        };
      default:
        throw new BadRequestException(`Unsupported provider`);
    }
  }

  private async verifyGoogleToken(idToken: string): Promise<{ socialId: string; email: string; name?: string; avatar_url?: string }> {
    // Verify with Google's tokeninfo endpoint
    const response = await fetch(`https://oauth2.googleapis.com/tokeninfo?id_token=${idToken}`);
    if (!response.ok) throw new UnauthorizedException('Invalid Google token');
    const data = await response.json();
    return {
      socialId: data.sub,
      email: data.email,
      name: data.name,
      avatar_url: data.picture,
    };
  }

  private async verifyFacebookToken(accessToken: string): Promise<{ socialId: string; email: string; name?: string; avatar_url?: string }> {
    // Verify with Facebook's Graph API
    const response = await fetch(`https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=${accessToken}`);
    if (!response.ok) throw new UnauthorizedException('Invalid Facebook token');
    const data = await response.json();
    return {
      socialId: data.id,
      email: data.email,
      name: data.name,
      avatar_url: data.picture?.data?.url,
    };
  }

  async changePassword(userId: string, currentPassword: string, newPassword: string) {
    const user = await this.usersService.findById(userId);
    if (!user) throw new UnauthorizedException();

    const valid = await bcrypt.compare(currentPassword, user.password_hash);
    if (!valid) throw new UnauthorizedException('Current password is incorrect');

    const password_hash = await bcrypt.hash(newPassword, 10);
    await this.usersService.update(userId, { password_hash });
    return { success: true };
  }
}
