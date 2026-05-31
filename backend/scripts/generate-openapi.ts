/**
 * Generates backend/openapi.json from the live NestJS Swagger
 * configuration — without booting an HTTP listener.
 *
 * Usage:
 *   npm run openapi:generate    # writes backend/openapi.json
 *   npm run openapi:check       # regenerate + git diff; CI gate
 *
 * Why a static spec file:
 *   - Clients (admin SPA, Flutter, Swift, etc.) can generate types
 *     from a known-good contract rather than scraping a moving API.
 *   - Any PR that intentionally breaks the wire shows up as a diff
 *     in openapi.json — the reviewer SEES the change instead of
 *     finding it post-deploy.
 *   - Standard S26 (architecture-standards.md) — "API contract is
 *     codified, not implicit."
 *
 * Why testcontainers:
 *   AppModule wires TypeORM with `synchronize: true`; it refuses to
 *   boot without a reachable Postgres.  We spin up an ephemeral
 *   container, point the schema validator at it, generate the spec,
 *   tear down.  Reuses the same images integration tests use, so
 *   spec-gen + tests fail / pass on the same shape.
 */
import { NestFactory } from '@nestjs/core';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { PostgreSqlContainer, StartedPostgreSqlContainer } from '@testcontainers/postgresql';
import { writeFileSync } from 'fs';
import { resolve } from 'path';

async function main(): Promise<void> {
  // eslint-disable-next-line no-console
  console.log('[openapi-gen] booting ephemeral Postgres…');
  const pg = await new PostgreSqlContainer('postgres:16-alpine')
    .withDatabase('openapi_gen')
    .withUsername('gen')
    .withPassword('gen')
    .start();

  // Set env BEFORE importing AppModule so ConfigModule + TypeORM see
  // the right values on first construction.
  process.env.DATABASE_URL = pg.getConnectionUri();
  // Redis is touched lazily by RewriteCacheService; ioredis 'error'
  // listener (added in the same commit as this script) prevents the
  // dead-connection retries from killing the process.
  process.env.REDIS_URL ||= 'redis://127.0.0.1:1';
  process.env.JWT_SECRET ||= 'spec-gen-jwt-secret-16chars-min';
  process.env.JWT_REFRESH_SECRET ||= 'spec-gen-refresh-secret-16chars';
  process.env.NODE_ENV = 'test';

  // Lazy-import so the env vars above are visible during module init.
  const { AppModule } = await import('../src/app.module');

  try {
    // eslint-disable-next-line no-console
    console.log('[openapi-gen] booting NestFactory…');
    // createApplicationContext skips the HTTP adapter, but
    // SwaggerModule.createDocument needs one. Use create() —
    // adapter is built but we never call .listen(), so no port is
    // bound.
    const app = await NestFactory.create(AppModule, {
      logger: ['error'],
    });
    await app.init();

    const config = new DocumentBuilder()
      .setTitle('DraftRight API')
      .setDescription('AI-powered text rewriting backend')
      .setVersion('1.0')
      .addBearerAuth()
      .build();
    const document = SwaggerModule.createDocument(app as any, config);

    // Stable key order so module-registration order can't cause
    // spurious drift in the checked-in spec (S26b). Paths +
    // components.schemas are the two big map-shaped sections; sort
    // both alphabetically.
    const stable = {
      ...document,
      paths: sortKeys(document.paths as Record<string, unknown>),
      components: document.components
        ? {
            ...document.components,
            schemas: document.components.schemas
              ? sortKeys(document.components.schemas as Record<string, unknown>)
              : document.components.schemas,
          }
        : document.components,
    };

    const out = resolve(__dirname, '..', 'openapi.json');
    writeFileSync(out, JSON.stringify(stable, null, 2) + '\n', 'utf-8');
    // eslint-disable-next-line no-console
    console.log(`[openapi-gen] wrote ${out}`);

    await app.close();
  } finally {
    await pg.stop();
  }

  // ioredis retry loop keeps the event loop alive — bail explicitly.
  process.exit(0);
}

function sortKeys<T extends Record<string, unknown>>(obj: T): T {
  return Object.keys(obj)
    .sort()
    .reduce((acc, k) => ({ ...acc, [k]: obj[k] }), {} as T);
}

main().catch(err => {
  // eslint-disable-next-line no-console
  console.error('openapi:generate failed:', err);
  process.exit(1);
});
