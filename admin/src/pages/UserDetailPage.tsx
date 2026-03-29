import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch } from '../api';

interface UsageLog {
  id: string;
  createdAt: string;
  type: string;
  tokensUsed?: number;
}

interface Subscription {
  plan: string;
  status: string;
  expiresAt?: string;
}

interface UserDetail {
  id: string;
  email: string;
  name: string;
  role: string;
  active: boolean;
  createdAt: string;
  usageToday: number;
  subscription?: Subscription;
  usageLogs?: UsageLog[];
}

interface Toast {
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
        borderBottom: '1px solid #333f55',
        fontSize: 14,
      }}
    >
      <span style={{ color: '#7c8fac' }}>{label}</span>
      <span style={{ color: '#eaeff4', fontWeight: 500 }}>{children}</span>
    </div>
  );
}

/* ── Info card ──────────────────────────────────────────── */
function InfoCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ background: '#2a3547', borderRadius: 7 }}>
      <div
        style={{
          padding: '16px 22px',
          borderBottom: '1px solid #333f55',
        }}
      >
        <h3 style={{ color: '#eaeff4', fontSize: 15, fontWeight: 600, margin: 0 }}>{title}</h3>
      </div>
      <div style={{ padding: '4px 22px 12px' }}>{children}</div>
    </div>
  );
}

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [user, setUser] = useState<UserDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<Toast | null>(null);

  const [showRoleModal, setShowRoleModal] = useState(false);
  const [showGrantModal, setShowGrantModal] = useState(false);
  const [newRole, setNewRole] = useState('');
  const [grantPlan, setGrantPlan] = useState('');
  const [grantDays, setGrantDays] = useState('30');
  const [saving, setSaving] = useState(false);

  const fetchUser = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiFetch(`/admin/users/${id}`) as UserDetail;
      setUser(data);
      setNewRole(data.role);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load user');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  async function toggleActive() {
    if (!user) return;
    setSaving(true);
    try {
      await apiFetch(`/admin/users/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ active: !user.active }),
      });
      setToast({ message: `User ${user.active ? 'deactivated' : 'activated'} successfully.`, type: 'success' });
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
        body: JSON.stringify({ userId: id, plan: grantPlan, days: Number(grantDays) }),
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
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '80px 0', color: '#7c8fac', gap: 10 }}>
        Loading user...
      </div>
    );
  }

  if (error || !user) {
    return <div className="alert-error">{error || 'User not found'}</div>;
  }

  return (
    <div style={{ maxWidth: 860 }}>

      {/* Back button */}
      <button
        onClick={() => navigate('/users')}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          background: 'transparent',
          border: 'none',
          color: '#7c8fac',
          fontSize: 13,
          cursor: 'pointer',
          padding: 0,
          marginBottom: 20,
          fontFamily: 'inherit',
          transition: 'color 0.15s',
        }}
        onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#5d87ff'; }}
        onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#7c8fac'; }}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <polyline points="15 18 9 12 15 6" />
        </svg>
        Back to Users
      </button>

      {/* Page title */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 24 }}>
        <div>
          <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>
            {user.name || user.email}
          </h1>
          <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>{user.email}</p>
        </div>
        <span className={`badge ${user.active ? 'badge-success' : 'badge-muted'}`} style={{ marginTop: 4 }}>
          {user.active ? 'Active' : 'Inactive'}
        </span>
      </div>

      {/* Info cards */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginBottom: 20 }}>
        <InfoCard title="User Info">
          <InfoRow label="Email">{user.email}</InfoRow>
          <InfoRow label="Name">{user.name || '—'}</InfoRow>
          <InfoRow label="Role"><span style={{ textTransform: 'capitalize' }}>{user.role}</span></InfoRow>
          <InfoRow label="Joined">{new Date(user.createdAt).toLocaleDateString()}</InfoRow>
          <InfoRow label="Usage Today">{user.usageToday} rewrites</InfoRow>
        </InfoCard>

        <InfoCard title="Subscription">
          {user.subscription ? (
            <>
              <InfoRow label="Plan"><span style={{ textTransform: 'capitalize' }}>{user.subscription.plan}</span></InfoRow>
              <InfoRow label="Status">
                <span className={`badge ${user.subscription.status === 'active' ? 'badge-success' : 'badge-muted'}`}>
                  {user.subscription.status}
                </span>
              </InfoRow>
              {user.subscription.expiresAt && (
                <InfoRow label="Expires">
                  {new Date(user.subscription.expiresAt).toLocaleDateString()}
                </InfoRow>
              )}
            </>
          ) : (
            <p style={{ color: '#7c8fac', fontSize: 13, padding: '16px 0' }}>No active subscription</p>
          )}
        </InfoCard>
      </div>

      {/* Actions card */}
      <div style={{ background: '#2a3547', borderRadius: 7, padding: '18px 22px', marginBottom: 20 }}>
        <h3 style={{ color: '#eaeff4', fontSize: 15, fontWeight: 600, margin: '0 0 14px' }}>Actions</h3>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
          <button
            onClick={toggleActive}
            disabled={saving}
            className={`btn btn-sm ${user.active ? 'btn-danger' : 'btn-primary'}`}
            style={{ background: user.active ? 'rgba(250,137,107,0.12)' : 'rgba(19,222,185,0.12)', color: user.active ? '#fa896b' : '#13deb9', border: `1px solid ${user.active ? 'rgba(250,137,107,0.25)' : 'rgba(19,222,185,0.25)'}` }}
          >
            {user.active ? 'Deactivate User' : 'Activate User'}
          </button>
          <button
            onClick={() => setShowRoleModal(true)}
            className="btn btn-sm"
            style={{ background: 'rgba(93,135,255,0.1)', color: '#5d87ff', border: '1px solid rgba(93,135,255,0.25)' }}
          >
            Change Role
          </button>
          <button
            onClick={() => setShowGrantModal(true)}
            className="btn btn-sm"
            style={{ background: 'rgba(73,190,255,0.1)', color: '#49beff', border: '1px solid rgba(73,190,255,0.25)' }}
          >
            Grant Subscription
          </button>
        </div>
      </div>

      {/* Recent Usage */}
      <div style={{ background: '#2a3547', borderRadius: 7, overflow: 'hidden' }}>
        <div style={{ padding: '16px 22px', borderBottom: '1px solid #333f55' }}>
          <h3 style={{ color: '#eaeff4', fontSize: 15, fontWeight: 600, margin: 0 }}>Recent Usage (last 20)</h3>
        </div>

        {user.usageLogs && user.usageLogs.length > 0 ? (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #333f55' }}>
                  {['Date', 'Type', 'Tokens'].map((h) => (
                    <th
                      key={h}
                      style={{
                        padding: '12px 22px',
                        textAlign: 'left',
                        color: '#7c8fac',
                        fontSize: 12,
                        fontWeight: 700,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                      }}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {user.usageLogs.map((log) => (
                  <tr key={log.id} style={{ borderBottom: '1px solid #333f55' }}>
                    <td style={{ padding: '13px 22px', color: '#7c8fac', fontSize: 13 }}>
                      {new Date(log.createdAt).toLocaleString()}
                    </td>
                    <td style={{ padding: '13px 22px', color: '#eaeff4', fontSize: 14 }}>{log.type}</td>
                    <td style={{ padding: '13px 22px', color: '#7c8fac', fontSize: 13 }}>{log.tokensUsed ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p style={{ padding: '32px 22px', color: '#7c8fac', fontSize: 13, textAlign: 'center', margin: 0 }}>
            No usage logs found.
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
            <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Role</label>
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
              <button onClick={grantSubscription} disabled={saving || !grantPlan} className="btn btn-primary btn-sm">
                {saving ? 'Granting...' : 'Grant'}
              </button>
            </>
          }
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Plan Name</label>
              <input
                type="text"
                value={grantPlan}
                onChange={(e) => setGrantPlan(e.target.value)}
                placeholder="e.g. pro"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Duration (days)</label>
              <input
                type="number"
                value={grantDays}
                onChange={(e) => setGrantDays(e.target.value)}
                min="1"
                className="dark-input"
              />
            </div>
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
