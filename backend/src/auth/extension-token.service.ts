import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { IsNull, Repository } from 'typeorm';
import * as crypto from 'crypto';
import { ExtensionToken } from './extension-token.entity';

const TOKEN_PREFIX = 'dr_ext_';

export interface ValidatedExtensionToken {
  tokenId: string;
  userId: string;
  scopes: string[];
}

@Injectable()
export class ExtensionTokenService {
  constructor(
    @InjectRepository(ExtensionToken)
    private readonly repo: Repository<ExtensionToken>,
  ) {}

  async mint(
    userId: string,
    deviceId: string,
    deviceName: string,
  ): Promise<{ token: string; id: string }> {
    // Revoke existing active token for this (user, device) pair so the
    // partial unique index on (user_id, device_id) WHERE revoked_at IS NULL
    // doesn't collide.
    const existing = await this.repo.findOne({
      where: { user_id: userId, device_id: deviceId, revoked_at: IsNull() },
    });
    if (existing) {
      await this.repo.update({ id: existing.id }, { revoked_at: new Date() });
    }

    // 32 random bytes → 43 url-safe base64 chars (no padding).
    const raw = crypto.randomBytes(32).toString('base64url');
    const token = `${TOKEN_PREFIX}${raw}`;
    const tokenHash = crypto.createHash('sha256').update(token).digest('hex');

    const row = this.repo.create({
      user_id: userId,
      token_hash: tokenHash,
      scopes: ['rewrite'],
      device_id: deviceId,
      device_name: deviceName,
    });
    const saved = await this.repo.save(row);
    return { token, id: saved.id };
  }

  async validate(presentedToken: string): Promise<ValidatedExtensionToken | null> {
    if (!presentedToken.startsWith(TOKEN_PREFIX)) return null;
    const tokenHash = crypto.createHash('sha256').update(presentedToken).digest('hex');
    const row = await this.repo.findOne({
      where: { token_hash: tokenHash, revoked_at: IsNull() },
    });
    if (!row) return null;
    // Update last_used_at write-behind. Failures here are non-fatal — the
    // request should still succeed even if the timestamp update fails.
    this.repo
      .update({ id: row.id }, { last_used_at: new Date() })
      .catch(() => undefined);
    return { tokenId: row.id, userId: row.user_id, scopes: row.scopes };
  }

  async list(userId: string): Promise<ExtensionToken[]> {
    return this.repo.find({
      where: { user_id: userId, revoked_at: IsNull() },
      order: { created_at: 'DESC' },
    });
  }

  async revoke(userId: string, tokenId: string): Promise<void> {
    await this.repo.update(
      { id: tokenId, user_id: userId },
      { revoked_at: new Date() },
    );
  }

  async revokeAll(userId: string): Promise<void> {
    await this.repo.update(
      { user_id: userId, revoked_at: IsNull() },
      { revoked_at: new Date() },
    );
  }
}
