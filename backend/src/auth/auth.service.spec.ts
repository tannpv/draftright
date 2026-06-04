import { AuthService } from './auth.service';
import { AuthProvider } from '../users/entities/user.entity';
import { BadRequestException } from '@nestjs/common';

describe('AuthService — forgot/reset password', () => {
  function build(user: any) {
    const usersService: any = {
      findByEmail: async () => user,
      update: jest.fn().mockResolvedValue(undefined),
    };
    const emailService: any = {
      sendPasswordResetEmail: jest.fn().mockResolvedValue(undefined),
    };
    // Only usersService + emailService are reached by these methods.
    const svc = new AuthService(
      usersService,
      undefined as any, // jwt
      undefined as any, // plans
      undefined as any, // subscriptions
      emailService,
      undefined as any, // settingsRepo
      undefined as any, // cfg
    );
    return { svc, usersService, emailService };
  }

  const future = new Date(Date.now() + 60_000);
  const past = new Date(Date.now() - 60_000);
  const localUser = (over: any = {}) => ({
    id: 'u', email: 'a@b.com', name: 'A',
    auth_provider: AuthProvider.LOCAL, password_hash: 'old',
    password_reset_code: '111111', password_reset_expires: future, ...over,
  });

  it('reset rejects a wrong code', async () => {
    const { svc } = build(localUser());
    await expect(svc.resetPassword('a@b.com', '999999', 'newpassword1'))
      .rejects.toBeInstanceOf(BadRequestException);
  });

  it('reset rejects an expired code', async () => {
    const { svc } = build(localUser({ password_reset_expires: past }));
    await expect(svc.resetPassword('a@b.com', '111111', 'newpassword1'))
      .rejects.toBeInstanceOf(BadRequestException);
  });

  it('reset rejects a too-short password', async () => {
    const { svc } = build(localUser());
    await expect(svc.resetPassword('a@b.com', '111111', 'short'))
      .rejects.toBeInstanceOf(BadRequestException);
  });

  it('reset succeeds: sets a new hash and clears the code (single-use)', async () => {
    const { svc, usersService } = build(localUser());
    const res = await svc.resetPassword('a@b.com', '111111', 'newpassword1');
    expect(res).toEqual({ success: true });
    const patch = usersService.update.mock.calls[0][1];
    expect(patch.password_hash).toBeTruthy();
    expect(patch.password_hash).not.toBe('old');
    expect(patch.password_reset_code).toBeNull();
    expect(patch.password_reset_expires).toBeNull();
  });

  it('forgot stays silent (no email) for a social-only account', async () => {
    const { svc, emailService } = build(localUser({
      auth_provider: AuthProvider.GOOGLE, password_hash: null,
    }));
    await svc.forgotPassword('a@b.com');
    expect(emailService.sendPasswordResetEmail).not.toHaveBeenCalled();
  });

  it('forgot stays silent for an unknown email', async () => {
    const { svc, emailService } = build(null);
    await svc.forgotPassword('nobody@b.com');
    expect(emailService.sendPasswordResetEmail).not.toHaveBeenCalled();
  });

  it('forgot emails a code to a local account', async () => {
    const { svc, emailService, usersService } = build(localUser());
    await svc.forgotPassword('a@b.com');
    expect(emailService.sendPasswordResetEmail).toHaveBeenCalledTimes(1);
    const patch = usersService.update.mock.calls[0][1];
    expect(patch.password_reset_code).toMatch(/^\d{6}$/);
  });
});
