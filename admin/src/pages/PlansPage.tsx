import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch } from '../api';

interface Plan {
  id: string;
  name: string;
  daily_limit: number;
  price_cents: number;
  billing_period: string;
  is_active: boolean;
  [key: string]: unknown;
}

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

const emptyForm = {
  name: '',
  daily_limit: '',
  price_cents: '',
  billing_period: 'monthly',
  is_active: true,
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
      daily_limit: String(plan.daily_limit),
      price_cents: String(plan.price_cents),
      billing_period: plan.billing_period,
      is_active: plan.is_active,
    });
    setShowModal(true);
  }

  async function savePlan() {
    setSaving(true);
    const payload = {
      name: form.name,
      daily_limit: Number(form.daily_limit),
      price_cents: Number(form.price_cents),
      billing_period: form.billing_period,
      is_active: form.is_active,
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
    {
      header: 'Name',
      key: 'name',
      render: (row: Plan) => <span style={{ color: '#eaeff4', fontWeight: 600 }}>{row.name}</span>,
    },
    {
      header: 'Daily Limit',
      key: 'daily_limit',
      render: (row: Plan) => <span style={{ color: '#7c8fac' }}>{row.daily_limit === -1 ? 'Unlimited' : row.daily_limit}</span>,
    },
    {
      header: 'Price',
      key: 'price_cents',
      render: (row: Plan) => (
        <span style={{ color: '#eaeff4', fontWeight: 600 }}>${(row.price_cents / 100).toFixed(2)}</span>
      ),
    },
    {
      header: 'Billing Period',
      key: 'billing_period',
      render: (row: Plan) => <span style={{ color: '#7c8fac', textTransform: 'capitalize' as const }}>{row.billing_period}</span>,
    },
    {
      header: 'Active',
      key: 'is_active',
      render: (row: Plan) => (
        <span className={`badge ${row.is_active ? 'badge-success' : 'badge-muted'}`}>
          {row.is_active ? 'Yes' : 'No'}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: Plan) => (
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            onClick={(e) => { e.stopPropagation(); openEdit(row); }}
            className="btn btn-sm"
            style={{ background: 'rgba(93,135,255,0.1)', color: '#5d87ff', border: '1px solid rgba(93,135,255,0.2)' }}
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); deletePlan(row); }}
            className="btn btn-sm"
            style={{ background: 'rgba(250,137,107,0.1)', color: '#fa896b', border: '1px solid rgba(250,137,107,0.2)' }}
          >
            Delete
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 24 }}>
        <div>
          <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Plans</h1>
          <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>Manage subscription plans</p>
        </div>
        <button onClick={openCreate} className="btn btn-primary">
          + Create Plan
        </button>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 16 }}>{error}</div>}

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
              <button onClick={() => setShowModal(false)} className="btn btn-ghost btn-sm">Cancel</button>
              <button
                onClick={savePlan}
                disabled={saving || !form.name}
                className="btn btn-primary btn-sm"
              >
                {saving ? 'Saving...' : editingPlan ? 'Update' : 'Create'}
              </button>
            </>
          }
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. Pro"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Daily Limit (-1 = unlimited)</label>
              <input
                type="number"
                value={form.daily_limit}
                onChange={(e) => setForm({ ...form, daily_limit: e.target.value })}
                placeholder="e.g. 100"
                min="-1"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Price (cents)</label>
              <input
                type="number"
                value={form.price_cents}
                onChange={(e) => setForm({ ...form, price_cents: e.target.value })}
                placeholder="e.g. 999 for $9.99"
                min="0"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Billing Period</label>
              <select
                value={form.billing_period}
                onChange={(e) => setForm({ ...form, billing_period: e.target.value })}
                className="dark-input"
              >
                <option value="none">None (Free)</option>
                <option value="monthly">Monthly</option>
                <option value="yearly">Yearly</option>
              </select>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <input
                type="checkbox"
                id="is_active"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                style={{ width: 16, height: 16, accentColor: '#5d87ff', cursor: 'pointer' }}
              />
              <label htmlFor="is_active" style={{ color: '#eaeff4', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>Active</label>
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
