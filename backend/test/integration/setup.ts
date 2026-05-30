import { PostgreSqlContainer, StartedPostgreSqlContainer } from '@testcontainers/postgresql';
import { RedisContainer, StartedRedisContainer } from '@testcontainers/redis';

/**
 * Boots one Postgres + one Redis container per `jest` invocation and
 * publishes their connection URLs onto process.env so test files can
 * load the existing ConfigService path unchanged.
 *
 * Each .int-spec.ts is responsible for spinning up its own NestJS
 * TestingModule with TypeORM `synchronize: true` — entities create
 * the schema on first connect, so we don't have to maintain a parallel
 * migration runner here.  Delta-only SQL migrations live in
 * `backend/sql/` and can be applied per-test on demand.
 *
 * Image versions track prod (postgres:16-alpine, redis:7-alpine). Any
 * drift would mean false-negative integration coverage: tests passing
 * against shapes that prod doesn't have.
 *
 * Stash the started containers on globalThis so the matching
 * teardown.ts can stop them after the suite.
 */
declare global {
  /* eslint-disable no-var */
  var __PG__: StartedPostgreSqlContainer | undefined;
  var __REDIS__: StartedRedisContainer | undefined;
  /* eslint-enable no-var */
}

export default async function setup(): Promise<void> {
  const pg = await new PostgreSqlContainer('postgres:16-alpine')
    .withDatabase('draftright_test')
    .withUsername('test')
    .withPassword('test')
    .start();

  process.env.DATABASE_URL = pg.getConnectionUri();
  globalThis.__PG__ = pg;

  const redis = await new RedisContainer('redis:7-alpine').start();
  process.env.REDIS_URL = redis.getConnectionUrl();
  globalThis.__REDIS__ = redis;

  // env.schema.ts enforces these minimums; populate them once so each
  // test file doesn't have to.
  process.env.JWT_SECRET ||= 'integration-test-jwt-secret-min-16';
  process.env.JWT_REFRESH_SECRET ||= 'integration-test-refresh-secret-16';
  process.env.NODE_ENV = 'test';
  process.env.GO_BACKEND_RAMP_PERCENT ||= '0';
}
