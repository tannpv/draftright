import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import Modal from '../components/Modal';
import Toast from '../components/Toast';
import { apiFetch, SEARCH_DEBOUNCE_MS } from '../api';

interface Provider {
  id: string;
  name: string;
  type: string;
  endpoint_url: string;
  model: string;
  api_key: string;
  temperature: number;
  is_default: boolean;
  is_active: boolean;
  [key: string]: unknown;
}

const PROVIDER_PRESETS: Record<string, { endpoint: string; model: string; needsKey: boolean }> = {
  openai: { endpoint: 'https://api.openai.com/v1/chat/completions', model: 'gpt-4o', needsKey: true },
  anthropic: { endpoint: 'https://api.anthropic.com/v1/messages', model: 'claude-sonnet-5', needsKey: true },
  google: { endpoint: 'https://generativelanguage.googleapis.com/v1beta/openai/chat/completions', model: 'gemini-2.5-flash', needsKey: true },
  ollama: { endpoint: 'http://localhost:11434/v1/chat/completions', model: 'llama3.2:latest', needsKey: false },
  custom: { endpoint: '', model: '', needsKey: true },
};

// Known models per provider type, newest/best first — the Model field is a
// dropdown of these. Extend here as new models ship; to add Gemini later,
// add a 'google' key here + a Type option + backend AiProviderType.GOOGLE
// + strategy. Single source: presets above pick their default from this list.
const MODEL_OPTIONS: Record<string, string[]> = {
  openai: ['gpt-4o', 'gpt-4o-mini', 'gpt-4.1', 'gpt-4.1-mini', 'gpt-5', 'gpt-5-mini', 'gpt-5-nano'],
  anthropic: ['claude-sonnet-5', 'claude-opus-4-8', 'claude-haiku-4-5-20251001', 'claude-sonnet-4-20250514'],
  google: ['gemini-2.5-pro', 'gemini-2.5-flash', 'gemini-2.0-flash'],
  ollama: ['llama3.2:latest', 'llama3.1:latest', 'qwen2.5:latest', 'mistral:latest'],
  custom: [],
};

// Sentinel <option> that reveals the free-text box, so a brand-new model
// not yet in MODEL_OPTIONS is never blocked.
const CUSTOM_MODEL = '__custom__';

function isKnownModel(type: string, model: string): boolean {
  return (MODEL_OPTIONS[type] ?? []).includes(model);
}

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

