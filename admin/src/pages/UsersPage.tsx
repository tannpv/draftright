import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import DataTable from '../components/DataTable';
import { apiFetch } from '../api';

interface User {
  id: string;
  email: string;
  name: string;
  plan: string;
  usageToday: number;
  status: string;
  createdAt: string;
  [key: string]: unknown;
}

interface UsersResponse {
  users: User[];
  total: number;
  page: number;
  limit: number;
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

  // Reset page on search change
  const handleSearch = (value: string) => {
    setSearch(value);
    setPage(1);
  };

  const totalPages = Math.max(1, Math.ceil(total / LIMIT));

  const columns = [
    { header: 'Email', key: 'email' },
    { header: 'Name', key: 'name' },
    { header: 'Plan', key: 'plan' },
    {
      header: 'Usage Today',
      key: 'usageToday',
      render: (row: User) => String(row.usageToday ?? 0),
    },
    {
      header: 'Status',
      key: 'status',
      render: (row: User) => (
        <span
          className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${
            row.status === 'active'
              ? 'bg-green-100 text-green-700'
              : 'bg-gray-100 text-gray-600'
          }`}
        >
          {row.status}
        </span>
      ),
    },
    {
      header: 'Joined',
      key: 'createdAt',
      render: (row: User) =>
        row.createdAt ? new Date(row.createdAt).toLocaleDateString() : '—',
    },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Users</h1>
          <p className="text-gray-500 text-sm mt-1">{total} total users</p>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      {/* Search */}
      <div className="mb-4">
        <input
          type="text"
          value={search}
          onChange={(e) => handleSearch(e.target.value)}
          placeholder="Search by email or name..."
          className="w-full max-w-md border border-gray-300 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
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
