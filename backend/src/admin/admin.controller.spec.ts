import { AdminController } from './admin.controller';

// Focused unit spec for the #32 admin soft-delete guards. Only adminUserRepo
// matters here, so we build the controller via the prototype and assign the
// single mock dependency the handler touches.
describe('deleteAdminUser guards (#32)', () => {
  const acting = { user: { id: 'admin-self' } } as any;

  function makeController(repo: any): AdminController {
    const c: any = Object.create(AdminController.prototype);
    c.adminUserRepo = repo;
    return c;
  }

  it('rejects self-deletion with 400', async () => {
    const repo = { findOne: jest.fn(), count: jest.fn(), update: jest.fn() };
    const c = makeController(repo);
    await expect(c.deleteAdminUser('admin-self', acting)).rejects.toThrow(
      'You cannot deactivate your own admin account',
    );
    expect(repo.update).not.toHaveBeenCalled();
  });

  it('rejects deleting the last active admin with 400', async () => {
    const repo = {
      findOne: jest.fn().mockResolvedValue({ id: 'other', is_active: true }),
      count: jest.fn().mockResolvedValue(1),
      update: jest.fn(),
    };
    const c = makeController(repo);
    await expect(c.deleteAdminUser('other', acting)).rejects.toThrow(
      'Cannot deactivate the last active admin',
    );
    expect(repo.update).not.toHaveBeenCalled();
  });

  it('allows a normal delete (other admins remain active)', async () => {
    const repo = {
      findOne: jest.fn().mockResolvedValue({ id: 'other', is_active: true }),
      count: jest.fn().mockResolvedValue(3),
      update: jest.fn().mockResolvedValue({}),
    };
    const c = makeController(repo);
    await expect(c.deleteAdminUser('other', acting)).resolves.toEqual({ success: true });
    expect(repo.update).toHaveBeenCalledWith('other', { is_active: false });
  });
});

// #48: GET /admin/payments leftJoinAndSelect's the raw user — listPayments must
// drop the six secret columns from each nested user before responding.
describe('listPayments strips nested user secrets (#48)', () => {
  const SECRET_COLS = [
    'password_hash',
    'email_verification_code',
    'email_verification_expires',
    'password_reset_code',
    'password_reset_expires',
    'password_reset_attempts',
  ];

  function makeController(paymentService: any): AdminController {
    const c: any = Object.create(AdminController.prototype);
    c.paymentService = paymentService;
    return c;
  }

  it('removes every secret column from the nested user, keeps non-secret fields', async () => {
    const rawUser = {
      id: 'u1',
      email: 'a@b.com',
      name: 'Alice',
      password_hash: '$2b$10$hash',
      email_verification_code: 'evc',
      email_verification_expires: '2026-01-01',
      password_reset_code: 'prc',
      password_reset_expires: '2026-01-01',
      password_reset_attempts: 2,
    };
    const paymentService = {
      findAll: jest
        .fn()
        .mockResolvedValue({ payments: [{ id: 'pay1', user: rawUser }], total: 1 }),
    };
    const c = makeController(paymentService);

    const res: any = await c.listPayments();

    expect(res.total).toBe(1);
    const u = res.payments[0].user;
    for (const col of SECRET_COLS) expect(u).not.toHaveProperty(col);
    expect(u).toMatchObject({ id: 'u1', email: 'a@b.com', name: 'Alice' });
  });

  it('passes a null nested user through unchanged', async () => {
    const paymentService = {
      findAll: jest
        .fn()
        .mockResolvedValue({ payments: [{ id: 'pay2', user: null }], total: 1 }),
    };
    const c = makeController(paymentService);

    const res: any = await c.listPayments();

    expect(res.payments[0].user).toBeNull();
  });
});
