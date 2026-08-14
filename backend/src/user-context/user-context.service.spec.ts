import { UserContextService } from './user-context.service';
import { UserContext } from './entities/user-context.entity';

/** In-memory stand-in for the TypeORM repository. */
function fakeRepo() {
  let row: UserContext | null = null;
  return {
    store: () => row,
    findOne: async ({ where }: any) =>
      row && row.user_id === where.user_id ? row : null,
    create: (partial: Partial<UserContext>) => ({ ...partial }) as UserContext,
    save: async (r: UserContext) => {
      const defaults = { enabled: false, job_title: '', industry: '', audience: '', style_notes: '' };
      row = { ...defaults, ...r } as UserContext;
      return row;
    },
    delete: async () => {
      row = null;
      return { affected: 1 };
    },
  };
}

function build() {
  const repo = fakeRepo();
  return { svc: new UserContextService(repo as any), repo };
}

const UID = 'user-1';

describe('UserContextService', () => {
  it('get returns null for a user with no context', async () => {
    const { svc } = build();
    expect(await svc.get(UID)).toBeNull();
  });

  it('upsert creates then patches only the provided fields', async () => {
    const { svc } = build();
    await svc.upsert(UID, { enabled: true, job_title: 'Lawyer' });
    let ctx = await svc.get(UID);
    expect(ctx?.enabled).toBe(true);
    expect(ctx?.job_title).toBe('Lawyer');

    // patch industry only — job_title must survive
    await svc.upsert(UID, { industry: 'finance' });
    ctx = await svc.get(UID);
    expect(ctx?.job_title).toBe('Lawyer');
    expect(ctx?.industry).toBe('finance');
  });

  it('clear removes the row (GDPR erasure)', async () => {
    const { svc } = build();
    await svc.upsert(UID, { enabled: true, job_title: 'Nurse' });
    await svc.clear(UID);
    expect(await svc.get(UID)).toBeNull();
  });

  it('getPreamble returns null when disabled, a block when enabled+filled', async () => {
    const { svc } = build();
    await svc.upsert(UID, { enabled: false, job_title: 'Lawyer' });
    expect(await svc.getPreamble(UID)).toBeNull();

    await svc.upsert(UID, { enabled: true });
    const pre = await svc.getPreamble(UID);
    expect(pre).toContain('Lawyer');
    expect(pre).toContain('do not mention it');
  });

  it('getPreamble degrades to null on a lookup error (never breaks the rewrite)', async () => {
    const repo: any = { findOne: async () => { throw new Error('db down'); } };
    const svc = new UserContextService(repo);
    expect(await svc.getPreamble(UID)).toBeNull();
  });
});
