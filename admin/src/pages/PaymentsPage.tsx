import { useState, useEffect, useCallback } from 'react';
import { apiFetch, SEARCH_DEBOUNCE_MS, DEFAULT_PAGE_SIZE } from '../api';
import DataTable from '../components/DataTable';
import Toast from '../components/Toast';
import { formatCurrency } from '../lib/format';
import { toneStyle, type Tone } from '../lib/status';

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

function methodBadge(method: string): { icon: string; label: string } {
  switch (method) {
    case 'stripe':        return { icon: '\uD83D\uDCB3', label: 'Stripe' };
    case 'paypal':        return { icon: '\uD83C\uDD7F\uFE0F', label: 'PayPal' };
    case 'vietqr':        return { icon: '\uD83D\uDCF1', label: 'VietQR' };
    case 'bank_transfer': return { icon: '\uD83C\uDFE6', label: 'Bank Transfer' };
    default:              return { icon: '\uD83D\uDCB0', label: method };
  }
}

const PAYMENT_TONE: Record<string, Tone> = {
  pending: 'warning', completed: 'success', failed: 'danger', refunded: 'info', expired: 'muted',
};
const statusStyle = (status: string) => toneStyle(PAYMENT_TONE[status] ?? 'muted');

/* ── Filter tabs ──────────────────────────────────────── */

type StatusFilter = 'all' | 'pending' | 'completed' | 'failed' | 'refunded';

const FILTER_TABS: { key: StatusFilter; label: string }[] = [
  { key: 'all',       label: 'All' },
  { key: 'pending',   label: 'Pending' },
  { key: 'completed', label: 'Completed' },
  { key: 'failed',    label: 'Failed' },
  { key: 'refunded',  label: 'Refunded' },
];

/* ── Provider modes ───────────────────────────────────────
 * The settings field holding each payment method's sandbox/live mode.
 * vietqr + bank_transfer settle via SePay, so they share `sepay_mode`.
 * Add a new method → add one entry here (nothing else to change). */
const METHOD_MODE_FIELD: Record<string, string> = {
  stripe: 'stripe_mode',
  paypal: 'paypal_mode',
  momo: 'momo_mode',
  vietqr: 'sepay_mode',
  bank_transfer: 'sepay_mode',
};
const MODE_FIELDS = [...new Set(Object.values(METHOD_MODE_FIELD))];
const SANDBOX_MODES = new Set(['sandbox', 'test']);
const isSandboxMode = (mode?: string) => SANDBOX_MODES.has(mode ?? '');


/* ── Component ────────────────────────────────────────── */

