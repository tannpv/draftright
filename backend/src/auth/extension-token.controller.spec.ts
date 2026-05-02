import { Test } from '@nestjs/testing';
import { ExtensionTokenController } from './extension-token.controller';
import { ExtensionTokenService } from './extension-token.service';

describe('ExtensionTokenController', () => {
  let controller: ExtensionTokenController;
  let service: jest.Mocked<ExtensionTokenService>;

  beforeEach(async () => {
    const serviceMock = {
      mint: jest.fn(),
      list: jest.fn(),
      revoke: jest.fn(),
    } as any;

    const module = await Test.createTestingModule({
      controllers: [ExtensionTokenController],
      providers: [{ provide: ExtensionTokenService, useValue: serviceMock }],
    })
      .overrideGuard(require('./jwt-auth.guard').JwtAuthGuard)
      .useValue({ canActivate: () => true })
      .compile();

    controller = module.get(ExtensionTokenController);
    service = module.get(ExtensionTokenService);
  });

  // TC: EXTTOK-001
  it('mint returns the plaintext token (only time it is exposed)', async () => {
    service.mint.mockResolvedValue({ token: 'dr_ext_abc', id: 'tok-1' });
    const req = { user: { id: 'user-1' } };
    const result = await controller.mint(req, {
      device_id: 'd1-uuid',
      device_name: 'iPhone',
    });
    expect(result).toEqual({ token: 'dr_ext_abc', id: 'tok-1' });
    expect(service.mint).toHaveBeenCalledWith('user-1', 'd1-uuid', 'iPhone');
  });

  // TC: EXTTOK-002 (list never exposes hash)
  it('list returns rows for current user, never exposes token_hash', async () => {
    service.list.mockResolvedValue([
      {
        id: 'tok-1',
        user_id: 'user-1',
        token_hash: 'secret',
        scopes: ['rewrite'],
        device_id: 'd1',
        device_name: 'iPhone',
        last_used_at: null,
        created_at: new Date('2026-05-02'),
        revoked_at: null,
      } as any,
    ]);
    const result = await controller.list({ user: { id: 'user-1' } });
    expect(result[0]).not.toHaveProperty('token_hash');
    expect(result[0]).not.toHaveProperty('user_id');
    expect(result[0]).toMatchObject({ id: 'tok-1', device_name: 'iPhone' });
  });

  // TC: EXTTOK-007
  it('revoke calls service with current user id and token id', async () => {
    await controller.revoke({ user: { id: 'user-1' } }, 'tok-1');
    expect(service.revoke).toHaveBeenCalledWith('user-1', 'tok-1');
  });
});
