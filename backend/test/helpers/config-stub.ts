import { ConfigService } from '@nestjs/config';
import { EnvSchema } from '../../src/config/env.schema';

/**
 * Builds a minimal ConfigService stub for unit tests of services
 * that inject `ConfigService<EnvSchema, true>`.
 *
 * Pass the env keys the service-under-test will actually read; any
 * other key throws so a forgotten dependency surfaces as a clear
 * test failure instead of a silent `undefined`.
 *
 * Example:
 *   const cfg = makeConfigStub({ GO_BACKEND_RAMP_PERCENT: 50 });
 *   const svc = new FeatureFlagsService(cfg);
 */
export function makeConfigStub(
  values: Partial<EnvSchema>,
): ConfigService<EnvSchema, true> {
  const stub = {
    get<T>(key: keyof EnvSchema): T {
      if (key in values) {
        return values[key] as unknown as T;
      }
      throw new Error(
        `makeConfigStub: missing fake for key "${String(key)}" — ` +
          `add it to the values map in the test setup.`,
      );
    },
    // @nestjs/config also exposes getOrThrow, set, changes$, etc.
    // The services we test today only call get(); add more methods
    // here as new consumers appear.
  } as unknown as ConfigService<EnvSchema, true>;
  return stub;
}
