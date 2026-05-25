import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch, SEARCH_DEBOUNCE_MS } from '../api';
import { formatCurrency } from '../lib/format';

interface Plan {
  id: string;
  name: string;
  daily_limit: number;
  price_cents: number;
  billing_period: string;
  is_active: boolean;
  currency: string | null;
  trial_days: number;
  stripe_price_id: string | null;
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
  currency: 'USD',
  trial_days: '30',
  stripe_price_id: '',
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

  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [sortBy, setSortBy] = useState<string>('created_at');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');

  const fetchPlans = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(page),
        limit: String(pageSize),
        status: statusFilter,
        sort_by: sortBy,
        sort_order: sortOrder,
      });
      if (search) params.set('search', search);
      const data = await apiFetch(`/admin/plans?${params}`) as { rows: Plan[]; total: number };
      setPlans(data.rows ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load plans');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, search, sortBy, sortOrder]);

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  // Debounce search input → search state.
  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [searchInput]);

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
      currency: plan.currency || 'USD',
      trial_days: String(plan.trial_days ?? 30),
      stripe_price_id: plan.stripe_price_id || '',
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
      currency: form.currency,
      trial_days: Number(form.trial_days),
      stripe_price_id: form.stripe_price_id || null,
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
      sortKey: 'name',
      render: (row: Plan) => <span style={{ color: 'var(--text)', fontWeight: 600 }}>{row.name}</span>,
    },
    {
      header: 'Daily Limit',
      key: 'daily_limit',
      render: (row: Plan) => <span style={{ color: 'var(--muted)' }}>{row.daily_limit === -1 ? 'Unlimited' : row.daily_limit}</span>,
    },
    {
      header: 'Price',
      key: 'price_cents',
      sortKey: 'price',
      render: (row: Plan) => (
        <span style={{ color: 'var(--text)', fontWeight: 600 }}>{formatCurrency(row.price_cents, row.currency)}</span>
      ),
    },
    {
      header: 'Billing Period',
      key: 'billing_period',
      sortKey: 'billing_period',
      render: (row: Plan) => <span style={{ color: 'var(--muted)', textTransform: 'capitalize' as const }}>{row.billing_period}</span>,
    },
    {
      header: 'Trial',
      key: 'trial_days',
      sortKey: 'trial_days',
      render: (row: Plan) => <span style={{ color: 'var(--muted)' }}>{row.trial_days || 0}d</span>,
    },
    {
      header: 'Stripe Price',
      key: 'stripe_price_id',
      render: (row: Plan) => (
        <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11 }}>
          {row.stripe_price_id ? `${row.stripe_price_id.slice(0, 14)}…` : '—'}
        </span>
      ),
    },
    {
      header: 'Active',
      key: 'is_active',
      sortKey: 'is_active',
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
            style={{ background: 'rgba(93,135,255,0.1)', color: 'var(--primary)', border: '1px solid rgba(93,135,255,0.2)' }}
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); deletePlan(row); }}
            className="btn btn-sm"
            style={{ background: 'rgba(250,137,107,0.1)', color: 'var(--danger)', border: '1px solid rgba(250,137,107,0.2)' }}
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
          <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Plans</h1>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>Manage subscription plans</p>
        </div>
        <button onClick={openCreate} className="btn btn-primary">
          + Create Plan
        </button>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 16 }}>{error}</div>}

      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Search by name, currency, or billing period..."
          style={{
            flex: '1 1 280px', maxWidth: 360,
            padding: '8px 14px 8px 36px',
            borderRadius: 7, border: '1px solid var(--border)', background: 'var(--bg)',
            color: 'var(--text)', fontSize: 13, fontFamily: 'inherit', outline: 'none',
            backgroundImage: "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%237c8fac' stroke-width='2'><circle cx='11' cy='11' r='8'/><path d='M21 21l-4.35-4.35'/></svg>\")",
            backgroundRepeat: 'no-repeat', backgroundPosition: '12px center',
          }}
        />
        <div style={{ display: 'flex', gap: 4, padding: 4, background: 'var(--bg)', border: '1px solid var(--border)', borderRadius: 7 }}>
          {(['all','active','inactive'] as const).map((s) => (
            <button
              key={s}
              onClick={() => { setStatusFilter(s); setPage(1); }}
              style={{
                padding: '6px 14px', borderRadius: 5, fontSize: 12, fontWeight: 600,
                border: 'none', cursor: 'pointer', fontFamily: 'inherit',
                background: statusFilter === s ? 'rgba(93,135,255,0.15)' : 'transparent',
                color: statusFilter === s ? 'var(--primary)' : 'var(--muted)',
                textTransform: 'capitalize',
              }}
            >
              {s}
            </button>
          ))}
        </div>
        <span style={{ marginLeft: 'auto', color: 'var(--muted)', fontSize: 12 }}>
          {total > 0 ? `${total} ${total === 1 ? 'plan' : 'plans'}` : ''}
        </span>
      </div>

      <DataTable<Plan>
        columns={columns}
        rows={plans}
        loading={loading}
        page={page}
        totalPages={Math.max(1, Math.ceil(total / pageSize))}
        onPageChange={setPage}
        total={total}
        pageSize={pageSize}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        sortBy={sortBy}
        sortOrder={sortOrder}
        onSortChange={(by, order) => { setSortBy(by); setSortOrder(order); setPage(1); }}
        emptyMessage={search || statusFilter !== 'all' ? 'No matches.' : 'No plans found. Create one to get started.'}
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
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. Pro"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Daily Limit (-1 = unlimited)</label>
              <input
                type="number"
                value={form.daily_limit}
                onChange={(e) => setForm({ ...form, daily_limit: e.target.value })}
                placeholder="e.g. 100"
                min="-1"
                className="dark-input"
              />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Currency</label>
                <select
                  value={form.currency}
                  onChange={(e) => setForm({ ...form, currency: e.target.value })}
                  className="dark-input"
                >
                  <option value="USD">USD ($)</option>
                  <option value="VND">VND (₫)</option>
                </select>
              </div>
              <div>
                <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                  Price ({form.currency === 'USD' ? 'cents' : 'whole VND'})
                </label>
                <input
                  type="number"
                  value={form.price_cents}
                  onChange={(e) => setForm({ ...form, price_cents: e.target.value })}
                  placeholder={form.currency === 'USD' ? '499 for $4.99' : '99000 for ₫99,000'}
                  min="0"
                  className="dark-input"
                />
              </div>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Billing Period</label>
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
              <div>
                <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Trial Days</label>
                <input
                  type="number"
                  value={form.trial_days}
                  onChange={(e) => setForm({ ...form, trial_days: e.target.value })}
                  placeholder="0 = no trial"
                  min="0"
                  className="dark-input"
                />
              </div>
            </div>
            <div>
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Stripe Price ID</label>
              <input
                type="text"
                value={form.stripe_price_id}
                onChange={(e) => setForm({ ...form, stripe_price_id: e.target.value })}
                placeholder="price_1XXXX (leave blank to auto-create)"
                className="dark-input"
                style={{ fontFamily: 'monospace', fontSize: 12 }}
              />
              <p style={{ color: 'var(--muted)', fontSize: 11, margin: '6px 0 0' }}>
                Run <code>scripts/sync-stripe-prices.js</code> after editing price to mint a fresh Stripe Price.
              </p>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <input
                type="checkbox"
                id="is_active"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                style={{ width: 16, height: 16, accentColor: 'var(--primary)', cursor: 'pointer' }}
              />
              <label htmlFor="is_active" style={{ color: 'var(--text)', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>Active</label>
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
