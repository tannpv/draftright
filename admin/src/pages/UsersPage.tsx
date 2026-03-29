import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import DataTable from '../components/DataTable';
import { apiFetch } from '../api';

interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  plan: string;
  is_active: boolean;
  usage_today: number;
  created_at: string;
  [key: string]: unknown;
}

interface UsersResponse {
  users: User[];
  total: number;
}

const LIMIT = 20;

export default function UsersPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<User[]>([]);
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        search,
        page: String(page),
        limit: String(LIMIT),
      });
      const data = await apiFetch(`/admin/users?${params}`) as UsersResponse;
      setUsers(data.users ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users');
    } finally {
      setLoading(false);
    }
  }, [search, page]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleSearch = (value: string) => {
    setSearch(value);
    setPage(1);
  };

  const totalPages = Math.max(1, Math.ceil(total / LIMIT));

  const columns = [
    { header: 'Email', key: 'email' },
    { header: 'Name', key: 'name' },
    {
      header: 'Plan',
      key: 'plan',
      render: (row: User) => (
        <span className="badge badge-primary">{row.plan || '—'}</span>
      ),
    },
    {
      header: 'Usage Today',
      key: 'usage_today',
      render: (row: User) => (
        <span style={{ color: '#7c8fac' }}>{String(row.usage_today ?? 0)}</span>
      ),
    },
    {
      header: 'Status',
      key: 'is_active',
      render: (row: User) => (
        <span className={`badge ${row.is_active ? 'badge-success' : 'badge-muted'}`}>
          {row.is_active ? 'Active' : 'Inactive'}
        </span>
      ),
    },
    {
      header: 'Joined',
      key: 'created_at',
      render: (row: User) => (
        <span style={{ color: '#7c8fac' }}>
          {row.created_at ? new Date(row.created_at).toLocaleDateString() : '—'}
        </span>
      ),
    },
  ];

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 24 }}>
        <div>
          <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Users</h1>
          <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>{total} total users</p>
        </div>
      </div>

      {error && (
        <div className="alert-error" style={{ marginBottom: 16 }}>{error}</div>
      )}

      {/* Search bar */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ position: 'relative', display: 'inline-block' }}>
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#7c8fac"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', pointerEvents: 'none' }}
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => handleSearch(e.target.value)}
            placeholder="Search by email or name..."
            className="dark-input"
            style={{ paddingLeft: 36, width: 320 }}
          />
        </div>
      </div>

      <DataTable<User>
        columns={columns}
        rows={users}
        onRowClick={(row) => navigate(`/users/${row.id}`)}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
        loading={loading}
        emptyMessage="No users found."
      />
    </div>
  );
}