const emptyForm = {
  name: '',
  type: 'openai',
  // Derived from the preset so the default endpoint/model live in one place.
  endpoint_url: PROVIDER_PRESETS.openai.endpoint,
  model: PROVIDER_PRESETS.openai.model,
  api_key: '',
  is_default: false,
  is_active: true,
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
  // True when the admin is entering a model not in MODEL_OPTIONS (free text).
  const [modelCustom, setModelCustom] = useState(false);
  const [saving, setSaving] = useState(false);

  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [sortBy, setSortBy] = useState<string>('created_at');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');

  const fetchProviders = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(page), limit: String(pageSize),
        status: statusFilter, sort_by: sortBy, sort_order: sortOrder,
      });
      if (search) params.set('search', search);
      const data = await apiFetch(`/admin/ai-providers/paginated?${params}`) as { rows: Provider[]; total: number };
      setProviders(data.rows ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load providers');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, search, sortBy, sortOrder]);

  useEffect(() => {
    fetchProviders();
  }, [fetchProviders]);

  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [searchInput]);

  function openCreate() {
    setEditingProvider(null);
    setForm(emptyForm);
    setModelCustom(!isKnownModel(emptyForm.type, emptyForm.model));
    setShowModal(true);
  }

  function openEdit(provider: Provider) {
    setEditingProvider(provider);
    setForm({
      name: provider.name,
      type: provider.type,
      endpoint_url: provider.endpoint_url,
      model: provider.model,
      api_key: '',
      is_default: provider.is_default,
      is_active: provider.is_active,
    });
    // An existing provider may hold a model not in the catalogue → free text.
    setModelCustom(!isKnownModel(provider.type, provider.model));
    setShowModal(true);
  }

  function handleTypeChange(newType: string) {
    const preset = PROVIDER_PRESETS[newType];
    const model = preset?.model || form.model;
    setForm({
      ...form,
      type: newType,
      endpoint_url: preset?.endpoint || form.endpoint_url,
      model,
    });
    setModelCustom(!isKnownModel(newType, model));
  }

  async function saveProvider() {
    setSaving(true);
    const payload: Record<string, unknown> = {
      name: form.name,
      type: form.type,
      endpoint_url: form.endpoint_url,
      model: form.model,
      is_default: form.is_default,
      is_active: form.is_active,
    };
    if (form.api_key) payload.api_key = form.api_key;

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
      sortKey: 'name',
      render: (row: Provider) => <span style={{ color: 'var(--text)', fontWeight: 600 }}>{row.name}</span>,
    },
    {
      header: 'Type',
      key: 'type',
      sortKey: 'type',
      render: (row: Provider) => <span style={{ color: 'var(--muted)', textTransform: 'capitalize' }}>{row.type}</span>,
    },
    {
      header: 'Endpoint',
      key: 'endpoint_url',
      render: (row: Provider) => (
        <span style={{ color: 'var(--muted)', fontSize: 12, fontFamily: 'monospace', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', display: 'inline-block' }}>
          {row.endpoint_url}
        </span>
      ),
    },
    {
      header: 'Model',
      key: 'model',
      sortKey: 'model',
      render: (row: Provider) => (
        <span style={{ color: 'var(--secondary)', fontSize: 12, fontFamily: 'monospace' }}>{row.model}</span>
      ),
    },
    {
      header: 'Default',
      key: 'is_default',
      sortKey: 'is_default',
      render: (row: Provider) =>
        row.is_default
          ? <span className="badge badge-primary">Default</span>
          : <span style={{ color: 'var(--border)' }}>—</span>,
    },
    {
      header: 'Active',
      key: 'is_active',
      sortKey: 'is_active',
      render: (row: Provider) => (
        <span className={`badge ${row.is_active ? 'badge-success' : 'badge-muted'}`}>
          {row.is_active ? 'Yes' : 'No'}
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
            style={{ background: 'rgba(124,143,172,0.1)', color: 'var(--muted)', border: '1px solid var(--border)', opacity: testingId === row.id ? 0.5 : 1 }}
          >
            {testingId === row.id ? 'Testing...' : 'Test'}
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); openEdit(row); }}
            className="btn btn-sm"
            style={{ background: 'rgba(93,135,255,0.1)', color: 'var(--primary)', border: '1px solid rgba(93,135,255,0.2)' }}
          >
            Edit
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); deleteProvider(row); }}
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
          <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>AI Providers</h1>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>Manage AI provider configurations</p>
        </div>
        <button onClick={openCreate} className="btn btn-primary">
          + Add Provider
        </button>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 16 }}>{error}</div>}

      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Search by name, type, or model..."
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
          {total > 0 ? `${total} ${total === 1 ? 'provider' : 'providers'}` : ''}
        </span>
      </div>

      <DataTable<Provider>
        columns={columns}
        rows={providers}
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
        emptyMessage={search || statusFilter !== 'all' ? 'No matches.' : 'No providers configured. Add one to get started.'}
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
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. OpenAI GPT-4"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Type</label>
              <select
                value={form.type}
                onChange={(e) => handleTypeChange(e.target.value)}
                className="dark-input"
              >
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic (Claude)</option>
                <option value="google">Google (Gemini)</option>
                <option value="ollama">Ollama (Local)</option>
                <option value="custom">Custom</option>
              </select>
            </div>
            <div>
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Endpoint URL</label>
              <input
                type="text"
                value={form.endpoint_url}
                onChange={(e) => setForm({ ...form, endpoint_url: e.target.value })}
                placeholder="e.g. https://api.openai.com/v1/chat/completions"
                className="dark-input"
              />
            </div>
            <div>
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Model</label>
              {(MODEL_OPTIONS[form.type] ?? []).length > 0 && (
                <select
                  value={modelCustom ? CUSTOM_MODEL : form.model}
                  onChange={(e) => {
                    if (e.target.value === CUSTOM_MODEL) {
                      setModelCustom(true);
                      setForm({ ...form, model: '' });
                    } else {
                      setModelCustom(false);
                      setForm({ ...form, model: e.target.value });
                    }
                  }}
                  className="dark-input"
                >
                  {(MODEL_OPTIONS[form.type] ?? []).map((m) => (
                    <option key={m} value={m}>{m}</option>
                  ))}
                  <option value={CUSTOM_MODEL}>Other (enter manually)…</option>
                </select>
              )}
              {/* Free text when the type has no catalogue (custom) or the
                  admin picked "Other" — never blocks a brand-new model. */}
              {((MODEL_OPTIONS[form.type] ?? []).length === 0 || modelCustom) && (
                <input
                  type="text"
                  value={form.model}
                  onChange={(e) => setForm({ ...form, model: e.target.value })}
                  placeholder="e.g. gpt-4o"
                  className="dark-input"
                  style={{ marginTop: (MODEL_OPTIONS[form.type] ?? []).length > 0 ? 8 : 0 }}
                />
              )}
            </div>
            <div>
              <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                API Key{editingProvider && <span style={{ color: 'var(--muted)', fontWeight: 400 }}> (leave blank to keep existing)</span>}
                {form.type === 'ollama' && <span style={{ color: 'var(--success)', fontWeight: 400 }}> (not required for Ollama)</span>}
              </label>
              <input
                type="password"
                value={form.api_key}
                onChange={(e) => setForm({ ...form, api_key: e.target.value })}
                placeholder={form.type === 'anthropic' ? 'sk-ant-...' : form.type === 'google' ? 'AIza...' : form.type === 'ollama' ? '(optional)' : 'sk-...'}
                className="dark-input"
              />
            </div>
            <div style={{ display: 'flex', gap: 24 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  id="isDefault"
                  checked={form.is_default}
                  onChange={(e) => setForm({ ...form, is_default: e.target.checked })}
                  style={{ width: 16, height: 16, accentColor: 'var(--primary)', cursor: 'pointer' }}
                />
                <label htmlFor="isDefault" style={{ color: 'var(--text)', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>Default</label>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  id="providerActive"
                  checked={form.is_active}
                  onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                  style={{ width: 16, height: 16, accentColor: 'var(--primary)', cursor: 'pointer' }}
                />
                <label htmlFor="providerActive" style={{ color: 'var(--text)', fontSize: 13, fontWeight: 500, cursor: 'pointer' }}>Active</label>
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
