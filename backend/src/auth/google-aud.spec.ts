import { AuthService } from './auth.service';
import { UnauthorizedException } from '@nestjs/common';

/**
 * Google ID tokens must be checked against OUR client ids.
 *
 * Google's tokeninfo endpoint only proves that Google minted the token — not
 * that it was minted for us. Without an `aud` check, an id_token issued to
 * ANY other OAuth client (any app the user has signed into with Google) could
 * be replayed against POST /auth/social to take over that user's account.
 */
describe('AuthService — Google id_token audience validation', () => {
  const OURS = 'ours.apps.googleusercontent.com';
  const ATTACKER = 'attacker.apps.googleusercontent.com';

  let originalFetch: typeof global.fetch;

  beforeEach(() => {
    originalFetch = global.fetch;
  });
  afterEach(() => {
    global.fetch = originalFetch;
  });

  /** Stub Google's tokeninfo response. */
  function stubTokenInfo(payload: any, ok = true) {
    global.fetch = jest.fn().mockResolvedValue({
      ok,
      json: async () => payload,
    }) as any;
  }

  function build(opts: { envAudiences?: string; settingsClientId?: string } = {}) {
    const settingsRepo: any = {
      findOne: async () =>
        opts.settingsClientId === undefined
          ? null
          : { google_client_id: opts.settingsClientId },
    };
    const cfg: any = {
      get: (key: string) => (key === 'GOOGLE_AUDIENCES' ? opts.envAudiences : undefined),
    };
    const svc = new AuthService(
      { findBySocialId: async () => null, findByEmail: async () => null } as any,
      undefined as any, // jwt
      undefined as any, // plans
      undefined as any, // subscriptions
      undefined as any, // email
      settingsRepo,
      cfg,
    );
    // Silence the service logger for expected-rejection paths.
    (svc as any).logger = { warn: jest.fn(), error: jest.fn(), log: jest.fn() };
    return svc;
  }

  /** Reach the private verifier directly — socialLogin() adds unrelated I/O. */
  const verify = (svc: AuthService, token = 'tok') =>
    (svc as any).verifyGoogleToken(token);

  const goodClaims = (aud: string) => ({
    aud,
    sub: 'google-user-1',
    email: 'victim@example.com',
    email_verified: 'true',
    iss: 'https://accounts.google.com',
    name: 'Victim',
  });

  it('REJECTS a token minted for another OAuth client (the takeover vector)', async () => {
    stubTokenInfo(goodClaims(ATTACKER));
    const svc = build({ settingsClientId: OURS });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('accepts a token minted for the configured client id', async () => {
    stubTokenInfo(goodClaims(OURS));
    const svc = build({ settingsClientId: OURS });
    const profile = await verify(svc);
    expect(profile.socialId).toBe('google-user-1');
    expect(profile.email).toBe('victim@example.com');
    expect(profile.emailVerified).toBe(true);
  });

  it('accepts any client id listed in GOOGLE_AUDIENCES (per-platform clients)', async () => {
    const linux = 'linux-desktop.apps.googleusercontent.com';
    stubTokenInfo(goodClaims(linux));
    const svc = build({ envAudiences: ` ${linux} , ios.apps `, settingsClientId: OURS });
    await expect(verify(svc)).resolves.toMatchObject({ socialId: 'google-user-1' });
  });

  it('merges env and settings sources', async () => {
    stubTokenInfo(goodClaims(OURS));
    const svc = build({ envAudiences: 'other.apps', settingsClientId: OURS });
    await expect(verify(svc)).resolves.toBeTruthy();
  });

  it('FAILS CLOSED when the audience list is explicitly emptied', async () => {
    // An empty allow-list must not degrade to "accept anything".
    stubTokenInfo(goodClaims(ATTACKER));
    const svc = build({ envAudiences: ' , ', settingsClientId: '' });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('keeps the shipped clients working with no configuration at all', async () => {
    // Deploying this check must not break existing sign-ins. macOS uses a
    // DIFFERENT client id from app_settings.google_client_id, so both are in
    // the built-in fallback.
    const shipped = {
      'web + mobile': '22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com',
      macOS: '22951518033-dvkn61dhibse9fu83ohh51mlovd7269a.apps.googleusercontent.com',
    };
    for (const [platform, clientId] of Object.entries(shipped)) {
      stubTokenInfo(goodClaims(clientId));
      const svc = build({}); // no env, no settings row
      await expect(verify(svc)).resolves.toMatchObject({ socialId: 'google-user-1' });
      expect(platform).toBeTruthy();
    }
  });

  it('an explicit GOOGLE_AUDIENCES replaces the fallback (lock-down)', async () => {
    stubTokenInfo(
      goodClaims('22951518033-dvkn61dhibse9fu83ohh51mlovd7269a.apps.googleusercontent.com'),
    );
    const svc = build({ envAudiences: OURS });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('rejects a token with no aud claim at all', async () => {
    const { aud, ...noAud } = goodClaims(OURS);
    stubTokenInfo(noAud);
    const svc = build({ settingsClientId: OURS });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('rejects an unexpected issuer', async () => {
    stubTokenInfo({ ...goodClaims(OURS), iss: 'https://evil.example.com' });
    const svc = build({ settingsClientId: OURS });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('accepts both spellings of the Google issuer', async () => {
    for (const iss of ['accounts.google.com', 'https://accounts.google.com']) {
      stubTokenInfo({ ...goodClaims(OURS), iss });
      const svc = build({ settingsClientId: OURS });
      await expect(verify(svc)).resolves.toBeTruthy();
    }
  });

  it('rejects a token with no sub claim', async () => {
    const { sub, ...noSub } = goodClaims(OURS);
    stubTokenInfo(noSub);
    const svc = build({ settingsClientId: OURS });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('still rejects when tokeninfo itself says the token is bad', async () => {
    stubTokenInfo({}, false);
    const svc = build({ settingsClientId: OURS });
    await expect(verify(svc)).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('does not treat an unverified email as verified', async () => {
    stubTokenInfo({ ...goodClaims(OURS), email_verified: 'false' });
    const svc = build({ settingsClientId: OURS });
    await expect(verify(svc)).resolves.toMatchObject({ emailVerified: false });
  });
});
