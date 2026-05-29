import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch, SEARCH_DEBOUNCE_MS } from '../api';

interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: 'admin' | 'super_admin';
  is_active: boolean;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

const emptyForm = {
  name: '',
  email: '',
  password: '',
  role: 'admin' as 'admin' | 'super_admin',
  is_active: true,
};

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);

  const [showModal, setShowModal] = useState(false);
  const [editingUser, setEditingUser] = useState<AdminUser | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);

  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [sortBy, setSortBy] = useState<string>('created_at');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(page), limit: String(pageSize),
        status: statusFilter, sort_by: sortBy, sort_order: sortOrder,
      });
      if (search) params.set('search', search);
      const data = await apiFetch(`/admin/admin-users?${params}`) as { rows: AdminUser[]; total: number };
      setUsers(data.rows ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load admin users');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, search, sortBy, sortOrder]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [searchInput]);

  function openCreate() {
    setEditingUser(null);
    setForm(emptyForm);
    setShowModal(true);
  }

  function openEdit(user: AdminUser) {
    setEditingUser(user);
    setForm({
      name: user.name,
      email: user.email,
      password: '',
      role: user.role,
      is_active: user.is_active,
    });
    setShowModal(true);
  }

  async function saveUser() {
    setSaving(true);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const payload: any = {
      name: form.name,
      email: form.email,
      role: form.role,
      is_active: form.is_active,
    };

    if (form.password) {
      payload.password = form.password;
    }

    try {
      if (editingUser) {
        await apiFetch(`/admin/admin-users/${editingUser.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        setToast({ message: 'Admin user updated', type: 'success' });
      } else {
        if (!form.password) {
          setToast({ message: 'Password is required for new users', type: 'error' });
          setSaving(false);
          return;
        }
        payload.password = form.password;
        await apiFetch('/admin/admin-users', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        setToast({ message: 'Admin user created', type: 'success' });
      }
      setShowModal(false);
      fetchUsers();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Save failed', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  async function toggleActive(user: AdminUser) {
    try {
      if (user.is_active) {
        await apiFetch(`/admin/admin-users/${user.id}`, { method: 'DELETE' });
        setToast({ message: `${user.name} deactivated`, type: 'success' });
      } else {
        await apiFetch(`/admin/admin-users/${user.id}`, {
          method: 'PATCH',
          body: JSON.stringify({ is_active: true }),
        });
        setToast({ message: `${user.name} activated`, type: 'success' });
      }
      fetchUsers();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Action failed', type: 'error' });
    }
  }

  const columns = [
    {
      header: 'Name',
      key: 'name',
      sortKey: 'name',
      render: (row: AdminUser) => (
        <span style={{ color: 'var(--text)', fontWeight: 600 }}>{row.name}</span>
      ),
    },
    {
      header: 'Email',
      key: 'email',
      sortKey: 'email',
      render: (row: AdminUser) => (
        <span style={{ color: 'var(--muted)' }}>{row.email}</span>
      ),
    },
    {
      header: 'Role',
      key: 'role',
      sortKey: 'role',
      render: (row: AdminUser) => {
        const isSuperAdmin = row.role === 'super_admin';
        return (
          <span
            style={{
              display: 'inline-block',
              padding: '3px 10px',
              borderRadius: 4,
              fontSize: 12,
              fontWeight: 600,
              background: isSuperAdmin ? 'rgba(255,174,31,0.15)' : 'rgba(93,135,255,0.15)',
              color: isSuperAdmin ? 'var(--warning)' : 'var(--primary)',
            }}
          >
            {isSuperAdmin ? 'Super Admin' : 'Admin'}
          </span>
        );
      },
    },
    {
      header: 'Status',
      key: 'is_active',
      sortKey: 'is_active',
      render: (row: AdminUser) => (
        <span
          style={{
            display: 'inline-block',
            padding: '3px 10px',
            borderRadius: 4,
            fontSize: 12,
            fontWeight: 600,
            background: row.is_active ? 'rgba(19,222,185,0.15)' : 'rgba(124,143,172,0.15)',
            color: row.is_active ? 'var(--success)' : 'var(--muted)',
          }}
        >
          {row.is_active ? 'Active' : 'Inactive'}
        </span>
      ),
    },
    {
      header: 'Created',
      key: 'created_at',
      sortKey: 'created_at',
      render: (row: AdminUser) => (
        <span style={{ color: 'var(--muted)', fontSize: 13 }}>
          {new Date(row.created_at).toLocaleDateString()}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: AdminUser) => (
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            onClick={(e) => { e.stopPropagation(); openEdit(row); }}
            style={{
              padding: '5px 12px',
              borderRadius: 6,
              fontSize: 12,
              fontWeight: 600,
              border: '1px solid var(--border)',
              background: 'transparent',
              color: 'var(--primary)',
              cursor: 'pointer',
              fontFamily: 'inherit',
              transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(93,135,255,0.1)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); toggleActive(row); }}
            style={{
              padding: '5px 12px',
              borderRadius: 6,
              fontSize: 12,
              fontWeight: 600,
              border: '1px solid var(--border)',
              background: 'transparent',
              color: row.is_active ? 'var(--danger)' : 'var(--success)',
              cursor: 'pointer',
              fontFamily: 'inherit',
              transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = row.is_active
                ? 'rgba(250,137,107,0.1)' : 'rgba(19,222,185,0.1)';
            }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            {row.is_active ? 'Deactivate' : 'Activate'}
          </button>
        </div>
      ),
    },
  ];

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '9px 14px',
    borderRadius: 7,
    border: '1px solid var(--border)',
    background: 'var(--bg)',
    color: 'var(--text)',
    fontSize: 14,
    fontFamily: 'inherit',
    outline: 'none',
    transition: 'border-color 0.15s',
  };

  const labelStyle: React.CSSProperties = {
    display: 'block',
    color: 'var(--muted)',
    fontSize: 12,
    fontWeight: 600,
    marginBottom: 6,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  };

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24 }}>
        <div>
          <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 600, margin: 0 }}>Admin Users</h1>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: '4px 0 0' }}>
            Manage portal administrator accounts
          </p>
        </div>
        <button
          onClick={openCreate}
          style={{
            padding: '9px 20px',
            borderRadius: 7,
            fontSize: 13,
            fontWeight: 600,
            border: 'none',
            background: 'var(--primary)',
            color: '#fff',
            cursor: 'pointer',
            fontFamily: 'inherit',
            transition: 'opacity 0.15s',
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.85'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
        >
          + Add Admin
        </button>
      </div>

      {/* Error banner */}
      {error && (
        <div
          style={{
            padding: '12px 18px',
            borderRadius: 7,
            background: 'rgba(250,137,107,0.1)',
            border: '1px solid rgba(250,137,107,0.3)',
            color: 'var(--danger)',
            fontSize: 13,
            marginBottom: 16,
          }}
        >
          {error}
        </div>
      )}

      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Search by name, email, or role..."
          style={{
            flex: '1 1 280px', maxWidth: 360,
            padding: '8px 14px 8px 36px',
            borderRadius: 7, border: '1px solid var(--border)', background: 'var(--bg)',
            color: 'var(--text)', fontSize: 13, fontFamily: 'inherit', outline: 'none',
            backgroundImage: "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%237c8fac' stroke-width='2'><circle cx='11' cy='11' r='8'/><path d='M21 21l-4.35-4.35'/></svg>\")",
            backgroundRepeat: 'no-repeat', backgroundPosition: '12px center',
          }}
        />
        <div style={{ display: 'flex', gap: 4, padding: 4, background: 'var(--bg)', border: '1px solid var(--border)', borderRadius: 7 }}>
          {(['all','active','inactive'] as const).map((s) => (
            <button
              key={s}
              onClick={() => { setStatusFilter(s); setPage(1); }}
              type="button"
              style={{
                padding: '6px 14px', borderRadius: 5, fontSize: 12, fontWeight: 600,
                border: 'none', cursor: 'pointer', fontFamily: 'inherit',
                background: statusFilter === s ? 'rgba(93,135,255,0.15)' : 'transparent',
                color: statusFilter === s ? 'var(--primary)' : 'var(--muted)',
                textTransform: 'capitalize',
              }}
            >
              {s}
            </button>
          ))}
        </div>
        <span style={{ marginLeft: 'auto', color: 'var(--muted)', fontSize: 12 }}>
          {total > 0 ? `${total} ${total === 1 ? 'admin' : 'admins'}` : ''}
        </span>
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        rows={users}
        loading={loading}
        page={page}
        totalPages={Math.max(1, Math.ceil(total / pageSize))}
        onPageChange={setPage}
        total={total}
        pageSize={pageSize}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        sortBy={sortBy}
        sortOrder={sortOrder}
        onSortChange={(by, order) => { setSortBy(by); setSortOrder(order); setPage(1); }}
        emptyMessage={search || statusFilter !== 'all' ? 'No matches.' : 'No admin users found.'}
      />

      {/* Add/Edit Modal */}
      {showModal && (
        <Modal
          title={editingUser ? 'Edit Admin User' : 'Add Admin User'}
          onClose={() => setShowModal(false)}
          footer={
            <>
              <button
                onClick={() => setShowModal(false)}
                style={{
                  padding: '8px 18px',
                  borderRadius: 7,
                  fontSize: 13,
                  fontWeight: 600,
                  border: '1px solid var(--border)',
                  background: 'transparent',
                  color: 'var(--muted)',
                  cursor: 'pointer',
                  fontFamily: 'inherit',
                }}
              >
                Cancel
              </button>
              <button
                onClick={saveUser}
                disabled={saving}
                style={{
                  padding: '8px 22px',
                  borderRadius: 7,
                  fontSize: 13,
                  fontWeight: 600,
                  border: 'none',
                  background: saving ? '#3a5bbf' : 'var(--primary)',
                  color: '#fff',
                  cursor: saving ? 'not-allowed' : 'pointer',
                  fontFamily: 'inherit',
                  transition: 'opacity 0.15s',
                }}
              >
                {saving ? 'Saving...' : 'Save'}
              </button>
            </>
          }
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {/* Name */}
            <div>
              <label style={labelStyle}>Name</label>
              <input
                style={inputStyle}
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="Full name"
              />
            </div>

            {/* Email */}
            <div>
              <label style={labelStyle}>Email</label>
              <input
                style={inputStyle}
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                placeholder="admin@example.com"
              />
            </div>

            {/* Password */}
            <div>
              <label style={labelStyle}>
                Password {editingUser && <span style={{ fontWeight: 400, textTransform: 'none' }}>(leave blank to keep current)</span>}
              </label>
              <input
                style={inputStyle}
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder={editingUser ? 'Leave blank to keep current' : 'Enter password'}
              />
            </div>

            {/* Role */}
            <div>
              <label style={labelStyle}>Role</label>
              <select
                style={{ ...inputStyle, appearance: 'auto' }}
                value={form.role}
                onChange={(e) => setForm({ ...form, role: e.target.value as 'admin' | 'super_admin' })}
              >
                <option value="admin">Admin</option>
                <option value="super_admin">Super Admin</option>
              </select>
            </div>

            {/* Active checkbox */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <input
                type="checkbox"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                style={{ width: 16, height: 16, accentColor: 'var(--primary)', cursor: 'pointer' }}
                id="admin-active-check"
              />
              <label htmlFor="admin-active-check" style={{ color: 'var(--text)', fontSize: 14, cursor: 'pointer' }}>
                Active
              </label>
            </div>
          </div>
        </Modal>
      )}

      {/* Toast */}
      {toast && (
        <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />
      )}
    </div>
  );
}
