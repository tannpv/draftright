import { Injectable } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Request } from 'express';
import * as jwt from 'jsonwebtoken';
import { EnvSchema } from '../config/env.schema';

/**
 * Decodes an optional Bearer JWT from a request and returns the user
 * id (`sub` or `user_id` claim) — or `null` for "anonymous" callers.
 *
 * Never throws: a missing / malformed / expired token is treated the
 * same as no token, so anonymous endpoints (bug-reports, feedback
 * board) can attach an authenticated row when possible without
 * forcing callers to be logged in.
 *
 * Class-shaped so the secret comes from the typed ConfigService
 * (S14) instead of a `process.env.JWT_SECRET` read with a
 * dangerous `'change_me'` fallback (former pattern).
 */
@Injectable()
export class JwtUserService {
  constructor(private readonly cfg: ConfigService<EnvSchema, true>) {}

  /**
   * Returns the user id encoded in the request's Bearer token, or
   * null when no valid token is present.
   */
  decodeOptional(req: Request): string | null {
    const h = req.headers['authorization'];
    if (typeof h !== 'string' || !h.startsWith('Bearer ')) return null;
    try {
      const decoded = jwt.verify(
        h.slice(7),
        this.cfg.get('JWT_SECRET', { infer: true }),
      ) as { sub?: string; user_id?: string };
      return decoded.sub || decoded.user_id || null;
    } catch {
      return null;
    }
  }
}
