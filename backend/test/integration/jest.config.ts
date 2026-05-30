import type { Config } from 'jest';

/**
 * Integration test config — separate from the unit-test jest config in
 * package.json so the slow paths (testcontainers boot, real DB
 * migrations) only run when explicitly invoked via
 *
 *   npm run test:integration
 *
 * Tests live under backend/test/integration/ and end with
 * .int-spec.ts so a misnamed unit spec can't accidentally pull in
 * the heavier setup.
 *
 * Key choices:
 *   maxWorkers=1            One container set, shared across tests.
 *                           Parallel workers would fork containers
 *                           and gum up local docker.
 *   globalSetup/Teardown    Boots PG + Redis once per `jest` invoke.
 *                           Test files read the connection strings
 *                           via process.env (populated by setup).
 *   testTimeout=120000      First-time image pull can run ~60 s on a
 *                           cold cache; 2 min keeps the suite green
 *                           on slow networks.
 *   detectOpenHandles=true  Surfaces leaked DB pools or pending
 *                           timers so a writer can't ship a flaky
 *                           cleanup.
 */
const config: Config = {
  rootDir: '../..',
  testMatch: ['<rootDir>/test/integration/**/*.int-spec.ts'],
  moduleFileExtensions: ['js', 'json', 'ts'],
  transform: { '^.+\\.(t|j)s$': 'ts-jest' },
  testEnvironment: 'node',
  globalSetup: '<rootDir>/test/integration/setup.ts',
  globalTeardown: '<rootDir>/test/integration/teardown.ts',
  maxWorkers: 1,
  testTimeout: 120_000,
  detectOpenHandles: true,
  forceExit: true,
};

export default config;
