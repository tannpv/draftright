import { Request } from 'express';
import * as jwt from 'jsonwebtoken';

/**
 * Decodes an optional Bearer JWT from the request's Authorization header.
 * Returns the user id (`sub` or `user_id` claim) or null — never throws,
 * so callers can treat a missing/invalid token as "anonymous".
 */
export function decodeOptionalUserId(req: Request): string | null {
  const h = req.headers['authorization'];
  if (typeof h !== 'string' || !h.startsWith('Bearer ')) return null;
  try {
    const decoded = jwt.verify(h.slice(7), process.env.JWT_SECRET || 'change_me') as {
      sub?: string; user_id?: string;
    };
    return decoded.sub || decoded.user_id || null;
  } catch {
    return null;
  }
}
