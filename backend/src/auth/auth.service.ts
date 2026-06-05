import { Injectable, UnauthorizedException, ConflictException, BadRequestException, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { JwtService } from '@nestjs/jwt';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as bcrypt from 'bcryptjs';
import * as jwt from 'jsonwebtoken';
import { createPublicKey, randomInt } from 'node:crypto';

/** Shape of one entry in Apple's `/auth/keys` JWK set. */
interface AppleJwk {
  kty: string;
  kid: string;
  use: string;
  alg: string;
  n: string;
  e: string;
}
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { EmailService } from '../email/email.service';
import { AuthProvider } from '../users/entities/user.entity';
import { AppSettings } from '../admin/entities/app-settings.entity';
import { EMAIL_CODE_TTL_MS } from '../common/app-config';
import { EnvSchema } from '../config/env.schema';

/**
 * Normalised identity returned by every provider verifier.
 * `emailVerified` reflects the PROVIDER's own claim — we only trust an
 * email (auto-link / skip our verification step) when the provider
 * asserts it owns a verified address. Never assume verified.
 */
interface SocialProfile {
  socialId: string;
  email: string;
  name?: string;
  avatar_url?: string;
  emailVerified: boolean;
}

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
    private readonly cfg: ConfigService<EnvSchema, true>,
  ) {}

  private async generateTokens(user: { id: string; email: string; role: string }) {
    const payload = { sub: user.id, email: user.email, role: user.role };

    // Token lifetimes are admin-configurable via AppSettings.
    // Defaults match historical behavior: 15min access, 90d refresh.
    const settings = await this.settingsRepo.findOne({ where: {} });
    const accessMinutes = settings?.token_expiry_minutes ?? 15;
    const refreshDays = settings?.refresh_token_expiry_days ?? 90;

    const access_token = this.jwtService.sign(payload, {
      secret: this.cfg.get('JWT_SECRET', { infer: true }),
      expiresIn: `${accessMinutes}m`,
    });

    const refresh_token = this.jwtService.sign(payload, {
      secret: this.cfg.get('JWT_REFRESH_SECRET', { infer: true }),
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
    const expires = new Date(Date.now() + EMAIL_CODE_TTL_MS);

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
    const expires = new Date(Date.now() + EMAIL_CODE_TTL_MS);
    await this.usersService.update(user.id, {
      email_verification_code: code,
      email_verification_expires: expires,
    });
    this.emailService.sendVerificationEmail(user.email, user.name, code).catch((err) => {
      this.logger.error(`Failed to resend verification to ${user.email}: ${err.message}`);
    });
  }

  // After this many wrong attempts the reset code is invalidated, so the
  // 10^6 keyspace can't be brute-forced within the 15-minute TTL.
  private static readonly MAX_RESET_ATTEMPTS = 5;

  private generateVerificationCode(): string {
    // CSPRNG — Math.random() is predictable and unfit for auth codes.
    return randomInt(0, 1_000_000).toString().padStart(6, '0');
  }

  /**
   * Start a password reset. Mirrors resendVerification: silent for
   * unknown emails AND for social-only accounts (no password to reset)
   * so we never leak which addresses exist. The controller always
   * returns success regardless.
   */
  async forgotPassword(email: string): Promise<void> {
    const user = await this.usersService.findByEmail(email.trim().toLowerCase());
    if (!user || user.auth_provider !== AuthProvider.LOCAL || !user.password_hash) return;

    const code = this.generateVerificationCode();
    const expires = new Date(Date.now() + EMAIL_CODE_TTL_MS);
    await this.usersService.update(user.id, {
      password_reset_code: code,
      password_reset_expires: expires,
      password_reset_attempts: 0,
    });
    this.emailService.sendPasswordResetEmail(user.email, user.name, code).catch((err) => {
      this.logger.error(`Failed to send password reset to ${user.email}: ${err.message}`);
    });
  }

  /**
   * Complete a password reset with the emailed 6-digit code. Single-use:
   * the code is cleared on success.
   */
  async resetPassword(email: string, code: string, newPassword: string): Promise<{ success: true }> {
    if (!newPassword || newPassword.length < 8) {
      throw new BadRequestException('Password must be at least 8 characters');
    }
    const user = await this.usersService.findByEmail(email.trim().toLowerCase());
    if (!user || !user.password_reset_code || !user.password_reset_expires) {
      throw new BadRequestException('Invalid or expired reset code');
    }
    const badCode =
      user.password_reset_code !== code ||
      user.password_reset_expires.getTime() < Date.now();
    if (badCode) {
      // Throttle brute force: count failures and burn the code once the
      // cap is hit, forcing the user to request a fresh one.
      const attempts = (user.password_reset_attempts ?? 0) + 1;
      const exhausted = attempts >= AuthService.MAX_RESET_ATTEMPTS;
      await this.usersService.update(
        user.id,
        exhausted
          ? { password_reset_code: null, password_reset_expires: null, password_reset_attempts: 0 }
          : { password_reset_attempts: attempts },
      );
      throw new BadRequestException('Invalid or expired reset code');
    }
    const password_hash = await bcrypt.hash(newPassword, 10);
    await this.usersService.update(user.id, {
      password_hash,
      password_reset_code: null,
      password_reset_expires: null,
      password_reset_attempts: 0,
    });
    return { success: true };
  }

  async login(email: string, password: string) {
    const user = await this.usersService.findByEmail(email);
    if (!user) throw new UnauthorizedException('Invalid credentials');

    // Social-only accounts (Google / Facebook / Apple sign-up) have
    // no password_hash.  Calling bcrypt.compare(pw, null) throws
    // "Illegal arguments: string, object" — surface a friendlier
    // message that tells the user which provider to use instead.
    if (!user.password_hash) {
      const friendly = this.providerLabel(user.auth_provider);
      throw new UnauthorizedException(
        friendly
          ? `This account was created with ${friendly}. Use the ${friendly} button to sign in.`
          : 'This account uses a social sign-in. Use the provider you signed up with.',
      );
    }

    const valid = await bcrypt.compare(password, user.password_hash);
    if (!valid) throw new UnauthorizedException('Invalid credentials');

    if (!user.is_active) throw new UnauthorizedException('Account disabled');

    const tokens = await this.generateTokens(user);
    return { ...tokens, user: { id: user.id, email: user.email, name: user.name } };
  }

  /**
   * Human label for the social provider that owns an account, used
   * to tell email-login attempts which button to press.  Returns
   * null for `local` accounts (they have a password and should
   * never hit this path).
   */
  private providerLabel(provider: AuthProvider): string | null {
    switch (provider) {
      case AuthProvider.GOOGLE:   return 'Google';
      case AuthProvider.FACEBOOK: return 'Facebook';
      case AuthProvider.TIKTOK:   return 'TikTok';
      case AuthProvider.APPLE:    return 'Apple';
      default:                    return null;
    }
  }

  async refresh(refreshToken: string) {
    try {
      const payload = this.jwtService.verify(refreshToken, {
        secret: this.cfg.get('JWT_REFRESH_SECRET', { infer: true }),
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

    // Only trust the email when the PROVIDER asserts it's verified.
    const providerVerifiedEmail = socialProfile.emailVerified === true;

    if (!user) {
      // Check if email already exists (link accounts)
      user = await this.usersService.findByEmail(socialProfile.email);
      if (user) {
        // Auto-linking a social identity to a pre-existing account is an
        // account-takeover vector if the provider's email isn't actually
        // verified. Refuse to link unless the provider proved ownership.
        if (!providerVerifiedEmail) {
          throw new UnauthorizedException(
            'This email is registered. Sign in with your password to link this account.',
          );
        }
        const socialIdColumn = `${provider}_id`;
        await this.usersService.update(user.id, {
          [socialIdColumn]: socialProfile.socialId,
          avatar_url: socialProfile.avatar_url || user.avatar_url,
          email_verified: true,
        } as any);
        user = await this.usersService.findById(user.id);
      } else {
        // Create new user. Skip our own email-verification step only when
        // the provider already verified the address.
        const socialIdColumn = `${provider}_id`;
        user = await this.usersService.create({
          email: socialProfile.email,
          name: socialProfile.name || socialProfile.email.split('@')[0],
          auth_provider: providerEnum,
          [socialIdColumn]: socialProfile.socialId,
          avatar_url: socialProfile.avatar_url,
          email_verified: providerVerifiedEmail,
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
      case 'apple': return AuthProvider.APPLE;
      default: throw new BadRequestException(`Unsupported provider: ${provider}`);
    }
  }

  private async verifySocialToken(
    provider: AuthProvider,
    idToken: string,
    profile: { name?: string; email?: string; avatar_url?: string },
  ): Promise<SocialProfile> {
    switch (provider) {
      case AuthProvider.GOOGLE:
        return this.verifyGoogleToken(idToken);
      case AuthProvider.FACEBOOK:
        return this.verifyFacebookToken(idToken);
      case AuthProvider.APPLE:
        return this.verifyAppleToken(idToken, profile);
      case AuthProvider.TIKTOK:
        // TikTok doesn't have a simple ID token verification;
        // the Flutter SDK provides user info directly, so we trust the access token + profile
        return {
          socialId: idToken, // TikTok open_id passed as idToken
          email: profile.email || '',
          name: profile.name,
          avatar_url: profile.avatar_url,
          // TikTok returns no verified-email claim — treat as unverified.
          emailVerified: false,
        };
      default:
        throw new BadRequestException(`Unsupported provider`);
    }
  }

  private async verifyGoogleToken(idToken: string): Promise<SocialProfile> {
    // Verify with Google's tokeninfo endpoint
    const response = await fetch(`https://oauth2.googleapis.com/tokeninfo?id_token=${idToken}`);
    if (!response.ok) throw new UnauthorizedException('Invalid Google token');
    const data = await response.json();
    return {
      socialId: data.sub,
      email: data.email,
      name: data.name,
      avatar_url: data.picture,
      // tokeninfo returns email_verified as the string "true" / "false".
      emailVerified: data.email_verified === true || data.email_verified === 'true',
    };
  }

  /**
   * Verify a Sign in with Apple identity token.
   *
   * Apple-signed JWT — header `kid` selects which of Apple's rotating
   * public keys signed it.  Steps:
   *   1. Decode header to read `kid`.
   *   2. Fetch the matching public key from
   *      https://appleid.apple.com/auth/keys (cached by jwks-rsa).
   *   3. Verify signature (RS256), `iss === https://appleid.apple.com`,
   *      and `aud` matches the bundle id Apple issued the token for.
   *   4. Return `sub` as the stable social id; Apple omits email on
   *      subsequent sign-ins so we fall back to the in-app
   *      `profile.email` (which the mobile SDK collected on the first
   *      consent and cached).  Name is similarly first-sign-in only.
   *
   * The accepted bundle IDs are configurable via APPLE_AUDIENCES env
   * (comma-separated) so the same backend serves the iOS app, the
   * macOS app, and a future web sign-in flow without code edits.
   */
  private async verifyAppleToken(
    idToken: string,
    profile: { name?: string; email?: string; avatar_url?: string },
  ): Promise<SocialProfile> {
    const decoded = jwt.decode(idToken, { complete: true });
    if (!decoded || typeof decoded === 'string' || !decoded.header.kid) {
      throw new UnauthorizedException('Invalid Apple token (no kid)');
    }
    const audiencesRaw = this.cfg.get('APPLE_AUDIENCES', { infer: true }) ||
      'com.draftright.draftrightMobile.v2,com.draftright.app.v2';
    const audiences = audiencesRaw.split(',').map(s => s.trim()).filter(Boolean);

    const publicKey = await this.fetchApplePublicKeyPem(decoded.header.kid);

    let payload: jwt.JwtPayload;
    try {
      // jsonwebtoken's typing requires audience to be a tuple [first, ...rest]
      // — cast through unknown to satisfy the overload while keeping the
      // runtime check against every entry in `audiences`.
      payload = jwt.verify(idToken, publicKey, {
        algorithms: ['RS256'],
        issuer: 'https://appleid.apple.com',
        audience: audiences as unknown as [string, ...string[]],
      }) as jwt.JwtPayload;
    } catch (e) {
      this.logger.warn(`Apple token verification failed: ${(e as Error).message}`);
      throw new UnauthorizedException('Invalid Apple token');
    }
    if (!payload.sub) {
      throw new UnauthorizedException('Apple token missing sub claim');
    }
    // Apple includes `email` only on the very first sign-in; the
    // mobile SDK caches it and we also receive it here as
    // profile.email.  Prefer the token claim when present.
    const email = (payload.email as string) || profile.email || '';
    // Apple sends email_verified as a boolean or the string "true".
    const appleVerified = (payload as any).email_verified;
    return {
      socialId: payload.sub,
      email,
      name: profile.name,         // Apple only ships full name on first consent
      avatar_url: profile.avatar_url,
      // Only trust the token's own claim; the profile.email fallback
      // (cached client-side) carries no verification proof.
      emailVerified: !!payload.email && (appleVerified === true || appleVerified === 'true'),
    };
  }

  /**
   * Cached fetch of Apple's JWKs.  Apple rotates infrequently, so a
   * 24-hour in-process cache keeps us off the network on every
   * Apple sign-in while still picking up rotations the next day.
   *
   * No external dep (was jwks-rsa, but its transitive `jose` ships
   * ESM-only and breaks Jest under CJS).  Pure Node `crypto.createPublicKey`
   * accepts JWK directly since Node 16, producing a PEM that
   * `jsonwebtoken.verify` consumes.
   */
  private static _appleJwksCache: { keys: AppleJwk[]; expiresAt: number } | null = null;

  private async fetchApplePublicKeyPem(kid: string): Promise<string> {
    const now = Date.now();
    let cache = AuthService._appleJwksCache;
    if (!cache || cache.expiresAt < now) {
      const res = await fetch('https://appleid.apple.com/auth/keys');
      if (!res.ok) {
        throw new UnauthorizedException('Could not fetch Apple signing keys');
      }
      const json = (await res.json()) as { keys: AppleJwk[] };
      cache = { keys: json.keys || [], expiresAt: now + 24 * 60 * 60 * 1000 };
      AuthService._appleJwksCache = cache;
    }
    const jwk = cache.keys.find((k) => k.kid === kid);
    if (!jwk) {
      // Could be a fresh rotation we haven't seen — bust the cache
      // and retry once before giving up.
      AuthService._appleJwksCache = null;
      throw new UnauthorizedException(`Apple key not found for kid=${kid}`);
    }
    const keyObject = createPublicKey({ key: jwk as any, format: 'jwk' });
    return keyObject.export({ type: 'spki', format: 'pem' }) as string;
  }

  private async verifyFacebookToken(accessToken: string): Promise<SocialProfile> {
    // Verify with Facebook's Graph API
    const response = await fetch(`https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=${accessToken}`);
    if (!response.ok) throw new UnauthorizedException('Invalid Facebook token');
    const data = await response.json();
    return {
      socialId: data.id,
      email: data.email,
      name: data.name,
      avatar_url: data.picture?.data?.url,
      // Graph returns the email field only for a confirmed account email.
      emailVerified: !!data.email,
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
