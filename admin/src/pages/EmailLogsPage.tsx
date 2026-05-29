import { useState, useEffect, useCallback } from 'react';
import { apiFetch, DEFAULT_PAGE_SIZE } from '../api';
import { timeAgo } from '../lib/format';
import { toneStyle, type Tone } from '../lib/status';

interface EmailLog {
  id: string;
  to_email: string;
  email_type: string;
  subject: string;
  status: string; // sent | failed | skipped
  provider_id: string | null;
  error: string | null;
  created_at: string;
}

const STATUS_TONE: Record<string, Tone> = { sent: 'success', failed: 'danger', skipped: 'muted' };
const FILTERS = ['all', 'sent', 'failed', 'skipped'] as const;

export default function EmailLogsPage() {
  const [rows, setRows] = useState<EmailLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState<(typeof FILTERS)[number]>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(page), limit: String(DEFAULT_PAGE_SIZE) });
      if (status !== 'all') params.set('status', status);
      const data = await apiFetch(`/admin/email-logs?${params}`) as { rows: EmailLog[]; total: number };
      setRows(data.rows ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load email logs');
    } finally {
      setLoading(false);
    }
  }, [page, status]);

  useEffect(() => { load(); }, [load]);

  const totalPages = Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE));

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Email Logs</h1>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
          Every email send attempt — sent, failed, or skipped (Resend not configured).
        </p>
      </div>

      <div style={{ display: 'flex', gap: 4, marginBottom: 16 }}>
        {FILTERS.map((f) => (
          <button key={f} onClick={() => { setStatus(f); setPage(1); }} style={{
            padding: '7px 16px', borderRadius: 7, fontSize: 13, fontWeight: 600, border: 'none', cursor: 'pointer',
            textTransform: 'capitalize', fontFamily: 'inherit',
            background: status === f ? 'rgba(93,135,255,0.15)' : 'transparent',
            color: status === f ? 'var(--primary)' : 'var(--muted)',
          }}>{f}</button>
        ))}
        <span style={{ marginLeft: 'auto', color: 'var(--muted)', fontSize: 12, alignSelf: 'center' }}>{total} total</span>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 16 }}>{error}</div>}

      <div style={{ background: 'var(--card)', borderRadius: 7, overflow: 'hidden' }}>
        {loading ? (
          <div style={{ padding: '48px', textAlign: 'center', color: 'var(--muted)', fontSize: 13 }}>Loading…</div>
        ) : rows.length === 0 ? (
          <div style={{ padding: '48px', textAlign: 'center', color: 'var(--muted)', fontSize: 13 }}>No emails yet.</div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid var(--border)' }}>
                  {['Time', 'To', 'Type', 'Subject', 'Status', 'Detail'].map(h => (
                    <th key={h} style={{ padding: '12px 16px', textAlign: 'left', color: 'var(--muted)', fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', whiteSpace: 'nowrap' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => {
                  const s = toneStyle(STATUS_TONE[r.status] ?? 'muted');
                  return (
                    <tr key={r.id} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '12px 16px', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }} title={new Date(r.created_at).toLocaleString()}>{timeAgo(r.created_at)}</td>
                      <td style={{ padding: '12px 16px', color: 'var(--text)', fontSize: 13 }}>{r.to_email}</td>
                      <td style={{ padding: '12px 16px', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>{r.email_type}</td>
                      <td style={{ padding: '12px 16px', color: 'var(--text)', fontSize: 13 }}>{r.subject}</td>
                      <td style={{ padding: '12px 16px' }}>
                        <span style={{ display: 'inline-block', padding: '3px 10px', borderRadius: 4, fontSize: 12, fontWeight: 600, textTransform: 'capitalize', background: s.bg, color: s.color }}>{r.status}</span>
                      </td>
                      <td style={{ padding: '12px 16px', color: r.error ? 'var(--danger)' : 'var(--muted)', fontSize: 12, maxWidth: 280, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={r.error ?? r.provider_id ?? ''}>
                        {r.error ?? r.provider_id ?? '—'}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {totalPages > 1 && (
        <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 16 }}>
          <button disabled={page <= 1} onClick={() => setPage(p => p - 1)} className="btn btn-sm">Prev</button>
          <span style={{ color: 'var(--muted)', fontSize: 13, alignSelf: 'center' }}>Page {page} / {totalPages}</span>
          <button disabled={page >= totalPages} onClick={() => setPage(p => p + 1)} className="btn btn-sm">Next</button>
        </div>
      )}
    </div>
  );
}
