import { Injectable, NestMiddleware } from '@nestjs/common';
import { Request, Response, NextFunction } from 'express';
import { randomUUID } from 'crypto';

/**
 * Stamps every request with an `X-Request-Id` that:
 *   - reflects whatever Caddy / reverse-proxy forwarded (when present),
 *     so a single id traces across edge → backend → Go service;
 *   - falls back to a fresh UUIDv4 when none arrived;
 *   - lands on `req.requestId` for downstream handlers + logs;
 *   - echoes back on the response so the client sees the id it can
 *     paste into a support ticket.
 *
 * This is the cross-service correlation seam recommended in the
 * architecture review.  The Go /rewrite service already plumbs this id
 * via chi RequestID; the matching middleware here closes the loop on
 * the NestJS side.
 */
const HEADER = 'x-request-id';

declare module 'express-serve-static-core' {
  interface Request {
    requestId?: string;
  }
}

@Injectable()
export class RequestIdMiddleware implements NestMiddleware {
  use(req: Request, res: Response, next: NextFunction): void {
    const incoming = (req.headers[HEADER] as string | undefined)?.trim();
    const id = incoming && incoming.length > 0 ? incoming : randomUUID();
    req.requestId = id;
    res.setHeader('X-Request-Id', id);
    next();
  }
}
