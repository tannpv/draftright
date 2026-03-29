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

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [user, setUser] = useState<UserDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<Toast | null>(null);

  // Modal states
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
      <div className="flex items-center justify-center py-20 text-gray-400">Loading...</div>
    );
  }

  if (error || !user) {
    return (
      <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3">
        {error || 'User not found'}
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      {/* Back */}
      <button
        onClick={() => navigate('/users')}
        className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 mb-6"
      >
        ← Back to Users
      </button>

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{user.name || user.email}</h1>
          <p className="text-gray-500 text-sm mt-1">{user.email}</p>
        </div>
        <span
          className={`inline-flex px-3 py-1 rounded-full text-sm font-medium ${
            user.active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
          }`}
        >
          {user.active ? 'Active' : 'Inactive'}
        </span>
      </div>

      {/* Info cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        {/* User Info */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
          <h3 className="font-semibold text-gray-900 mb-4">User Info</h3>
          <dl className="space-y-3 text-sm">
            <div className="flex justify-between">
              <dt className="text-gray-500">Email</dt>
              <dd className="text-gray-900 font-medium">{user.email}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Name</dt>
              <dd className="text-gray-900 font-medium">{user.name || '—'}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Role</dt>
              <dd className="text-gray-900 font-medium capitalize">{user.role}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Joined</dt>
              <dd className="text-gray-900 font-medium">
                {new Date(user.createdAt).toLocaleDateString()}
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Usage Today</dt>
              <dd className="text-gray-900 font-medium">{user.usageToday} rewrites</dd>
            </div>
          </dl>
        </div>

        {/* Subscription */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
          <h3 className="font-semibold text-gray-900 mb-4">Subscription</h3>
          {user.subscription ? (
            <dl className="space-y-3 text-sm">
              <div className="flex justify-between">
                <dt className="text-gray-500">Plan</dt>
                <dd className="text-gray-900 font-medium capitalize">{user.subscription.plan}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Status</dt>
                <dd>
                  <span
                    className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${
                      user.subscription.status === 'active'
                        ? 'bg-green-100 text-green-700'
                        : 'bg-gray-100 text-gray-600'
                    }`}
                  >
                    {user.subscription.status}
                  </span>
                </dd>
              </div>
              {user.subscription.expiresAt && (
                <div className="flex justify-between">
                  <dt className="text-gray-500">Expires</dt>
                  <dd className="text-gray-900 font-medium">
                    {new Date(user.subscription.expiresAt).toLocaleDateString()}
                  </dd>
                </div>
              )}
            </dl>
          ) : (
            <p className="text-sm text-gray-400">No active subscription</p>
          )}
        </div>
      </div>

      {/* Actions */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-8">
        <h3 className="font-semibold text-gray-900 mb-4">Actions</h3>
        <div className="flex flex-wrap gap-3">
          <button
            onClick={toggleActive}
            disabled={saving}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-60 ${
              user.active
                ? 'bg-red-50 text-red-700 hover:bg-red-100 border border-red-200'
                : 'bg-green-50 text-green-700 hover:bg-green-100 border border-green-200'
            }`}
          >
            {user.active ? 'Deactivate User' : 'Activate User'}
          </button>
          <button
            onClick={() => setShowRoleModal(true)}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-blue-50 text-blue-700 hover:bg-blue-100 border border-blue-200 transition-colors"
          >
            Change Role
          </button>
          <button
            onClick={() => setShowGrantModal(true)}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-purple-50 text-purple-700 hover:bg-purple-100 border border-purple-200 transition-colors"
          >
            Grant Subscription
          </button>
        </div>
      </div>

      {/* Recent Usage */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="font-semibold text-gray-900">Recent Usage (last 20)</h3>
        </div>
        {user.usageLogs && user.usageLogs.length > 0 ? (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Date</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Type</th>
                <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Tokens</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {user.usageLogs.map((log) => (
                <tr key={log.id}>
                  <td className="px-6 py-3 text-sm text-gray-700">
                    {new Date(log.createdAt).toLocaleString()}
                  </td>
                  <td className="px-6 py-3 text-sm text-gray-700">{log.type}</td>
                  <td className="px-6 py-3 text-sm text-gray-700">{log.tokensUsed ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="px-6 py-8 text-sm text-gray-400 text-center">No usage logs found.</p>
        )}
      </div>

      {/* Role Modal */}
      {showRoleModal && (
        <Modal
          title="Change Role"
          onClose={() => setShowRoleModal(false)}
          footer={
            <>
              <button
                onClick={() => setShowRoleModal(false)}
                className="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={changeRole}
                disabled={saving}
                className="px-4 py-2 text-sm rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {saving ? 'Saving...' : 'Save'}
              </button>
            </>
          }
        >
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Role</label>
            <select
              value={newRole}
              onChange={(e) => setNewRole(e.target.value)}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
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
              <button
                onClick={() => setShowGrantModal(false)}
                className="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={grantSubscription}
                disabled={saving || !grantPlan}
                className="px-4 py-2 text-sm rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {saving ? 'Granting...' : 'Grant'}
              </button>
            </>
          }
        >
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Plan Name</label>
              <input
                type="text"
                value={grantPlan}
                onChange={(e) => setGrantPlan(e.target.value)}
                placeholder="e.g. pro"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Duration (days)</label>
              <input
                type="number"
                value={grantDays}
                onChange={(e) => setGrantDays(e.target.value)}
                min="1"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
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
