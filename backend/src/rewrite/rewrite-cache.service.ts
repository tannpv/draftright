import { Injectable, OnModuleDestroy } from '@nestjs/common';
import Redis from 'ioredis';
import { createHash } from 'crypto';

const CACHE_TTL = 300; // 5 minutes in seconds

@Injectable()
export class RewriteCacheService implements OnModuleDestroy {
  private readonly redis: Redis;

  constructor() {
    this.redis = new Redis(process.env.REDIS_URL || 'redis://localhost:6379', {
      lazyConnect: true,
      maxRetriesPerRequest: 1,
    });
    // ioredis emits a stream of `error` events on a dead connection.
    // Without a listener, Node treats them as unhandled and prints
    // `[ioredis] Unhandled error event: ...` (or worse — crashes
    // tooling that boots AppModule without Redis, e.g. openapi:generate).
    // Cache misses already degrade gracefully via the .catch on each
    // op below; the listener here just swallows the noise.
    this.redis.on('error', () => {
      /* degrade silently; per-op .catch handles actual call paths */
    });
    this.redis.connect().catch(() => {
      // Redis unavailable — degrade gracefully
    });
  }

  onModuleDestroy() {
    this.redis.disconnect();
  }

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
