import { Injectable, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as bcrypt from 'bcryptjs';
import { AdminUser } from './entities/admin-user.entity';

@Injectable()
export class AdminAuthService {
  constructor(
    @InjectRepository(AdminUser)
    private readonly adminUserRepo: Repository<AdminUser>,
    private readonly jwtService: JwtService,
  ) {}

  private generateTokens(admin: AdminUser) {
    const payload = { sub: admin.id, email: admin.email, role: admin.role, isAdmin: true };

    const access_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_SECRET || 'dev-secret',
      expiresIn: process.env.NODE_ENV === 'production' ? '15m' : '24h',
    });

    const refresh_token = this.jwtService.sign(payload, {
      secret: process.env.JWT_REFRESH_SECRET || 'dev-refresh-secret',
      expiresIn: '7d',
    });

    return { access_token, refresh_token };
  }

  async login(email: string, password: string) {
    const admin = await this.adminUserRepo
      .createQueryBuilder('a')
      .where('LOWER(a.email) = LOWER(:email)', { email: email.trim() })
      .getOne();
    if (!admin) throw new UnauthorizedException('Invalid credentials');

    const valid = await bcrypt.compare(password, admin.password_hash);
    if (!valid) throw new UnauthorizedException('Invalid credentials');

    if (!admin.is_active) throw new UnauthorizedException('Account disabled');

    const tokens = this.generateTokens(admin);
    return { ...tokens, user: { id: admin.id, email: admin.email, name: admin.name, role: admin.role } };
  }

  async changePassword(adminId: string, currentPassword: string, newPassword: string) {
    const admin = await this.adminUserRepo.findOne({ where: { id: adminId } });
    if (!admin) throw new UnauthorizedException();

    const valid = await bcrypt.compare(currentPassword, admin.password_hash);
    if (!valid) throw new UnauthorizedException('Current password is incorrect');

    const password_hash = await bcrypt.hash(newPassword, 10);
    await this.adminUserRepo.update(adminId, { password_hash });
    return { success: true };
  }

  async getProfile(adminId: string) {
    const admin = await this.adminUserRepo.findOne({ where: { id: adminId } });
    if (!admin) throw new UnauthorizedException();

    const { password_hash, ...profile } = admin;
    return profile;
  }
}
