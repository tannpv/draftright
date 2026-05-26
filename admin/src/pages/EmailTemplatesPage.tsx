import { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '../api';
import Toast from '../components/Toast';

interface Template {
  key: string;
  label: string;
  variables: string[];
  subject: string;
  html: string;
  customized: boolean;
  default_subject: string;
  default_html: string;
}

export default function EmailTemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [selKey, setSelKey] = useState<string | null>(null);
  const [subject, setSubject] = useState('');
  const [html, setHtml] = useState('');
  const [saving, setSaving] = useState(false);
  const [preview, setPreview] = useState<{ subject: string; html: string } | null>(null);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  const sel = templates.find((t) => t.key === selKey) || null;
  const dirty = !!sel && (subject !== sel.subject || html !== sel.html);

  const load = useCallback(async () => {
    const data = await apiFetch('/admin/email-templates') as Template[];
    setTemplates(data);
    setSelKey((k) => k ?? data[0]?.key ?? null);
  }, []);
  useEffect(() => { load(); }, [load]);

  // Load editor fields when selection changes.
  useEffect(() => {
    if (sel) { setSubject(sel.subject); setHtml(sel.html); setPreview(null); }
  }, [selKey, templates.length]); // eslint-disable-line react-hooks/exhaustive-deps

  async function save() {
    if (!sel) return;
    setSaving(true);
    try {
      await apiFetch(`/admin/email-templates/${sel.key}`, { method: 'PATCH', body: JSON.stringify({ subject, html }) });
      setToast({ message: 'Template saved.', type: 'success' });
      await load();
    } catch (e) {
      setToast({ message: e instanceof Error ? e.message : 'Save failed', type: 'error' });
    } finally { setSaving(false); }
  }

  async function reset() {
    if (!sel || !confirm(`Reset "${sel.label}" to the built-in default?`)) return;
    try {
      await apiFetch(`/admin/email-templates/${sel.key}`, { method: 'DELETE' });
      setToast({ message: 'Reset to default.', type: 'success' });
      setSubject(sel.default_subject); setHtml(sel.default_html);
      await load();
    } catch (e) {
      setToast({ message: e instanceof Error ? e.message : 'Reset failed', type: 'error' });
    }
  }

  async function doPreview() {
    if (!sel) return;
    try {
      // Preview the SAVED version; save first if there are edits.
      if (dirty) await save();
      const p = await apiFetch(`/admin/email-templates/${sel.key}/preview`) as { subject: string; html: string };
      setPreview(p);
    } catch (e) {
      setToast({ message: e instanceof Error ? e.message : 'Preview failed', type: 'error' });
    }
  }

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Email Templates</h1>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
          Edit transactional emails. Use <code>{'{{variable}}'}</code> tokens; blank/reset falls back to the built-in default.
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '240px 1fr', gap: 20, alignItems: 'start' }}>
        {/* List */}
        <div style={{ background: 'var(--card)', borderRadius: 7, overflow: 'hidden' }}>
          {templates.map((t) => (
            <button key={t.key} onClick={() => setSelKey(t.key)} style={{
              display: 'block', width: '100%', textAlign: 'left', padding: '12px 16px', border: 'none', cursor: 'pointer',
              borderBottom: '1px solid var(--border)', fontFamily: 'inherit', fontSize: 13,
              background: selKey === t.key ? 'rgba(93,135,255,0.12)' : 'transparent',
              color: selKey === t.key ? 'var(--primary)' : 'var(--text)',
            }}>
              {t.label}
              {t.customized && <span style={{ marginLeft: 6, fontSize: 10, color: 'var(--warning)' }}>● edited</span>}
            </button>
          ))}
        </div>

        {/* Editor */}
        {sel && (
          <div style={{ background: 'var(--card)', borderRadius: 7, padding: 20 }}>
            <div style={{ marginBottom: 12 }}>
              <span style={{ color: 'var(--muted)', fontSize: 12 }}>Variables: </span>
              {sel.variables.map((v) => (
                <code key={v} style={{ fontSize: 12, background: 'var(--bg)', color: 'var(--secondary)', padding: '2px 6px', borderRadius: 4, marginRight: 6 }}>{`{{${v}}}`}</code>
              ))}
            </div>
            <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Subject</label>
            <input value={subject} onChange={(e) => setSubject(e.target.value)} className="dark-input" style={{ width: '100%', marginBottom: 16 }} />
            <label style={{ display: 'block', color: 'var(--text)', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>HTML body</label>
            <textarea value={html} onChange={(e) => setHtml(e.target.value)} rows={16} className="dark-input"
              style={{ width: '100%', fontFamily: 'monospace', fontSize: 12, resize: 'vertical', boxSizing: 'border-box' }} />

            <div style={{ display: 'flex', gap: 10, marginTop: 16 }}>
              <button onClick={save} disabled={saving || !dirty} className="btn btn-primary btn-sm">{saving ? 'Saving…' : 'Save'}</button>
              <button onClick={doPreview} className="btn btn-sm" style={{ border: '1px solid var(--border)', color: 'var(--text)' }}>Preview</button>
              {sel.customized && <button onClick={reset} className="btn btn-sm" style={{ marginLeft: 'auto', color: 'var(--danger)', border: '1px solid var(--border)' }}>Reset to default</button>}
            </div>

            {preview && (
              <div style={{ marginTop: 20 }}>
                <p style={{ color: 'var(--muted)', fontSize: 12, margin: '0 0 6px' }}>Preview — subject: <strong style={{ color: 'var(--text)' }}>{preview.subject}</strong></p>
                <iframe title="preview" srcDoc={preview.html} style={{ width: '100%', height: 360, border: '1px solid var(--border)', borderRadius: 7, background: '#fff' }} />
              </div>
            )}
          </div>
        )}
      </div>

      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  );
}
