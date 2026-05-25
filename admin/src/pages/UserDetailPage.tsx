import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch } from '../api';
import { formatCurrency } from '../lib/format';

interface UsageLog {
  id: string;
  created_at: string;
  tone: string;
  input_length: number;
  output_length: number;
  response_time_ms: number;
}

interface Subscription {
  plan: { name: string; daily_limit: number; billing_period: string };
  status: string;
  expires_at?: string;
}

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

interface UserEntity {
  id: string;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
}

interface UserDetailResponse {
  user: UserEntity;
  subscription: Subscription | null;
  usage_today: number;
  recent_usage: UsageLog[];
}

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

/* ── Reusable info row ──────────────────────────────────── */
function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: '10px 0',
        borderBottom: '1px solid var(--border)',
        fontSize: 14,
      }}
    >
      <span style={{ color: 'var(--muted)' }}>{label}</span>
      <span style={{ color: 'var(--text)', fontWeight: 500 }}>{children}</span>
    </div>
  );
}

/* ── Info card ──────────────────────────────────────────── */
function InfoCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ background: 'var(--card)', borderRadius: 7 }}>
      <div
        style={{
          padding: '16px 22px',
          borderBottom: '1px solid var(--border)',
        }}
      >
        <h3 style={{ color: 'var(--text)', fontSize: 15, fontWeight: 600, margin: 0 }}>{title}</h3>
      </div>
      <div style={{ padding: '4px 22px 12px' }}>{children}</div>
    </div>
  );
}

function storeBadge(store_type: string) {
  const map: Record<string, { label: string; color: string; bg: string }> = {
    google_play:   { label: 'Google Play',   color: 'var(--success)', bg: 'rgba(19,222,185,0.12)' },
    apple_iap:     { label: 'Apple IAP',     color: 'var(--secondary)', bg: 'rgba(73,190,255,0.12)' },
    admin_granted: { label: 'Admin Granted', color: 'var(--warning)', bg: 'rgba(255,174,31,0.12)' },
  };
  const s = map[store_type] || { label: store_type, color: 'var(--muted)', bg: 'rgba(124,143,172,0.12)' };
  return (
    <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: s.bg, color: s.color }}>
      {s.label}
    </span>
  );
}

