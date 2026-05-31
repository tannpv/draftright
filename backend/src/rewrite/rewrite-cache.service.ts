import { Inject, Injectable } from '@nestjs/common';
import Redis from 'ioredis';
import { createHash } from 'crypto';
import { REDIS_CLIENT } from '../common/redis/redis.module';

const CACHE_TTL = 300; // 5 minutes in seconds

/**
 * Per-user rewrite cache with batch-state flags + a generic
 * rate-limit counter helper.  Backed by Redis via DI — never
 * constructs its own client (S7 of the 2026-05-31 code review).
 *
 * Failure mode: every method swallows Redis exceptions and returns
 * the "no cache hit" answer.  Cache outage degrades to direct AI
 * calls; never blocks user-facing traffic.
 */
@Injectable()
export class RewriteCacheService {
  constructor(@Inject(REDIS_CLIENT) private readonly redis: Redis) {}

  async get(userId: string, text: string, tone: string): Promise<string | null> {
    try {
      return await this.redis.get(this.resultKey(userId, text, tone));
    } catch {
      return null;
    }
  }

  async set(userId: string, text: string, tone: string, result: string): Promise<void> {
    try {
      await this.redis.set(this.resultKey(userId, text, tone), result, 'EX', CACHE_TTL);
    } catch {
      // Redis unavailable — skip caching
    }
  }

  async isBatchStarted(userId: string, text: string): Promise<boolean> {
    try {
      const val = await this.redis.get(this.batchKey(userId, text));
      return val === '1';
    } catch {
      return false;
    }
  }

  async markBatchStarted(userId: string, text: string): Promise<void> {
    try {
      await this.redis.set(this.batchKey(userId, text), '1', 'EX', CACHE_TTL);
    } catch {
      // Redis unavailable — skip
    }
  }

  private hashText(text: string): string {
    return createHash('sha256').update(text).digest('hex').substring(0, 16);
  }

  private resultKey(userId: string, text: string, tone: string): string {
    return `rewrite:${userId}:${this.hashText(text)}:${tone}`;
  }

  private batchKey(userId: string, text: string): string {
    return `batch:${userId}:${this.hashText(text)}`;
  }

  /** Increment a Redis key and set expiry if new. Returns the new count. */
  async incrementWithExpiry(key: string, ttlSeconds: number): Promise<number> {
    try {
      const count = await this.redis.incr(key);
      if (count === 1) {
        await this.redis.expire(key, ttlSeconds);
      }
      return count;
    } catch {
      return 0; // Redis unavailable — allow request
    }
  }
}
