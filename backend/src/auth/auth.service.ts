import { Injectable, UnauthorizedException, ConflictException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import * as bcrypt from 'bcrypt';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';

@Injectable()
export class AuthService {
  constructor(
    private readonly usersService: UsersService,
    private readonly jwtService: JwtService,
    private readonly plansService: PlansService,
    private readonly subscriptionsService: SubscriptionsService,
  ) {}

  private generateTokens(user: { id: string; email: string; role: string }) {
    const payload = { sub: user.id, email: user.email, role: user.role };

    const access_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_SECRET || 'dev-secret',
      expiresIn: '15m',
    });

    const refresh_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      expiresIn: '7d',
    });

    return { access_token, refresh_token };
  }

  async register(email: string, password: string, name: string) {
    const existing = await this.usersService.findByEmail(email);
    if (existing) throw new ConflictException('Email already registered');

    const password_hash = await bcrypt.hash(password, 10);
    const user = await this.usersService.create({ email, password_hash, name });

    const freePlan = await this.plansService.findFreePlan();
    await this.subscriptionsService.createFreeSubscription(user.id, freePlan.id);

    const tokens = this.generateTokens(user);
    return { ...tokens, user: { id: user.id, email: user.email, name: user.name } };
  }

  async login(email: string, password: string) {
    const user = await this.usersService.findByEmail(email);
    if (!user) throw new UnauthorizedException('Invalid credentials');

    const valid = await bcrypt.compare(password, user.password_hash);
    if (!valid) throw new UnauthorizedException('Invalid credentials');

    if (!user.is_active) throw new UnauthorizedException('Account disabled');

    const tokens = this.generateTokens(user);
    return { ...tokens, user: { id: user.id, email: user.email, name: user.name } };
  }

  async refresh(refreshToken: string) {
    try {
      const payload = this.jwtService.verify(refreshToken, {
        secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      });
      const user = await this.usersService.findById(payload.sub);
      if (!user || !user.is_active) throw new UnauthorizedException();
      return this.generateTokens(user);
    } catch {
      throw new UnauthorizedException('Invalid refresh token');
    }
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
