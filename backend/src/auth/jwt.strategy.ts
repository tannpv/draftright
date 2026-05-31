import { Injectable } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { PassportStrategy } from '@nestjs/passport';
import { ExtractJwt, Strategy } from 'passport-jwt';
import { EnvSchema } from '../config/env.schema';

@Injectable()
export class JwtStrategy extends PassportStrategy(Strategy) {
  // Standard S14 — env reads go through the typed ConfigService.
  // The Zod schema already guarantees JWT_SECRET is present and ≥16
  // chars at boot, so no need for a `'dev-secret'` fallback that
  // would silently let a misconfigured prod accept any token.
  constructor(cfg: ConfigService<EnvSchema, true>) {
    super({
      jwtFromRequest: ExtractJwt.fromAuthHeaderAsBearerToken(),
      ignoreExpiration: false,
      secretOrKey: cfg.get('JWT_SECRET', { infer: true }),
    });
  }

  async validate(payload: { sub: string; email: string; role: string; isAdmin?: boolean }) {
    return { id: payload.sub, email: payload.email, role: payload.role, isAdmin: payload.isAdmin || false };
  }
}
