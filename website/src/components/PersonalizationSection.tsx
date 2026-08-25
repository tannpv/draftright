import { useEffect, useState } from 'react';

import { API_URL as API } from '../lib/api';

// UI-side length hints that mirror the backend DTO caps
// (backend-rewrite-go/internal/usercontext/handler.go — fieldMaxLen/notesMaxLen).
// The backend is the enforcer; these just stop the user typing past the limit.
// Keep in sync if the backend caps change.
const FIELD_MAX = 120;
const NOTES_MAX = 1000;

interface ContextForm {
  enabled: boolean;
  job_title: string;
  industry: string;
  audience: string;
  style_notes: string;
}

const EMPTY: ContextForm = {
  enabled: false,
  job_title: '',
  industry: '',
  audience: '',
  style_notes: '',
};

/**
 * Personalization ("a specialist for you", #173). Lets a signed-in user give the
 * AI context about who they are so rewrites are tailored. Opt-in: nothing is
 * used until the user fills it in and enables it. Reads/writes GET/PUT/DELETE
 * /me/context (scoped server-side to the caller).
 */
export default function PersonalizationSection({ token }: { token: string }) {
  const [form, setForm] = useState<ContextForm>(EMPTY);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch(`${API}/me/context`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) setForm({ ...EMPTY, ...(await res.json()) });
      } catch {
        // Non-fatal — show empty defaults; save still works.
      } finally {
        setLoading(false);
      }
    })();
  }, [token]);

  const set = <K extends keyof ContextForm>(key: K, value: ContextForm[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
    setSaved(false);
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(`${API}/me/context`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(form),
      });
      if (!res.ok) throw new Error(`Save failed (${res.status})`);
      setForm({ ...EMPTY, ...(await res.json()) });
      setSaved(true);
    } catch (e: any) {
      setError(e?.message ?? 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const clear = async () => {
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(`${API}/me/context`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok && res.status !== 204) throw new Error(`Clear failed (${res.status})`);
      setForm(EMPTY);
      setSaved(false);
    } catch (e: any) {
      setError(e?.message ?? 'Clear failed');
    } finally {
      setSaving(false);
    }
  };

  const field = (
    label: string,
    key: keyof ContextForm,
    placeholder: string,
  ) => (
    <label className="block">
      <span className="text-sm text-gray-400">{label}</span>
      <input
        type="text"
        value={form[key] as string}
        maxLength={FIELD_MAX}
        placeholder={placeholder}
        onChange={(e) => set(key, e.target.value)}
        className="mt-1 w-full rounded-lg border border-dark-border bg-dark-bg px-3 py-2 text-white placeholder-gray-600 focus:border-brand-400 focus:outline-none"
      />
    </label>
  );

  return (
    <div className="rounded-2xl border border-dark-border bg-dark-card p-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-white">Personalization</h2>
          <p className="mt-1 text-sm text-gray-500">
            Tell the AI who you are so rewrites match your role and style. Optional and private — used only when enabled.
          </p>
        </div>
      </div>

      {loading ? (
        <p className="mt-6 text-sm text-gray-500">Loading…</p>
      ) : (
        <div className="mt-6 space-y-4">
          <label className="flex items-center gap-3">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={(e) => set('enabled', e.target.checked)}
              className="h-4 w-4 accent-brand-400"
            />
            <span className="text-sm text-white">Use my personalization on rewrites</span>
          </label>

          <div className="grid gap-4 sm:grid-cols-3">
            {field('Job title', 'job_title', 'e.g. Lawyer')}
            {field('Industry', 'industry', 'e.g. finance')}
            {field('Audience', 'audience', 'e.g. clients')}
          </div>

          <label className="block">
            <span className="text-sm text-gray-400">Writing style</span>
            <textarea
              value={form.style_notes}
              maxLength={NOTES_MAX}
              placeholder="e.g. formal, British spelling, no emojis"
              onChange={(e) => set('style_notes', e.target.value)}
              rows={3}
              className="mt-1 w-full rounded-lg border border-dark-border bg-dark-bg px-3 py-2 text-white placeholder-gray-600 focus:border-brand-400 focus:outline-none"
            />
            <span className="mt-1 block text-right text-xs text-gray-600">
              {form.style_notes.length}/{NOTES_MAX}
            </span>
          </label>

          {error && (
            <p className="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</p>
          )}
          {saved && (
            <p className="rounded-lg border border-brand-400/30 bg-brand-400/10 p-3 text-sm text-brand-300">Saved.</p>
          )}

          <div className="flex flex-wrap items-center gap-3">
            <button
              onClick={save}
              disabled={saving}
              className="rounded-lg bg-brand-400 px-5 py-2 text-sm font-semibold text-dark-bg hover:bg-brand-300 disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
            <button
              onClick={clear}
              disabled={saving}
              className="text-sm text-gray-400 hover:text-white disabled:opacity-50"
            >
              Clear my personalization
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
