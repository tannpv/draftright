import { ExecutionContext, UnauthorizedException } from '@nestjs/common';
import { RewriteAuthGuard } from './rewrite-auth.guard';
import { ExtensionTokenService } from './extension-token.service';
import { JwtAuthGuard } from './jwt-auth.guard';

function makeCtx(headers: Record<string, string>): ExecutionContext {
  const req: any = { headers };
  return {
    switchToHttp: () => ({ getRequest: () => req }),
  } as any;
}

describe('RewriteAuthGuard', () => {
  let extService: jest.Mocked<ExtensionTokenService>;
  let jwtGuard: jest.Mocked<JwtAuthGuard>;
  let guard: RewriteAuthGuard;

  beforeEach(() => {
    extService = { validate: jest.fn() } as any;
    jwtGuard = { canActivate: jest.fn() } as any;
    guard = new RewriteAuthGuard(extService, jwtGuard);
  });

  it('rejects requests with no Authorization header', async () => {
    const ctx = makeCtx({});
    await expect(guard.canActivate(ctx)).rejects.toThrow(UnauthorizedException);
  });

  // TC: EXTTOK-009
  it('uses extension-token path when Authorization is dr_ext_*', async () => {
    extService.validate.mockResolvedValue({
      tokenId: 'tok-1',
      userId: 'user-1',
      scopes: ['rewrite'],
    });
    const ctx = makeCtx({ authorization: 'Bearer dr_ext_abc' });
    const result = await guard.canActivate(ctx);
    expect(result).toBe(true);
    const req = ctx.switchToHttp().getRequest();
    expect(req.user).toEqual({
      id: 'user-1',
      email: '',
      role: '',
      isAdmin: false,
      via: 'extension_token',
      tokenId: 'tok-1',
    });
    expect(jwtGuard.canActivate).not.toHaveBeenCalled();
  });

  // TC: EXTTOK-010 (scope enforcement)
  it('rejects extension token without rewrite scope', async () => {
    extService.validate.mockResolvedValue({
      tokenId: 'tok-1',
      userId: 'user-1',
      scopes: ['something-else'],
    });
    const ctx = makeCtx({ authorization: 'Bearer dr_ext_abc' });
    await expect(guard.canActivate(ctx)).rejects.toThrow(UnauthorizedException);
  });

  // TC: EXTTOK-005
  it('rejects invalid extension token', async () => {
    extService.validate.mockResolvedValue(null);
    const ctx = makeCtx({ authorization: 'Bearer dr_ext_bogus' });
    await expect(guard.canActivate(ctx)).rejects.toThrow(UnauthorizedException);
  });

  // TC: EXTTOK-008
  it('falls through to JWT guard for non-prefixed tokens', async () => {
    jwtGuard.canActivate.mockResolvedValue(true);
    const ctx = makeCtx({ authorization: 'Bearer regular.jwt.value' });
    const result = await guard.canActivate(ctx);
    expect(result).toBe(true);
    expect(jwtGuard.canActivate).toHaveBeenCalledWith(ctx);
    expect(extService.validate).not.toHaveBeenCalled();
  });
});
