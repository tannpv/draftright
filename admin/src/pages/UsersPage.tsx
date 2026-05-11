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

export default function UsersPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<User[]>([]);
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [sortBy, setSortBy] = useState<string>('created_at');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        search,
        page: String(page),
        limit: String(pageSize),
        status: statusFilter,
        sort_by: sortBy,
        sort_order: sortOrder,
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
  }, [search, page, pageSize, statusFilter, sortBy, sortOrder]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  const columns = [
    { header: 'Email', key: 'email', sortKey: 'email' },
    { header: 'Name', key: 'name', sortKey: 'name' },
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
      sortKey: 'is_active',
      render: (row: User) => (
        <span className={`badge ${row.is_active ? 'badge-success' : 'badge-muted'}`}>
          {row.is_active ? 'Active' : 'Inactive'}
        </span>
      ),
    },
    {
      header: 'Joined',
      key: 'created_at',
      sortKey: 'created_at',
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

      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Search by email or name..."
          style={{
            flex: '1 1 280px', maxWidth: 360,
            padding: '8px 14px 8px 36px',
            borderRadius: 7, border: '1px solid #333f55', background: '#202936',
            color: '#eaeff4', fontSize: 13, fontFamily: 'inherit', outline: 'none',
            backgroundImage: "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%237c8fac' stroke-width='2'><circle cx='11' cy='11' r='8'/><path d='M21 21l-4.35-4.35'/></svg>\")",
            backgroundRepeat: 'no-repeat', backgroundPosition: '12px center',
          }}
        />
        <div style={{ display: 'flex', gap: 4, padding: 4, background: '#202936', border: '1px solid #333f55', borderRadius: 7 }}>
          {(['all','active','inactive'] as const).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => { setStatusFilter(s); setPage(1); }}
              style={{
                padding: '6px 14px', borderRadius: 5, fontSize: 12, fontWeight: 600,
                border: 'none', cursor: 'pointer', fontFamily: 'inherit',
                background: statusFilter === s ? 'rgba(93,135,255,0.15)' : 'transparent',
                color: statusFilter === s ? '#5d87ff' : '#7c8fac',
                textTransform: 'capitalize',
              }}
            >
              {s}
            </button>
          ))}
        </div>
        <span style={{ marginLeft: 'auto', color: '#7c8fac', fontSize: 12 }}>
          {total > 0 ? `${total} ${total === 1 ? 'user' : 'users'}` : ''}
        </span>
      </div>

      <DataTable<User>
        columns={columns}
        rows={users}
        onRowClick={(row) => navigate(`/users/${row.id}`)}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
        total={total}
        pageSize={pageSize}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        sortBy={sortBy}
        sortOrder={sortOrder}
        onSortChange={(by, order) => { setSortBy(by); setSortOrder(order); setPage(1); }}
        loading={loading}
        emptyMessage="No users found."
      />
    </div>
  );
}
