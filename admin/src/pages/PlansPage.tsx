import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch } from '../api';

interface Plan {
  id: string;
  name: string;
  dailyLimit: number;
  price: number;
  billingPeriod: string;
  active: boolean;
  [key: string]: unknown;
}

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

const emptyForm = {
  name: '',
  dailyLimit: '',
  price: '',
  billingPeriod: 'monthly',
  active: true,
};

export default function PlansPage() {
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);

  const [showModal, setShowModal] = useState(false);
  const [editingPlan, setEditingPlan] = useState<Plan | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);

  const fetchPlans = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiFetch('/admin/plans') as Plan[];
      setPlans(Array.isArray(data) ? data : []);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load plans');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  function openCreate() {
    setEditingPlan(null);
    setForm(emptyForm);
    setShowModal(true);
  }

  function openEdit(plan: Plan) {
    setEditingPlan(plan);
    setForm({
      name: plan.name,
      dailyLimit: String(plan.dailyLimit),
      price: String(plan.price),
      billingPeriod: plan.billingPeriod,
      active: plan.active,
    });
    setShowModal(true);
  }

  async function savePlan() {
    setSaving(true);
    const payload = {
      name: form.name,
      dailyLimit: Number(form.dailyLimit),
      price: Number(form.price),
      billingPeriod: form.billingPeriod,
      active: form.active,
    };
    try {
      if (editingPlan) {
        await apiFetch(`/admin/plans/${editingPlan.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        setToast({ message: 'Plan updated successfully.', type: 'success' });
      } else {
        await apiFetch('/admin/plans', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        setToast({ message: 'Plan created successfully.', type: 'success' });
      }
      setShowModal(false);
      fetchPlans();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to save plan', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  async function deletePlan(plan: Plan) {
    if (!confirm(`Delete plan "${plan.name}"?`)) return;
    try {
      await apiFetch(`/admin/plans/${plan.id}`, { method: 'DELETE' });
      setToast({ message: 'Plan deleted.', type: 'success' });
      fetchPlans();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to delete plan', type: 'error' });
    }
  }

  const columns = [
    { header: 'Name', key: 'name' },
    { header: 'Daily Limit', key: 'dailyLimit' },
    {
      header: 'Price',
      key: 'price',
      render: (row: Plan) => `$${Number(row.price).toFixed(2)}`,
    },
    { header: 'Billing Period', key: 'billingPeriod' },
    {
      header: 'Active',
      key: 'active',
      render: (row: Plan) => (
        <span
          className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${
            row.active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'
          }`}
        >
          {row.active ? 'Yes' : 'No'}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: Plan) => (
        <div className="flex gap-2">
          <button
            onClick={(e) => { e.stopPropagation(); openEdit(row); }}
            className="text-xs px-3 py-1 rounded border border-blue-300 text-blue-600 hover:bg-blue-50"
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); deletePlan(row); }}
            className="text-xs px-3 py-1 rounded border border-red-300 text-red-600 hover:bg-red-50"
          >
            Delete
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Plans</h1>
          <p className="text-gray-500 text-sm mt-1">Manage subscription plans</p>
        </div>
        <button
          onClick={openCreate}
          className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors"
        >
          + Create Plan
        </button>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      <DataTable<Plan>
        columns={columns}
        rows={plans}
        loading={loading}
        emptyMessage="No plans found. Create one to get started."
      />

      {showModal && (
        <Modal
          title={editingPlan ? 'Edit Plan' : 'Create Plan'}
          onClose={() => setShowModal(false)}
          footer={
            <>
              <button
                onClick={() => setShowModal(false)}
                className="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={savePlan}
                disabled={saving || !form.name}
                className="px-4 py-2 text-sm rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {saving ? 'Saving...' : editingPlan ? 'Update' : 'Create'}
              </button>
            </>
          }
        >
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. Pro"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Daily Limit</label>
              <input
                type="number"
                value={form.dailyLimit}
                onChange={(e) => setForm({ ...form, dailyLimit: e.target.value })}
                placeholder="e.g. 100"
                min="0"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Price ($)</label>
              <input
                type="number"
                value={form.price}
                onChange={(e) => setForm({ ...form, price: e.target.value })}
                placeholder="e.g. 9.99"
                min="0"
                step="0.01"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Billing Period</label>
              <select
                value={form.billingPeriod}
                onChange={(e) => setForm({ ...form, billingPeriod: e.target.value })}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="monthly">Monthly</option>
                <option value="annual">Annual</option>
                <option value="lifetime">Lifetime</option>
              </select>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="active"
                checked={form.active}
                onChange={(e) => setForm({ ...form, active: e.target.checked })}
                className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <label htmlFor="active" className="text-sm font-medium text-gray-700">Active</label>
            </div>
          </div>
        </Modal>
      )}

      {toast && (
        <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />
      )}
    </div>
  );
}