export default function PaymentsPage() {
  const [payments, setPayments] = useState<Payment[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [sortBy, setSortBy] = useState<string>('created_at');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [stats, setStats] = useState<PaymentStats | null>(null);
  // Per-provider modes (sandbox/test vs live), from settings. A payment whose
  // provider is in sandbox/test gets a "Simulate paid" button.
  const [modes, setModes] = useState<Record<string, string>>({});

  // A payment is simulatable when its provider's mode is sandbox/test.
  const isSandbox = (method: string) => isSandboxMode(modes[METHOD_MODE_FIELD[method] ?? 'sepay_mode']);
  const anySandbox = MODE_FIELDS.some((f) => isSandboxMode(modes[f]));

  // Modal state
  const [confirmPayment, setConfirmPayment] = useState<Payment | null>(null);
  const [confirmNotes, setConfirmNotes] = useState('');
  const [confirming, setConfirming] = useState(false);

  // Refund modal state
  const [refundPayment, setRefundPayment] = useState<Payment | null>(null);
  const [refundReason, setRefundReason] = useState('requested_by_customer');
  const [refunding, setRefunding] = useState(false);

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
  const fetchPayments = useCallback(async (status: StatusFilter, p: number, limit: number) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(p), limit: String(limit),
        sort_by: sortBy, sort_order: sortOrder,
      });
      if (status !== 'all') params.set('status', status);
      if (search) params.set('search', search);
      const data = await apiFetch(`/admin/payments?${params.toString()}`) as PaymentsResponse;
      setPayments(data.payments);
      setTotal(data.total);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load payments');
    } finally {
      setLoading(false);
    }
  }, [search, sortBy, sortOrder]);

  useEffect(() => {
    fetchStats();
    // Load per-provider modes so the admin knows when "Confirm" = simulate.
    apiFetch('/admin/settings')
      .then((s) => {
        const x = s as Record<string, string>;
        setModes(Object.fromEntries(MODE_FIELDS.map((f) => [f, x[f]])));
      })
      .catch(() => {});
  }, [fetchStats]);

  useEffect(() => {
    fetchPayments(statusFilter, page, pageSize);
  }, [fetchPayments, statusFilter, page, pageSize]);

  // Debounce search input.
  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [searchInput]);

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
      fetchPayments(statusFilter, page, pageSize);
      fetchStats();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to confirm payment', type: 'error' });
    } finally {
      setConfirming(false);
    }
  }

  /* ── Refund payment ───────────────────────────────── */
  async function handleRefund() {
    if (!refundPayment) return;
    setRefunding(true);
    try {
      await apiFetch(`/admin/payments/${refundPayment.id}/refund`, {
        method: 'POST',
        body: JSON.stringify({ reason: refundReason }),
      });
      setToast({ message: `Refunded ${refundPayment.reference_code}. Subscription cancelled.`, type: 'success' });
      setRefundPayment(null);
      fetchPayments(statusFilter, page, pageSize);
      fetchStats();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to refund', type: 'error' });
    } finally {
      setRefunding(false);
    }
  }

  function handleFilterChange(f: StatusFilter) {
    setStatusFilter(f);
    setPage(1);
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  /* ── Table columns ────────────────────────────────── */
  const columns = [
    {
      header: 'Reference',
      key: 'reference_code',
      sortKey: 'reference_code',
      render: (row: Payment) => (
        <span style={{ fontFamily: 'monospace', color: 'var(--primary)', fontSize: 13 }}>
          {row.reference_code}
        </span>
      ),
    },
    {
      header: 'User',
      key: 'user',
      sortKey: 'user.email',
      render: (row: Payment) => (
        <div>
          <p style={{ color: 'var(--text)', fontSize: 14, fontWeight: 500, margin: 0, lineHeight: 1.3 }}>
            {row.user?.name || row.user?.email || '—'}
          </p>
          {row.user?.name && row.user?.email && (
            <p style={{ color: 'var(--muted)', fontSize: 12, margin: 0 }}>{row.user.email}</p>
          )}
        </div>
      ),
    },
    {
      header: 'Plan',
      key: 'plan',
      render: (row: Payment) => (
        <span style={{ color: 'var(--text)', fontSize: 14 }}>{row.plan?.name || '—'}</span>
      ),
    },
    {
      header: 'Amount',
      key: 'amount',
      sortKey: 'amount',
      render: (row: Payment) => (
        <span style={{ color: 'var(--success)', fontSize: 14, fontWeight: 600 }}>
          {formatCurrency(row.amount, row.currency)}
        </span>
      ),
    },
    {
      header: 'Method',
      key: 'method',
      sortKey: 'method',
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
              color: 'var(--primary)',
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
      sortKey: 'status',
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
      sortKey: 'created_at',
      render: (row: Payment) => (
        <span style={{ color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>
          {new Date(row.created_at).toLocaleDateString()}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: Payment) => (
        <div style={{ display: 'flex', gap: 6 }}>
          {row.status === 'pending' && (
            <button
              className="btn btn-primary btn-sm"
              onClick={(e) => {
                e.stopPropagation();
                setConfirmPayment(row);
                setConfirmNotes('');
              }}
            >
              {isSandbox(row.method) ? '🧪 Simulate paid' : 'Confirm'}
            </button>
          )}
          {row.status === 'completed' && row.method === 'stripe' && (
            <button
              className="btn btn-sm"
              onClick={(e) => {
                e.stopPropagation();
                setRefundPayment(row);
                setRefundReason('requested_by_customer');
              }}
              style={{ background: 'rgba(73,190,255,0.1)', color: 'var(--secondary)', border: '1px solid rgba(73,190,255,0.2)' }}
            >
              Refund
            </button>
          )}
        </div>
      ),
    },
  ];

  /* ── Stat card helper ─────────────────────────────── */
  function StatCard({ label, value, color }: { label: string; value: string | number; color?: string }) {
    return (
      <div
        style={{
          flex: '1 1 200px',
          background: 'var(--card)',
          borderRadius: 7,
          padding: '20px 22px',
          minWidth: 180,
        }}
      >
        <p style={{ color: 'var(--muted)', fontSize: 12, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', margin: '0 0 6px' }}>
          {label}
        </p>
        <p style={{ color: color || 'var(--text)', fontSize: 24, fontWeight: 700, margin: 0 }}>
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
        <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Payments</h1>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
          Manual and QR payment management
        </p>
      </div>

      {anySandbox && (
        <div style={{ marginBottom: 24, padding: '12px 16px', borderRadius: 8,
            border: '1px solid rgba(255,174,31,0.4)', background: 'rgba(255,174,31,0.08)', color: 'var(--warning)', fontSize: 13 }}>
          🧪 <strong>A payment provider is in sandbox/test mode.</strong> Pending payments on a sandbox provider show “Simulate paid” — completing one activates the subscription with no real charge. Set each provider to <strong>live</strong> in Settings → Payment to go live.
        </div>
      )}

      {/* Stats row */}
      {stats && (
        <div style={{ display: 'flex', gap: 16, marginBottom: 24, flexWrap: 'wrap' }}>
          <StatCard label="Total Payments" value={stats.total} />
          <StatCard label="Completed" value={stats.completed} color="var(--success)" />
          <StatCard label="Pending" value={stats.pending} color="var(--warning)" />
          <StatCard label="Revenue (mixed)" value={stats.revenue.toLocaleString('en-US')} color="var(--success)" />
        </div>
      )}

      {/* Toolbar — search + filter tabs */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 20, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Search by reference, email, plan..."
          style={{
            flex: '1 1 280px', maxWidth: 360,
            padding: '8px 14px 8px 36px',
            borderRadius: 7, border: '1px solid var(--border)', background: 'var(--bg)',
            color: 'var(--text)', fontSize: 13, fontFamily: 'inherit', outline: 'none',
            backgroundImage: "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%237c8fac' stroke-width='2'><circle cx='11' cy='11' r='8'/><path d='M21 21l-4.35-4.35'/></svg>\")",
            backgroundRepeat: 'no-repeat', backgroundPosition: '12px center',
          }}
        />
        <div style={{ display: 'flex', gap: 4 }}>
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
                color: statusFilter === tab.key ? 'var(--primary)' : 'var(--muted)',
              }}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 20 }}>{error}</div>}

      <DataTable
        columns={columns}
        rows={payments}
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
        emptyMessage={search ? `No matches for "${search}".` : 'No payments found.'}
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
              background: 'var(--card)',
              borderRadius: 10,
              padding: '28px',
              width: '100%',
              maxWidth: 460,
              boxShadow: '0 12px 40px rgba(0,0,0,0.4)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <h2 style={{ color: 'var(--text)', fontSize: 18, fontWeight: 700, margin: '0 0 18px' }}>
              Confirm Payment
            </h2>

            {/* Payment details */}
            <div style={{ marginBottom: 18 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '8px 16px', fontSize: 14 }}>
                <span style={{ color: 'var(--muted)' }}>Reference:</span>
                <span style={{ color: 'var(--primary)', fontFamily: 'monospace' }}>{confirmPayment.reference_code}</span>

                <span style={{ color: 'var(--muted)' }}>User:</span>
                <span style={{ color: 'var(--text)' }}>{confirmPayment.user?.email || '—'}</span>

                <span style={{ color: 'var(--muted)' }}>Amount:</span>
                <span style={{ color: 'var(--success)', fontWeight: 600 }}>{formatCurrency(confirmPayment.amount, confirmPayment.currency)}</span>

                <span style={{ color: 'var(--muted)' }}>Method:</span>
                <span style={{ color: 'var(--text)' }}>
                  {methodBadge(confirmPayment.method).icon} {methodBadge(confirmPayment.method).label}
                </span>
              </div>
            </div>

            {/* Notes textarea */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
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
                  border: '1px solid var(--border)',
                  color: 'var(--muted)',
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

      {/* ── Refund Modal ───────────────────────────────── */}
      {refundPayment && (
        <div
          style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.55)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}
          onClick={() => { if (!refunding) setRefundPayment(null); }}
        >
          <div
            style={{ background: 'var(--card)', borderRadius: 10, padding: 28, width: '100%', maxWidth: 460, boxShadow: '0 12px 40px rgba(0,0,0,0.4)' }}
            onClick={(e) => e.stopPropagation()}
          >
            <h2 style={{ color: 'var(--text)', fontSize: 18, fontWeight: 700, margin: '0 0 18px' }}>
              Refund Payment
            </h2>
            <div style={{ background: 'rgba(250,137,107,0.08)', border: '1px solid rgba(250,137,107,0.25)', borderRadius: 7, padding: 12, marginBottom: 18 }}>
              <p style={{ color: 'var(--danger)', fontSize: 12, margin: 0, lineHeight: 1.5 }}>
                ⚠ This issues a Stripe refund AND cancels the user's subscription immediately. They lose access right now.
              </p>
            </div>
            <div style={{ marginBottom: 18 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '8px 16px', fontSize: 14 }}>
                <span style={{ color: 'var(--muted)' }}>Reference:</span>
                <span style={{ color: 'var(--primary)', fontFamily: 'monospace' }}>{refundPayment.reference_code}</span>
                <span style={{ color: 'var(--muted)' }}>User:</span>
                <span style={{ color: 'var(--text)' }}>{refundPayment.user?.email || '—'}</span>
                <span style={{ color: 'var(--muted)' }}>Amount:</span>
                <span style={{ color: 'var(--success)', fontWeight: 600 }}>{formatCurrency(refundPayment.amount, refundPayment.currency)}</span>
              </div>
            </div>
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Reason
              </label>
              <select className="dark-input" value={refundReason} onChange={(e) => setRefundReason(e.target.value)} style={{ width: '100%' }}>
                <option value="requested_by_customer">Requested by customer</option>
                <option value="duplicate">Duplicate charge</option>
                <option value="fraudulent">Fraudulent</option>
              </select>
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
              <button
                className="btn btn-sm"
                style={{ background: 'transparent', border: '1px solid var(--border)', color: 'var(--muted)', padding: '8px 18px', borderRadius: 7, fontSize: 13, fontFamily: 'inherit', cursor: 'pointer' }}
                onClick={() => setRefundPayment(null)}
                disabled={refunding}
              >
                Cancel
              </button>
              <button
                onClick={handleRefund}
                disabled={refunding}
                style={{ padding: '8px 22px', background: 'var(--danger)', color: '#fff', border: 'none', borderRadius: 7, fontSize: 13, fontWeight: 600, fontFamily: 'inherit', cursor: refunding ? 'wait' : 'pointer' }}
              >
                {refunding ? 'Refunding...' : 'Issue Refund'}
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
