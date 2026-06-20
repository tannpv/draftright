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
