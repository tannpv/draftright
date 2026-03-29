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
    { header: 'Name', key: 'name' },
    { header: 'Type', key: 'type' },
    { header: 'Endpoint', key: 'endpoint' },
    { header: 'Model', key: 'model' },
    {
      header: 'Default',
      key: 'isDefault',
      render: (row: Provider) => (
        row.isDefault ? (
          <span className="inline-flex px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700">
            Default
          </span>
        ) : <span className="text-gray-400">—</span>
      ),
    },
    {
      header: 'Active',
      key: 'active',
      render: (row: Provider) => (
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
      render: (row: Provider) => (
        <div className="flex gap-2 flex-wrap">
          <button
            onClick={(e) => { e.stopPropagation(); testConnection(row); }}
            disabled={testingId === row.id}
            className="text-xs px-3 py-1 rounded border border-gray-300 text-gray-600 hover:bg-gray-50 disabled:opacity-50"
          >
            {testingId === row.id ? 'Testing...' : 'Test'}
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); openEdit(row); }}
            className="text-xs px-3 py-1 rounded border border-blue-300 text-blue-600 hover:bg-blue-50"
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); deleteProvider(row); }}
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
          <h1 className="text-2xl font-bold text-gray-900">AI Providers</h1>
          <p className="text-gray-500 text-sm mt-1">Manage AI provider configurations</p>
        </div>
        <button
          onClick={openCreate}
          className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors"
        >
          + Add Provider
        </button>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

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
              <button
                onClick={() => setShowModal(false)}
                className="px-4 py-2 text-sm rounded-lg border border-gray-300 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={saveProvider}
                disabled={saving || !form.name || !form.model}
                className="px-4 py-2 text-sm rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {saving ? 'Saving...' : editingProvider ? 'Update' : 'Create'}
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
                placeholder="e.g. OpenAI GPT-4"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Type</label>
              <select
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value })}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="ollama">Ollama</option>
                <option value="custom">Custom</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Endpoint</label>
              <input
                type="text"
                value={form.endpoint}
                onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
                placeholder="e.g. https://api.openai.com/v1"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Model</label>
              <input
                type="text"
                value={form.model}
                onChange={(e) => setForm({ ...form, model: e.target.value })}
                placeholder="e.g. gpt-4o"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                API Key {editingProvider && <span className="text-gray-400">(leave blank to keep existing)</span>}
              </label>
              <input
                type="password"
                value={form.apiKey}
                onChange={(e) => setForm({ ...form, apiKey: e.target.value })}
                placeholder="sk-..."
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div className="flex gap-6">
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="isDefault"
                  checked={form.isDefault}
                  onChange={(e) => setForm({ ...form, isDefault: e.target.checked })}
                  className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <label htmlFor="isDefault" className="text-sm font-medium text-gray-700">Default</label>
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="providerActive"
                  checked={form.active}
                  onChange={(e) => setForm({ ...form, active: e.target.checked })}
                  className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <label htmlFor="providerActive" className="text-sm font-medium text-gray-700">Active</label>
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
