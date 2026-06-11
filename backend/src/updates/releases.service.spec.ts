import { BadRequestException } from '@nestjs/common';
import { ReleasesService } from './releases.service';
import { AppRelease } from './entities/app-release.entity';

// Minimal in-memory repo — only the methods upsertChannel touches.
class FakeReleaseRepo {
  rows: AppRelease[] = [];
  async findOne({ where }: any) {
    return (
      this.rows.find(
        (r) => r.platform === where.platform && r.channel === where.channel,
      ) ?? null
    );
  }
  create(p: Partial<AppRelease>): AppRelease {
    return { ...(p as any) };
  }
  async save(r: AppRelease) {
    const i = this.rows.findIndex(
      (x) => x.platform === r.platform && x.channel === r.channel,
    );
    if (i >= 0) this.rows[i] = r;
    else this.rows.push(r);
    return r;
  }
}

describe('ReleasesService.upsertChannel — sha256', () => {
  let repo: FakeReleaseRepo;
  let svc: ReleasesService;
  const HASH = 'a'.repeat(64);

  beforeEach(() => {
    repo = new FakeReleaseRepo();
    svc = new ReleasesService(repo as any, {} as any);
  });

  const base = { platform: 'linux', channel: 'direct', version: '2.4.1', download_url: 'https://x/app' };

  it('stores a valid 64-char hex hash (lowercased)', async () => {
    const row = await svc.upsertChannel({ ...base, sha256: HASH.toUpperCase() });
    expect(row.sha256).toBe(HASH);
  });

  it('rejects a malformed hash', async () => {
    await expect(svc.upsertChannel({ ...base, sha256: 'nothex' })).rejects.toBeInstanceOf(
      BadRequestException,
    );
  });

  it('allows an empty hash (unrecorded artifact)', async () => {
    const row = await svc.upsertChannel({ ...base, sha256: '' });
    expect(row.sha256).toBe('');
  });

  it('defaults to empty when sha256 is omitted', async () => {
    const row = await svc.upsertChannel({ ...base });
    expect(row.sha256).toBe('');
  });

  it('clears a stale hash when a new artifact is published without one', async () => {
    await svc.upsertChannel({ ...base, sha256: HASH });
    const updated = await svc.upsertChannel({ ...base, version: '2.4.2', download_url: 'https://x/app2' });
    expect(updated.sha256).toBe('');
  });
});
