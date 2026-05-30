import { StartedPostgreSqlContainer } from '@testcontainers/postgresql';
import { StartedRedisContainer } from '@testcontainers/redis';

declare global {
  /* eslint-disable no-var */
  var __PG__: StartedPostgreSqlContainer | undefined;
  var __REDIS__: StartedRedisContainer | undefined;
  /* eslint-enable no-var */
}

/**
 * Stop the containers spun up in setup.ts. Always logs the stop result
 * so a CI log shows the containers actually shut down — leaked
 * containers from a botched teardown will eventually exhaust docker
 * disk on the host running the suite.
 */
export default async function teardown(): Promise<void> {
  if (globalThis.__REDIS__) {
    await globalThis.__REDIS__.stop();
  }
  if (globalThis.__PG__) {
    await globalThis.__PG__.stop();
  }
}
