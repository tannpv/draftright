import { Test } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { ExtensionToken } from './extension-token.entity';
import { ExtensionTokenService } from './extension-token.service';
import * as crypto from 'crypto';

describe('ExtensionTokenService', () => {
  let service: ExtensionTokenService;
  let repo: jest.Mocked<Repository<ExtensionToken>>;

  beforeEach(async () => {
    const repoMock = {
      findOne: jest.fn(),
      find: jest.fn(),
      create: jest.fn((x) => x),
      save: jest.fn(async (x) => ({ ...x, id: 'id-1', created_at: new Date() })),
      // Real TypeORM Repository.update returns a Promise<UpdateResult>.
      // The mock must too; the service calls .catch() on the result.
      update: jest.fn(async () => ({ affected: 1 })),
    } as any;

    const module = await Test.createTestingModule({
      providers: [
        ExtensionTokenService,
        { provide: getRepositoryToken(ExtensionToken), useValue: repoMock },
      ],
    }).compile();

    service = module.get(ExtensionTokenService);
    repo = module.get(getRepositoryToken(ExtensionToken));
  });

  describe('mint', () => {
    // TC: EXTTOK-001
    it('returns plaintext token with dr_ext_ prefix', async () => {
      repo.findOne.mockResolvedValue(null);
      const result = await service.mint('user-1', 'device-1', 'iPhone Keyboard');
      expect(result.token).toMatch(/^dr_ext_[A-Za-z0-9_-]{43}$/);
    });

    // TC: EXTTOK-002
    it('stores only the sha256 hash, not the plaintext', async () => {
      repo.findOne.mockResolvedValue(null);
      const result = await service.mint('user-1', 'device-1', 'iPhone Keyboard');
      const expectedHash = crypto.createHash('sha256').update(result.token).digest('hex');
      expect(repo.save).toHaveBeenCalledWith(
        expect.objectContaining({ token_hash: expectedHash }),
      );
      expect(repo.save).not.toHaveBeenCalledWith(
        expect.objectContaining({ token_hash: result.token }),
      );
    });

    // TC: EXTTOK-003
    it('revokes the existing active token for the same (user, device)', async () => {
      repo.findOne.mockResolvedValue({ id: 'existing-id' } as any);
      await service.mint('user-1', 'device-1', 'iPhone Keyboard');
      expect(repo.update).toHaveBeenCalledWith(
        { id: 'existing-id' },
        { revoked_at: expect.any(Date) },
      );
    });
  });

  describe('validate', () => {
    // TC: EXTTOK-004 (negative case)
    it('returns null for unknown token', async () => {
      repo.findOne.mockResolvedValue(null);
      const result = await service.validate('dr_ext_unknown');
      expect(result).toBeNull();
    });

    // TC: EXTTOK-005
    it('returns null for revoked token', async () => {
      repo.findOne.mockResolvedValue(null); // findOne uses revoked_at IS NULL filter
      const result = await service.validate('dr_ext_revoked');
      expect(result).toBeNull();
    });

    // TC: EXTTOK-004
    it('returns user_id and scopes for valid token', async () => {
      repo.findOne.mockResolvedValue({
        id: 'tok-1',
        user_id: 'user-1',
        scopes: ['rewrite'],
        revoked_at: null,
      } as any);
      const result = await service.validate('dr_ext_valid');
      expect(result).toEqual({ tokenId: 'tok-1', userId: 'user-1', scopes: ['rewrite'] });
    });

    // TC: EXTTOK-006
    it('returns null for token without dr_ext_ prefix', async () => {
      const result = await service.validate('not-an-extension-token');
      expect(result).toBeNull();
      expect(repo.findOne).not.toHaveBeenCalled();
    });
  });

  describe('revoke', () => {
    // TC: EXTTOK-007
    it('sets revoked_at on the row', async () => {
      await service.revoke('user-1', 'tok-1');
      expect(repo.update).toHaveBeenCalledWith(
        { id: 'tok-1', user_id: 'user-1' },
        { revoked_at: expect.any(Date) },
      );
    });
  });
});
