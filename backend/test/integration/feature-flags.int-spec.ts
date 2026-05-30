import { Test, TestingModule } from '@nestjs/testing';
import { ConfigModule } from '@nestjs/config';
import { FeatureFlagsService } from '../../src/auth/feature-flags.service';
import { validateEnv } from '../../src/config/env.schema';

/**
 * Reference integration test:
 *   - Wires the real @nestjs/config ConfigModule with our Zod schema.
 *   - Boots a tiny TestingModule that contains only the
 *     FeatureFlagsService + its dep.
 *   - Exercises the service the same way a request would, but ALSO
 *     proves the env→Zod→ConfigService→service path works end-to-end
 *     (the unit test mocks ConfigService and so wouldn't catch a
 *     schema typo).
 *
 * Postgres + Redis are running via globalSetup (testcontainers); this
 * test doesn't need them yet, but the next integration test
 * (e.g. auth-login.int-spec.ts) will hit them.
 */
describe('FeatureFlagsService (integration)', () => {
  let mod: TestingModule;
  let svc: FeatureFlagsService;

  afterAll(async () => {
    await mod?.close();
  });

  it('reads GO_BACKEND_RAMP_PERCENT through the Zod-validated config', async () => {
    process.env.GO_BACKEND_RAMP_PERCENT = '37';

    mod = await Test.createTestingModule({
      imports: [
        ConfigModule.forRoot({
          isGlobal: true,
          ignoreEnvFile: true,
          // Disable cache so this spec can set the env var BEFORE
          // forRoot reads it — otherwise prior runs cache stale values.
          cache: false,
          validate: validateEnv,
        }),
      ],
      providers: [FeatureFlagsService],
    }).compile();

    svc = mod.get(FeatureFlagsService);
    expect(svc.rampPercent()).toBe(37);
  });

  it('rejects malformed env at boot rather than silently degrading', async () => {
    process.env.GO_BACKEND_RAMP_PERCENT = '200'; // outside 0..100

    await expect(
      Test.createTestingModule({
        imports: [
          ConfigModule.forRoot({
            isGlobal: true,
            ignoreEnvFile: true,
            cache: false,
            validate: validateEnv,
          }),
        ],
        providers: [FeatureFlagsService],
      }).compile(),
    ).rejects.toThrow(/Environment validation failed/);

    // Restore a valid value for the rest of the suite.
    process.env.GO_BACKEND_RAMP_PERCENT = '0';
  });
});
