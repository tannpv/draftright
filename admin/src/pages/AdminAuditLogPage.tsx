import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import { apiFetch } from '../api';

interface AuditRow {
  id: string;
  actor_admin_id: string;
  actor_email: string;
  target_admin_id: string;
  target_email: string;
  created_at: string;
  [key: string]: unknown;
}

export default function AdminAuditLogPage() {
  const [rows, setRows] = useState<AuditRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [total, setTotal] = useState(0);

  const fetchRows = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        limit: String(pageSize),
        offset: String((page - 1) * pageSize),
      });
      const data = await apiFetch(`/admin/admin-user-audit?${params}`) as { rows: AuditRow[]; total: number };
      setRows(data.rows ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audit log');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize]);

  useEffect(() => { fetchRows(); }, [fetchRows]);

  const columns = [
    {
      header: 'When',
      key: 'created_at',
      render: (row: AuditRow) => (
        <span style={{ color: 'var(--muted)', fontSize: 13 }}>{new Date(row.created_at).toLocaleString()}</span>
      ),
    },
    {
      header: 'Actor',
      key: 'actor_email',
      render: (row: AuditRow) => (
        <span style={{ color: 'var(--text)', fontWeight: 600 }}>{row.actor_email}</span>
      ),
    },
    {
      header: 'Deactivated',
      key: 'target_email',
      render: (row: AuditRow) => (
        <span style={{ color: 'var(--text)' }}>{row.target_email}</span>
      ),
    },
  ];

  return (
    <div>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 600, margin: 0 }}>Admin Audit Log</h1>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: '4px 0 0' }}>
          Admin-account deactivations, newest first.
        </p>
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

      {/* Table */}
      <DataTable
        columns={columns}
        rows={rows}
        loading={loading}
        page={page}
        totalPages={total === 0 ? 0 : Math.max(1, Math.ceil(total / pageSize))}
        onPageChange={setPage}
        total={total}
        pageSize={pageSize}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        emptyMessage="No admin deactivations recorded."
      />
    </div>
  );
}
