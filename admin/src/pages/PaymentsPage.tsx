import { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '../api';
import DataTable from '../components/DataTable';
import Toast from '../components/Toast';

/* ── Types ────────────────────────────────────────────── */

interface PaymentUser {
  email: string;
  name: string;
}

interface PaymentPlan {
  name: string;
  price_cents: number;
}

interface Payment {
  id: string;
  user_id: string;
  user: PaymentUser;
  plan: PaymentPlan;
  amount: number;
  currency: string;
  method: string;
  status: string;
  reference_code: string;
  qr_data: string | null;
  notes: string | null;
  expires_at: string | null;
  completed_at: string | null;
  created_at: string;
}

interface PaymentsResponse {
  payments: Payment[];
  total: number;
}

interface PaymentStats {
  total: number;
  completed: number;
  pending: number;
  revenue: number;
}

/* ── Helpers ──────────────────────────────────────────── */

const vndFmt = new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' });

function formatVND(amount: number): string {
  return vndFmt.format(amount);
}

function methodBadge(method: string): { icon: string; label: string } {
  switch (method) {
    case 'stripe':        return { icon: '\uD83D\uDCB3', label: 'Stripe' };
    case 'paypal':        return { icon: '\uD83C\uDD7F\uFE0F', label: 'PayPal' };
    case 'vietqr':        return { icon: '\uD83D\uDCF1', label: 'VietQR' };
    case 'bank_transfer': return { icon: '\uD83C\uDFE6', label: 'Bank Transfer' };
    default:              return { icon: '\uD83D\uDCB0', label: method };
  }
}

function statusStyle(status: string): { color: string; bg: string } {
  switch (status) {
    case 'pending':   return { color: '#ffae1f', bg: 'rgba(255,174,31,0.12)' };
    case 'completed': return { color: '#13deb9', bg: 'rgba(19,222,185,0.12)' };
    case 'failed':    return { color: '#fa896b', bg: 'rgba(250,137,107,0.12)' };
    case 'expired':   return { color: '#7c8fac', bg: 'rgba(124,143,172,0.12)' };
    default:          return { color: '#7c8fac', bg: 'rgba(124,143,172,0.12)' };
  }
}

/* ── Filter tabs ──────────────────────────────────────── */

type StatusFilter = 'all' | 'pending' | 'completed' | 'failed';

const FILTER_TABS: { key: StatusFilter; label: string }[] = [
  { key: 'all',       label: 'All' },
  { key: 'pending',   label: 'Pending' },
  { key: 'completed', label: 'Completed' },
  { key: 'failed',    label: 'Failed' },
];

const PAGE_SIZE = 20;

/* ── Component ────────────────────────────────────────── */

export default function PaymentsPage() {
  const [payments, setPayments] = useState<Payment[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [stats, setStats] = useState<PaymentStats | null>(null);

  // Modal state
  const [confirmPayment, setConfirmPayment] = useState<Payment | null>(null);
  const [confirmNotes, setConfirmNotes] = useState('');
  const [confirming, setConfirming] = useState(false);

  // Toast state
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  /* ── Fetch stats ──────────────────────────────────── */
  const fetchStats = useCallback(async () => {
    try {
      const data = await apiFetch('/admin/payments/stats') as PaymentStats;
      setStats(data);
    } catch {
      // Non-critical — stats card just won't show
    }
  }, []);

  /* ── Fetch payments ───────────────────────────────── */
  const fetchPayments = useCallback(async (status: StatusFilter, p: number) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(p), limit: String(PAGE_SIZE) });
      if (status !== 'all') params.set('status', status);
      const data = await apiFetch(`/admin/payments?${params.toString()}`) as PaymentsResponse;
      setPayments(data.payments);
      setTotal(data.total);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load payments');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  useEffect(() => {
    fetchPayments(statusFilter, page);
  }, [fetchPayments, statusFilter, page]);

  /* ── Confirm payment ──────────────────────────────── */
  async function handleConfirm() {
    if (!confirmPayment) return;
    setConfirming(true);
    try {
      await apiFetch(`/admin/payments/${confirmPayment.id}/confirm`, {
        method: 'POST',
        body: JSON.stringify({ notes: confirmNotes || undefined }),
      });
      setToast({ message: `Payment ${confirmPayment.reference_code} confirmed.`, type: 'success' });
      setConfirmPayment(null);
      setConfirmNotes('');
      fetchPayments(statusFilter, page);
      fetchStats();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to confirm payment', type: 'error' });
    } finally {
      setConfirming(false);
    }
  }

  function handleFilterChange(f: StatusFilter) {
    setStatusFilter(f);
    setPage(1);
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  /* ── Table columns ────────────────────────────────── */
  const columns = [
    {
      header: 'Reference',
      key: 'reference_code',
      render: (row: Payment) => (
        <span style={{ fontFamily: 'monospace', color: '#5d87ff', fontSize: 13 }}>
          {row.reference_code}
        </span>
      ),
    },
    {
      header: 'User',
      key: 'user',
      render: (row: Payment) => (
        <div>
          <p style={{ color: '#eaeff4', fontSize: 14, fontWeight: 500, margin: 0, lineHeight: 1.3 }}>
            {row.user?.name || row.user?.email || '—'}
          </p>
          {row.user?.name && row.user?.email && (
            <p style={{ color: '#7c8fac', fontSize: 12, margin: 0 }}>{row.user.email}</p>
          )}
        </div>
      ),
    },
    {
      header: 'Plan',
      key: 'plan',
      render: (row: Payment) => (
        <span style={{ color: '#eaeff4', fontSize: 14 }}>{row.plan?.name || '—'}</span>
      ),
    },
    {
      header: 'Amount',
      key: 'amount',
      render: (row: Payment) => (
        <span style={{ color: '#13deb9', fontSize: 14, fontWeight: 600 }}>
          {formatVND(row.amount)}
        </span>
      ),
    },
    {
      header: 'Method',
      key: 'method',
      render: (row: Payment) => {
        const m = methodBadge(row.method);
        return (
          <span
            style={{
              display: 'inline-block',
              padding: '3px 10px',
              borderRadius: 4,
              fontSize: 12,
              fontWeight: 600,
              background: 'rgba(93,135,255,0.1)',
              color: '#5d87ff',
              whiteSpace: 'nowrap',
            }}
          >
            {m.icon} {m.label}
          </span>
        );
      },
    },
    {
      header: 'Status',
      key: 'status',
      render: (row: Payment) => {
        const s = statusStyle(row.status);
        return (
          <span
            style={{
              display: 'inline-block',
              padding: '3px 10px',
              borderRadius: 4,
              fontSize: 12,
              fontWeight: 600,
              background: s.bg,
              color: s.color,
              textTransform: 'capitalize' as const,
            }}
          >
            {row.status}
          </span>
        );
      },
    },
    {
      header: 'Date',
      key: 'created_at',
      render: (row: Payment) => (
        <span style={{ color: '#7c8fac', fontSize: 13, whiteSpace: 'nowrap' }}>
          {new Date(row.created_at).toLocaleDateString()}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: Payment) =>
        row.status === 'pending' ? (
          <button
            className="btn btn-primary btn-sm"
            onClick={(e) => {
              e.stopPropagation();
              setConfirmPayment(row);
              setConfirmNotes('');
            }}
          >
            Confirm
          </button>
        ) : null,
    },
  ];

  /* ── Stat card helper ─────────────────────────────── */
  function StatCard({ label, value, color }: { label: string; value: string | number; color?: string }) {
    return (
      <div
        style={{
          flex: '1 1 200px',
          background: '#2a3547',
          borderRadius: 7,
          padding: '20px 22px',
          minWidth: 180,
        }}
      >
        <p style={{ color: '#7c8fac', fontSize: 12, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', margin: '0 0 6px' }}>
          {label}
        </p>
        <p style={{ color: color || '#eaeff4', fontSize: 24, fontWeight: 700, margin: 0 }}>
          {value}
        </p>
      </div>
    );
  }

  /* ── Render ────────────────────────────────────────── */
  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Payments</h1>
        <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>
          Manual and QR payment management
        </p>
      </div>

      {/* Stats row */}
      {stats && (
        <div style={{ display: 'flex', gap: 16, marginBottom: 24, flexWrap: 'wrap' }}>
          <StatCard label="Total Payments" value={stats.total} />
          <StatCard label="Completed" value={stats.completed} color="#13deb9" />
          <StatCard label="Pending" value={stats.pending} color="#ffae1f" />
          <StatCard label="Revenue" value={formatVND(stats.revenue)} color="#13deb9" />
        </div>
      )}

      {/* Filter tabs */}
      <div style={{ display: 'flex', gap: 4, marginBottom: 20 }}>
        {FILTER_TABS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => handleFilterChange(tab.key)}
            style={{
              padding: '7px 18px',
              borderRadius: 7,
              fontSize: 13,
              fontWeight: 600,
              fontFamily: 'inherit',
              border: 'none',
              cursor: 'pointer',
              transition: 'all 0.15s',
              background: statusFilter === tab.key ? 'rgba(93,135,255,0.15)' : 'transparent',
              color: statusFilter === tab.key ? '#5d87ff' : '#7c8fac',
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 20 }}>{error}</div>}

      {/* Data table */}
      <DataTable
        columns={columns}
        rows={payments}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
        loading={loading}
        emptyMessage="No payments found."
      />

      {/* ── Confirm Modal ─────────────────────────────── */}
      {confirmPayment && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.55)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
          onClick={() => { if (!confirming) setConfirmPayment(null); }}
        >
          <div
            style={{
              background: '#2a3547',
              borderRadius: 10,
              padding: '28px',
              width: '100%',
              maxWidth: 460,
              boxShadow: '0 12px 40px rgba(0,0,0,0.4)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <h2 style={{ color: '#eaeff4', fontSize: 18, fontWeight: 700, margin: '0 0 18px' }}>
              Confirm Payment
            </h2>

            {/* Payment details */}
            <div style={{ marginBottom: 18 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '8px 16px', fontSize: 14 }}>
                <span style={{ color: '#7c8fac' }}>Reference:</span>
                <span style={{ color: '#5d87ff', fontFamily: 'monospace' }}>{confirmPayment.reference_code}</span>

                <span style={{ color: '#7c8fac' }}>User:</span>
                <span style={{ color: '#eaeff4' }}>{confirmPayment.user?.email || '—'}</span>

                <span style={{ color: '#7c8fac' }}>Amount:</span>
                <span style={{ color: '#13deb9', fontWeight: 600 }}>{formatVND(confirmPayment.amount)}</span>

                <span style={{ color: '#7c8fac' }}>Method:</span>
                <span style={{ color: '#eaeff4' }}>
                  {methodBadge(confirmPayment.method).icon} {methodBadge(confirmPayment.method).label}
                </span>
              </div>
            </div>

            {/* Notes textarea */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', color: '#7c8fac', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Notes (optional)
              </label>
              <textarea
                className="dark-input"
                value={confirmNotes}
                onChange={(e) => setConfirmNotes(e.target.value)}
                rows={3}
                placeholder="Add notes about this confirmation..."
                style={{ width: '100%', resize: 'vertical', fontSize: 13, boxSizing: 'border-box' }}
              />
            </div>

            {/* Buttons */}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
              <button
                className="btn btn-sm"
                style={{
                  background: 'transparent',
                  border: '1px solid #333f55',
                  color: '#7c8fac',
                  padding: '8px 18px',
                  borderRadius: 7,
                  fontSize: 13,
                  fontFamily: 'inherit',
                  cursor: 'pointer',
                }}
                onClick={() => setConfirmPayment(null)}
                disabled={confirming}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleConfirm}
                disabled={confirming}
                style={{ padding: '8px 22px' }}
              >
                {confirming ? 'Confirming...' : 'Confirm Payment'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
}
