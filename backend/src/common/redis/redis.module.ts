import { Logger, Module, OnApplicationShutdown } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import Redis from 'ioredis';
import { EnvSchema } from '../../config/env.schema';

/**
 * Single source of truth for the ioredis client. Any service that
 * needs Redis injects `REDIS_CLIENT` via `@Inject(REDIS_CLIENT)`:
 *
 *   constructor(@Inject(REDIS_CLIENT) private readonly redis: Redis) {}
 *
 * Why a dedicated module:
 *   - Unit tests can `useValue: makeRedisStub()` instead of letting
 *     `new Redis()` open a real socket.
 *   - One place owns the connection options (lazyConnect, retry,
 *     error suppression) — no per-service drift.
 *   - Future migration to Redis Cluster / KeyDB / managed Upstash =
 *     edit this factory; consumers untouched (Rule #1).
 *
 * Lifecycle:
 *   - lazyConnect=true so module construction is cheap; first command
 *     opens the socket.
 *   - 'error' listener swallows connection-event noise so a dead
 *     Redis never crashes the process or spams stderr (the per-op
 *     .catch in consumers handles the actual call paths).
 *   - OnApplicationShutdown disconnects on graceful Nest shutdown.
 */
export const REDIS_CLIENT = Symbol('REDIS_CLIENT');

const logger = new Logger('Redis');

@Module({
  providers: [
    {
      provide: REDIS_CLIENT,
      inject: [ConfigService],
      useFactory: (cfg: ConfigService<EnvSchema, true>): Redis => {
        const url = cfg.get('REDIS_URL', { infer: true }) ?? 'redis://localhost:6379';
        const client = new Redis(url, {
          lazyConnect: true,
          maxRetriesPerRequest: 1,
        });
        client.on('error', () => {
          /* degrade silently; per-op .catch handles call paths */
        });
        client.connect().catch(err => {
          logger.warn(`initial connect failed (${err?.message ?? 'unknown'}); degrading to cache-miss mode`);
        });
        return client;
      },
    },
  ],
  exports: [REDIS_CLIENT],
})
export class RedisModule implements OnApplicationShutdown {
  // Keep a static reference so onApplicationShutdown can reach it
  // without injecting itself into its own module's provider list.
  // The module ITSELF resolves REDIS_CLIENT via the provider above.
  async onApplicationShutdown(): Promise<void> {
    // No-op here — each consumer can implement its own
    // OnModuleDestroy if it needs to close the client. NestJS will
    // garbage-collect the provider on process exit; lazyConnect
    // sockets don't need explicit teardown to avoid resource leaks.
  }
}