function statusBadge(status: string) {
  const map: Record<string, { color: string; bg: string }> = {
    active:    { color: 'var(--success)', bg: 'rgba(19,222,185,0.12)' },
    cancelled: { color: 'var(--danger)', bg: 'rgba(250,137,107,0.12)' },
    expired:   { color: 'var(--muted)', bg: 'rgba(124,143,172,0.12)' },
  };
  const s = map[status] || { color: 'var(--muted)', bg: 'rgba(124,143,172,0.12)' };
  return (
    <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: s.bg, color: s.color, textTransform: 'capitalize' }}>
      {status}
    </span>
  );
}

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<UserDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);

  const [showRoleModal, setShowRoleModal] = useState(false);
  const [showGrantModal, setShowGrantModal] = useState(false);
  const [newRole, setNewRole] = useState('');
  const [grantPlanId, setGrantPlanId] = useState('');
  const [plans, setPlans] = useState<{ id: string; name: string }[]>([]);
  const [saving, setSaving] = useState(false);
  const [txHistory, setTxHistory] = useState<Transaction[]>([]);

  const fetchUser = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await apiFetch(`/admin/users/${id}`) as UserDetailResponse;
      setData(resp);
      setNewRole(resp.user.role);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load user');
    } finally {
      setLoading(false);
    }
  }, [id]);

  const fetchPlans = useCallback(async () => {
    try {
      const p = await apiFetch('/admin/plans') as { id: string; name: string }[];
      setPlans(p);
    } catch (_) { /* ignore */ }
  }, []);

  const fetchTxHistory = useCallback(async (email: string) => {
    try {
      const params = new URLSearchParams({ search: email, limit: '50' });
      const data = await apiFetch(`/admin/transactions?${params.toString()}`) as { transactions: Transaction[]; total: number };
      setTxHistory(data.transactions);
    } catch (_) { /* ignore */ }
  }, []);

  useEffect(() => {
    fetchUser();
    fetchPlans();
  }, [fetchUser, fetchPlans]);

  useEffect(() => {
    if (data?.user?.email) {
      fetchTxHistory(data.user.email);
    }
  }, [data?.user?.email, fetchTxHistory]);

  const user = data?.user;

  async function toggleActive() {
    if (!user) return;
    setSaving(true);
    try {
      await apiFetch(`/admin/users/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ is_active: !user.is_active }),
      });
      setToast({ message: `User ${user.is_active ? 'deactivated' : 'activated'} successfully.`, type: 'success' });
      fetchUser();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to update user', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  async function changeRole() {
    setSaving(true);
    try {
      await apiFetch(`/admin/users/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ role: newRole }),
      });
      setToast({ message: 'Role updated successfully.', type: 'success' });
      setShowRoleModal(false);
      fetchUser();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to change role', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  async function grantSubscription() {
    setSaving(true);
    try {
      await apiFetch('/admin/subscriptions/grant', {
        method: 'POST',
        body: JSON.stringify({ user_id: id, plan_id: grantPlanId }),
      });
      setToast({ message: 'Subscription granted successfully.', type: 'success' });
      setShowGrantModal(false);
      fetchUser();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to grant subscription', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '80px 0', color: 'var(--muted)', gap: 10 }}>
        Loading user...
      </div>
    );
  }

  if (error || !user) {
    return <div className="alert-error">{error || 'User not found'}</div>;
  }

  const sub = data?.subscription;
  const recentUsage = data?.recent_usage || [];
  const usageToday = data?.usage_today || 0;

  return (
    <div style={{ maxWidth: 860 }}>

      {/* Back button */}
      <button
        onClick={() => navigate('/users')}
        style={{
          display: 'flex', alignItems: 'center', gap: 6, background: 'transparent',
          border: 'none', color: 'var(--muted)', fontSize: 13, cursor: 'pointer',
          padding: 0, marginBottom: 20, fontFamily: 'inherit', transition: 'color 0.15s',
        }}
        onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--primary)'; }}
        onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--muted)'; }}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <polyline points="15 18 9 12 15 6" />
        </svg>
        Back to Users
      </button>

      {/* Page title */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 24 }}>
        <div>
          <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>
            {user.name || user.email}
          </h1>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>{user.email}</p>
        </div>
        <span className={`badge ${user.is_active ? 'badge-success' : 'badge-muted'}`} style={{ marginTop: 4 }}>
          {user.is_active ? 'Active' : 'Inactive'}
        </span>
      </div>

      {/* Info cards */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginBottom: 20 }}>
        <InfoCard title="User Info">
          <InfoRow label="Email">{user.email}</InfoRow>
          <InfoRow label="Name">{user.name || '—'}</InfoRow>
          <InfoRow label="Role"><span style={{ textTransform: 'capitalize' }}>{user.role}</span></InfoRow>
          <InfoRow label="Joined">{new Date(user.created_at).toLocaleDateString()}</InfoRow>
          <InfoRow label="Usage Today">{usageToday} rewrites</InfoRow>
        </InfoCard>

        <InfoCard title="Subscription">
          {sub ? (
            <>
              <InfoRow label="Plan"><span style={{ textTransform: 'capitalize' }}>{sub.plan?.name || '—'}</span></InfoRow>
              <InfoRow label="Daily Limit">{sub.plan?.daily_limit === -1 ? 'Unlimited' : sub.plan?.daily_limit}</InfoRow>
              <InfoRow label="Status">
                <span className={`badge ${sub.status === 'active' ? 'badge-success' : 'badge-muted'}`}>
                  {sub.status}
                </span>
              </InfoRow>
              {sub.expires_at && (
                <InfoRow label="Expires">
                  {new Date(sub.expires_at).toLocaleDateString()}
                </InfoRow>
              )}
            </>
          ) : (
            <p style={{ color: 'var(--muted)', fontSize: 13, padding: '16px 0' }}>No active subscription</p>
          )}
        </InfoCard>
      </div>

      {/* Actions card */}
      <div style={{ background: 'var(--card)', borderRadius: 7, padding: '18px 22px', marginBottom: 20 }}>
        <h3 style={{ color: 'var(--text)', fontSize: 15, fontWeight: 600, margin: '0 0 14px' }}>Actions</h3>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
          <button
            onClick={toggleActive}
            disabled={saving}
            className={`btn btn-sm ${user.is_active ? 'btn-danger' : 'btn-primary'}`}
            style={{ background: user.is_active ? 'rgba(250,137,107,0.12)' : 'rgba(19,222,185,0.12)', color: user.is_active ? 'var(--danger)' : 'var(--success)', border: `1px solid ${user.is_active ? 'rgba(250,137,107,0.25)' : 'rgba(19,222,185,0.25)'}` }}
          >
            {user.is_active ? 'Deactivate User' : 'Activate User'}
          </button>
          <button
            onClick={() => setShowRoleModal(true)}
            className="btn btn-sm"
            style={{ background: 'rgba(93,135,255,0.1)', color: 'var(--primary)', border: '1px solid rgba(93,135,255,0.25)' }}
          >
            Change Role
          </button>
          <button
            onClick={() => setShowGrantModal(true)}
            className="btn btn-sm"
            style={{ background: 'rgba(73,190,255,0.1)', color: 'var(--secondary)', border: '1px solid rgba(73,190,255,0.25)' }}
          >
            Grant Subscription
          </button>
        </div>
      </div>

      {/* Recent Usage */}
      <div style={{ background: 'var(--card)', borderRadius: 7, overflow: 'hidden' }}>
        <div style={{ padding: '16px 22px', borderBottom: '1px solid var(--border)' }}>
          <h3 style={{ color: 'var(--text)', fontSize: 15, fontWeight: 600, margin: 0 }}>Recent Usage (last 20)</h3>
        </div>

        {recentUsage.length > 0 ? (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid var(--border)' }}>
                  {['Date', 'Tone', 'Input', 'Output', 'Time'].map((h) => (
                    <th key={h} style={{ padding: '12px 22px', textAlign: 'left', color: 'var(--muted)', fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {recentUsage.map((log) => (
                  <tr key={log.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td style={{ padding: '13px 22px', color: 'var(--muted)', fontSize: 13 }}>
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td style={{ padding: '13px 22px', color: 'var(--text)', fontSize: 14, textTransform: 'capitalize' }}>{log.tone}</td>
                    <td style={{ padding: '13px 22px', color: 'var(--muted)', fontSize: 13 }}>{log.input_length} chars</td>
                    <td style={{ padding: '13px 22px', color: 'var(--muted)', fontSize: 13 }}>{log.output_length} chars</td>
                    <td style={{ padding: '13px 22px', color: 'var(--muted)', fontSize: 13 }}>{log.response_time_ms}ms</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p style={{ padding: '32px 22px', color: 'var(--muted)', fontSize: 13, textAlign: 'center', margin: 0 }}>
            No usage logs found.
          </p>
        )}
      </div>

      {/* Transaction History */}
      <div style={{ background: 'var(--card)', borderRadius: 7, overflow: 'hidden', marginTop: 20 }}>
        <div style={{ padding: '16px 22px', borderBottom: '1px solid var(--border)' }}>
          <h3 style={{ color: 'var(--text)', fontSize: 15, fontWeight: 600, margin: 0 }}>
            Transaction History ({txHistory.length})
          </h3>
        </div>

        {txHistory.length > 0 ? (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid var(--border)' }}>
                  {['Plan', 'Amount', 'Store', 'Status', 'Started', 'Expires'].map(h => (
                    <th key={h} style={{ padding: '12px 22px', textAlign: 'left', color: 'var(--muted)', fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', whiteSpace: 'nowrap' }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {txHistory.map(tx => (
                  <tr key={tx.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td style={{ padding: '13px 22px', color: 'var(--text)', fontSize: 14, fontWeight: 500 }}>{tx.plan_name}</td>
                    <td style={{ padding: '13px 22px', color: tx.price_cents === 0 ? 'var(--muted)' : 'var(--success)', fontSize: 14, fontWeight: 600 }}>
                      {tx.price_cents === 0 ? 'Free' : formatCurrency(tx.price_cents)}
                    </td>
                    <td style={{ padding: '13px 22px' }}>{storeBadge(tx.store_type)}</td>
                    <td style={{ padding: '13px 22px' }}>{statusBadge(tx.status)}</td>
                    <td style={{ padding: '13px 22px', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>
                      {new Date(tx.started_at).toLocaleDateString()}
                    </td>
                    <td style={{ padding: '13px 22px', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>
                      {tx.expires_at ? new Date(tx.expires_at).toLocaleDateString() : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p style={{ padding: '32px 22px', color: 'var(--muted)', fontSize: 13, textAlign: 'center', margin: 0 }}>
            No transaction history.
          </p>
        )}
      </div>

      {/* Role Modal */}
      {showRoleModal && (
        <Modal
          title="Change Role"
          onClose={() => setShowRoleModal(false)}
          footer={
            <>
              <button onClick={() => setShowRoleModal(false)} className="btn btn-ghost btn-sm">Cancel</button>
              <button onClick={changeRole} disabled={saving} className="btn btn-primary btn-sm">
                {saving ? 'Saving...' : 'Save'}
              </button>
            </>
          }
        >
          <div>
            <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Role</label>
            <select
              value={newRole}
              onChange={(e) => setNewRole(e.target.value)}
              className="dark-input"
            >
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </div>
        </Modal>
      )}

      {/* Grant Subscription Modal */}
      {showGrantModal && (
        <Modal
          title="Grant Subscription"
          onClose={() => setShowGrantModal(false)}
          footer={
            <>
              <button onClick={() => setShowGrantModal(false)} className="btn btn-ghost btn-sm">Cancel</button>
              <button onClick={grantSubscription} disabled={saving || !grantPlanId} className="btn btn-primary btn-sm">
                {saving ? 'Granting...' : 'Grant'}
              </button>
            </>
          }
        >
          <div>
            <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Plan</label>
            <select
              value={grantPlanId}
              onChange={(e) => setGrantPlanId(e.target.value)}
              className="dark-input"
            >
              <option value="">Select a plan...</option>
              {plans.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
          </div>
        </Modal>
      )}

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
