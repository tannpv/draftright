import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch } from '../api';

interface Provider {
  id: string;
  name: string;
  type: string;
  endpoint: string;
  model: string;
  isDefault: boolean;
  active: boolean;
  [key: string]: unknown;
}

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

const emptyForm = {
  name: '',
  type: 'openai',
  endpoint: '',
  model: '',
  apiKey: '',
  isDefault: false,
  active: true,
};

export default function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);

  const [showModal, setShowModal] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);

  const fetchProviders = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiFetch('/admin/ai-providers') as Provider[];
      setProviders(Array.isArray(data) ? data : []);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load providers');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchProviders();
  }, [fetchProviders]);

  function openCreate() {
    setEditingProvider(null);
    setForm(emptyForm);
    setShowModal(true);
  }

  function openEdit(provider: Provider) {
    setEditingProvider(provider);
    setForm({
      name: provider.name,
      type: provider.type,
      endpoint: provider.endpoint,
      model: provider.model,
      apiKey: '',
      isDefault: provider.isDefault,
      active: provider.active,
    });
    setShowModal(true);
  }

  async function saveProvider() {
    setSaving(true);
    const payload: Record<string, unknown> = {
      name: form.name,
      type: form.type,
      endpoint: form.endpoint,
      model: form.model,
      isDefault: form.isDefault,
      active: form.active,
    };
    if (form.apiKey) payload.apiKey = form.apiKey;

    try {
      if (editingProvider) {
        await apiFetch(`/admin/ai-providers/${editingProvider.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        setToast({ message: 'Provider updated successfully.', type: 'success' });
      } else {
        await apiFetch('/admin/ai-providers', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        setToast({ message: 'Provider created successfully.', type: 'success' });
      }
      setShowModal(false);
      fetchProviders();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to save provider', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  async function deleteProvider(provider: Provider) {
    if (!confirm(`Delete provider "${provider.name}"?`)) return;
    try {
      await apiFetch(`/admin/ai-providers/${provider.id}`, { method: 'DELETE' });
      setToast({ message: 'Provider deleted.', type: 'success' });
      fetchProviders();
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to delete provider', type: 'error' });
    }
  }

  async function testConnection(provider: Provider) {
    setTestingId(provider.id);
    try {
      await apiFetch(`/admin/ai-providers/${provider.id}/test`, { method: 'POST' });
      setToast({ message: `Connection to "${provider.name}" successful!`, type: 'success' });
    } catch (err) {
      setToast({ message: `Connection failed: ${err instanceof Error ? err.message : 'Unknown error'}`, type: 'error' });
    } finally {
      setTestingId(null);
    }
  }

  const columns = [
    {
      header: 'Name',
      key: 'name',
      render: (row: Provider) => <span style={{ color: '#eaeff4', fontWeight: 600 }}>{row.name}</span>,
    },
    {
      header: 'Type',
      key: 'type',
      render: (row: Provider) => <span style={{ color: '#7c8fac', textTransform: 'capitalize' }}>{row.type}</span>,
    },
    {
      header: 'Endpoint',
      key: 'endpoint',
      render: (row: Provider) => (
        <span style={{ color: '#7c8fac', fontSize: 12, fontFamily: 'monospace', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', display: 'inline-block' }}>
          {row.endpoint}
        </span>
      ),
    },
    {
      header: 'Model',
      key: 'model',
      render: (row: Provider) => (
        <span style={{ color: '#49beff', fontSize: 12, fontFamily: 'monospace' }}>{row.model}</span>
      ),
    },
    {
      header: 'Default',
      key: 'isDefault',
      render: (row: Provider) =>
        row.isDefault
          ? <span className="badge badge-primary">Default</span>
          : <span style={{ color: '#333f55' }}>—</span>,
    },
    {
      header: 'Active',
      key: 'active',
      render: (row: Provider) => (
        <span className={`badge ${row.active ? 'badge-success' : 'badge-muted'}`}>
          {row.active ? 'Yes' : 'No'}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: Provider) => (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button
            onClick={(e) => { e.stopPropagation(); testConnection(row); }}
            disabled={testingId === row.id}
            className="btn btn-sm"
            style={{ background: 'rgba(124,143,172,0.1)', color: '#7c8fac', border: '1px solid #333f55', opacity: testingId === row.id ? 0.5 : 1 }}
          >
            {testingId === row.id ? 'Testing...' : 'Test'}
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); openEdit(row); }}
            className="btn btn-sm"
            style={{ background: 'rgba(93,135,255,0.1)', color: '#5d87ff', border: '1px solid rgba(93,135,255,0.2)' }}
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); deleteProvider(row); }}
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
          <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>AI Providers</h1>
          <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>Manage AI provider configurations</p>
        </div>
        <button onClick={openCreate} className="btn btn-primary">
          + Add Provider
        </button>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 16 }}>{error}</div>}

      <DataTable<Provider>
        columns={columns}
        rows={providers}
        loading={loading}
        emptyMessage="No providers configured. Add one to get started."
      />

      {showModal && (
        <Modal
          title={editingProvider ? 'Edit Provider' : 'Add Provider'}
          onClose={() => setShowModal(false)}
          footer={
            <>
              <button onClick={() => setShowModal(false)} className="btn btn-ghost btn-sm">Cancel</button>
              <button
                onClick={saveProvider}
                disabled={saving || !form.name || !form.model}
                className="btn btn-primary btn-sm"
              >
                {saving ? 'Saving...' : editingProvider ? 'Update' : 'Create'}
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
                placeholder="e.g. OpenAI GPT-4"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Type</label>
              <select
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value })}
                className="dark-input"
              >
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="ollama">Ollama</option>
                <option value="custom">Custom</option>
              </select>
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Endpoint</label>
              <input
                type="text"
                value={form.endpoint}
                onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
                placeholder="e.g. https://api.openai.com/v1"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Model</label>
              <input
                type="text"
                value={form.model}
                onChange={(e) => setForm({ ...form, model: e.target.value })}
                placeholder="e.g. gpt-4o"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                API Key{editingProvider && <span style={{ color: '#7c8fac', fontWeight: 400 }}> (leave blank to keep existing)</span>}
              </label>
              <input
                type="password"
                value={form.apiKey}
                onChange={(e) => setForm({ ...form, apiKey: e.target.value })}
                placeholder="sk-..."
                className="dark-input"
              />
            </div>
            <div style={{ display: 'flex', gap: 24 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  id="isDefault"
                  checked={form.isDefault}
                  onChange={(e) => setForm({ ...form, isDefault: e.target.checked })}
                  style={{ width: 16, height: 16, accentColor: '#5d87ff', cursor: 'pointer' }}
                />
                <label htmlFor="isDefault" style={{ color: '#eaeff4', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>Default</label>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  id="providerActive"
                  checked={form.active}
                  onChange={(e) => setForm({ ...form, active: e.target.checked })}
                  style={{ width: 16, height: 16, accentColor: '#5d87ff', cursor: 'pointer' }}
                />
                <label htmlFor="providerActive" style={{ color: '#eaeff4', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>Active</label>
              </div>
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
