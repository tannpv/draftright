import { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiFetch } from '../api';

interface Transaction {
  id: string;
  user_email: string;
  user_name: string;
  user_id: string;
  plan_name: string;
  price_cents: number;
  store_type: string;
  store_transaction_id: string | null;
  status: string;
  started_at: string;
  expires_at: string | null;
  created_at: string;
}

interface TransactionsResponse {
  transactions: Transaction[];
  total: number;
}

function formatCents(cents: number): string {
  if (cents === 0) return 'Free';
  return `$${(cents / 100).toFixed(2)}`;
}

function storeLabel(store_type: string): { label: string; color: string; bg: string } {
  switch (store_type) {
    case 'google_play':  return { label: 'Google Play', color: '#13deb9', bg: 'rgba(19,222,185,0.12)' };
    case 'apple_iap':    return { label: 'Apple IAP',   color: '#49beff', bg: 'rgba(73,190,255,0.12)' };
    case 'admin_granted': return { label: 'Admin Granted', color: '#ffae1f', bg: 'rgba(255,174,31,0.12)' };
    default: return { label: store_type, color: '#7c8fac', bg: 'rgba(124,143,172,0.12)' };
  }
}

function statusStyle(status: string): { color: string; bg: string } {
  switch (status) {
    case 'active':    return { color: '#13deb9', bg: 'rgba(19,222,185,0.12)' };
    case 'cancelled': return { color: '#fa896b', bg: 'rgba(250,137,107,0.12)' };
    case 'expired':   return { color: '#7c8fac', bg: 'rgba(124,143,172,0.12)' };
    default:          return { color: '#7c8fac', bg: 'rgba(124,143,172,0.12)' };
  }
}

const PAGE_SIZE = 20;

export default function TransactionsPage() {
  const navigate = useNavigate();
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchTransactions = useCallback(async (s: string, p: number) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(p), limit: String(PAGE_SIZE) });
      if (s) params.set('search', s);
      const data = await apiFetch(`/admin/transactions?${params.toString()}`) as TransactionsResponse;
      setTransactions(data.transactions);
      setTotal(data.total);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load transactions');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTransactions(search, page);
  }, [fetchTransactions, search, page]);

  function handleSearchChange(val: string) {
    setSearchInput(val);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setSearch(val);
      setPage(1);
    }, 400);
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: 24, display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap', gap: 16 }}>
        <div>
          <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Transactions</h1>
          <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>
            All subscription records — {total} total
          </p>
        </div>

        {/* Search */}
        <div style={{ position: 'relative', minWidth: 260 }}>
          <svg
            style={{ position: 'absolute', left: 11, top: '50%', transform: 'translateY(-50%)', pointerEvents: 'none' }}
            width="14" height="14" viewBox="0 0 24 24" fill="none"
            stroke="#7c8fac" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            type="text"
            placeholder="Search by email or name..."
            value={searchInput}
            onChange={(e) => handleSearchChange(e.target.value)}
            className="dark-input"
            style={{ paddingLeft: 34, minWidth: 260, fontSize: 13 }}
          />
        </div>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 20 }}>{error}</div>}

      {/* Table */}
      <div style={{ background: '#2a3547', borderRadius: 7, overflow: 'hidden' }}>
        {loading ? (
          <div style={{ padding: '48px 22px', textAlign: 'center', color: '#7c8fac', fontSize: 13 }}>
            Loading...
          </div>
        ) : transactions.length === 0 ? (
          <div style={{ padding: '48px 22px', textAlign: 'center', color: '#7c8fac', fontSize: 13 }}>
            No transactions found.
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #333f55' }}>
                  {['User', 'Plan', 'Amount', 'Store', 'Status', 'Started', 'Expires'].map(h => (
                    <th
                      key={h}
                      style={{
                        padding: '12px 16px',
                        textAlign: 'left',
                        color: '#7c8fac',
                        fontSize: 12,
                        fontWeight: 700,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {transactions.map((tx) => {
                  const store = storeLabel(tx.store_type);
                  const status = statusStyle(tx.status);
                  return (
                    <tr
                      key={tx.id}
                      onClick={() => navigate(`/users/${tx.user_id}`)}
                      style={{
                        borderBottom: '1px solid #333f55',
                        cursor: 'pointer',
                        transition: 'background 0.15s',
                      }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'rgba(93,135,255,0.05)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'transparent'; }}
                    >
                      {/* User */}
                      <td style={{ padding: '13px 16px', minWidth: 160 }}>
                        <p style={{ color: '#eaeff4', fontSize: 14, fontWeight: 500, margin: 0, lineHeight: 1.3 }}>
                          {tx.user_name !== '—' ? tx.user_name : tx.user_email}
                        </p>
                        {tx.user_name !== '—' && (
                          <p style={{ color: '#7c8fac', fontSize: 12, margin: 0 }}>{tx.user_email}</p>
                        )}
                      </td>

                      {/* Plan */}
                      <td style={{ padding: '13px 16px', color: '#eaeff4', fontSize: 14, whiteSpace: 'nowrap' }}>
                        {tx.plan_name}
                      </td>

                      {/* Amount */}
                      <td style={{ padding: '13px 16px', color: tx.price_cents === 0 ? '#7c8fac' : '#13deb9', fontSize: 14, fontWeight: 600, whiteSpace: 'nowrap' }}>
                        {formatCents(tx.price_cents)}
                      </td>

                      {/* Store badge */}
                      <td style={{ padding: '13px 16px' }}>
                        <span
                          style={{
                            display: 'inline-block',
                            padding: '3px 10px',
                            borderRadius: 4,
                            fontSize: 12,
                            fontWeight: 600,
                            background: store.bg,
                            color: store.color,
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {store.label}
                        </span>
                      </td>

                      {/* Status badge */}
                      <td style={{ padding: '13px 16px' }}>
                        <span
                          style={{
                            display: 'inline-block',
                            padding: '3px 10px',
                            borderRadius: 4,
                            fontSize: 12,
                            fontWeight: 600,
                            background: status.bg,
                            color: status.color,
                            textTransform: 'capitalize',
                          }}
                        >
                          {tx.status}
                        </span>
                      </td>

                      {/* Started */}
                      <td style={{ padding: '13px 16px', color: '#7c8fac', fontSize: 13, whiteSpace: 'nowrap' }}>
                        {new Date(tx.started_at).toLocaleDateString()}
                      </td>

                      {/* Expires */}
                      <td style={{ padding: '13px 16px', color: '#7c8fac', fontSize: 13, whiteSpace: 'nowrap' }}>
                        {tx.expires_at ? new Date(tx.expires_at).toLocaleDateString() : '—'}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && totalPages > 1 && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '14px 22px',
              borderTop: '1px solid #333f55',
            }}
          >
            <span style={{ color: '#7c8fac', fontSize: 13 }}>
              Page {page} of {totalPages} ({total} records)
            </span>
            <div style={{ display: 'flex', gap: 8 }}>
              <button
                disabled={page <= 1}
                onClick={() => setPage(p => p - 1)}
                className="btn btn-ghost btn-sm"
                style={{ opacity: page <= 1 ? 0.4 : 1 }}
              >
                Previous
              </button>
              <button
                disabled={page >= totalPages}
                onClick={() => setPage(p => p + 1)}
                className="btn btn-ghost btn-sm"
                style={{ opacity: page >= totalPages ? 0.4 : 1 }}
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
