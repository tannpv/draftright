import { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '../api';
import Toast from '../components/Toast';
import { timeAgo } from '../lib/format';
import { toneStyle, type Tone } from '../lib/status';

/* ── Types ────────────────────────────────────────────── */

interface ErrorReport {
  id: string;
  display_no: number | null;
  platform: string;
  app_version: string | null;
  severity: string;
  error_type: string | null;
  message: string | null;
  stack_trace: string | null;
  context: Record<string, any> | null;
  user_id: string | null;
  device_id: string | null;
  fingerprint: string;
  count: number;
  status: number;
  ai_fix_proposal: string | null;
  resolved_by: string | null;
  resolved_at: string | null;
  first_seen_at: string;
  last_seen_at: string;
}

interface ListResponse {
  items: ErrorReport[];
  total: number;
}

const STATUS_LABELS: Record<number, string> = {
  0: 'NEW',
  1: 'TRIAGED',
  2: 'IN PROGRESS',
  3: 'FIX PROPOSED',
  4: 'RESOLVED',
  5: 'CLOSED',
  6: "WON'T FIX",
  7: 'DUPLICATE',
};

const PLATFORM_ICONS: Record<string, string> = {
  ios: '\u{1F4F1}',
  android: '\u{1F916}',
  macos: '\u{1F34E}',
  windows: '\u{1F5A5}\u{FE0F}',
  linux: '\u{1F427}',
  web: '\u{1F310}',
};

function severityStyle(severity: string): { color: string; bg: string } {
  switch (severity) {
    case 'fatal':   return { color: 'var(--danger)', bg: 'rgba(250,137,107,0.12)' };
    case 'error':   return { color: 'var(--warning)', bg: 'rgba(255,174,31,0.12)' };
    case 'warning': return { color: 'var(--secondary)', bg: 'rgba(73,190,255,0.12)' };
    case 'info':    return { color: 'var(--success)', bg: 'rgba(19,222,185,0.12)' };
    default:        return { color: 'var(--muted)', bg: 'rgba(124,143,172,0.12)' };
  }
}

const ERROR_TONE: Record<number, Tone> = { 0: 'danger', 1: 'warning', 2: 'warning', 3: 'warning', 4: 'info', 5: 'success' };
const statusStyle = (status: number) => toneStyle(ERROR_TONE[status] ?? 'muted');

/* ── Page ─────────────────────────────────────────────── */

export default function ErrorsPage() {
  const [items, setItems] = useState<ErrorReport[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [platform, setPlatform] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [severityFilter, setSeverityFilter] = useState('');
  const [selected, setSelected] = useState<ErrorReport | null>(null);
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (platform) params.set('platform', platform);
      if (statusFilter !== '') params.set('status', statusFilter);
      if (severityFilter) params.set('severity', severityFilter);
      params.set('limit', '100');
      const data = await apiFetch(`/admin/errors?${params}`) as ListResponse;
      setItems(data.items);
      setTotal(data.total);
    } catch (e: any) {
      setToast({ msg: e.message || 'Failed to load errors', type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [platform, statusFilter, severityFilter]);

  useEffect(() => { load(); }, [load]);

  const setStatus = async (id: string, status: number) => {
    try {
      await apiFetch(`/admin/errors/${id}`, {
        method: 'PATCH',
        body: JSON.stringify({ status }),
      });
      setToast({ msg: `Marked ${STATUS_LABELS[status]}`, type: 'success' });
      await load();
      if (selected?.id === id) setSelected({ ...selected, status });
    } catch (e: any) {
      setToast({ msg: e.message || 'Update failed', type: 'error' });
    }
  };

  const [deleting, setDeleting] = useState(false);
  const deleteError = async (id: string) => {
    // Hard-delete confirmation. Errors are auto-captured noise more often
    // than real signal, so admins frequently sweep them; one confirm is
    // enough to prevent the slip but doesn't get in the way.
    if (!window.confirm('Delete this error report permanently? This cannot be undone.')) return;
    setDeleting(true);
    try {
      await apiFetch(`/admin/errors/${id}`, { method: 'DELETE' });
      setToast({ msg: 'Error deleted', type: 'success' });
      if (selected?.id === id) setSelected(null);
      await load();
    } catch (e: any) {
      setToast({ msg: e.message || 'Delete failed', type: 'error' });
    } finally {
      setDeleting(false);
    }
  };

  const [suggesting, setSuggesting] = useState(false);
  const [runningCron, setRunningCron] = useState(false);
  const runAiCron = async () => {
    setRunningCron(true);
    try {
      await apiFetch('/admin/errors/run-ai-cron', { method: 'POST' });
      setToast({ msg: 'AI cron triggered — check rows that flipped to FIX_PROPOSED', type: 'success' });
      await load();
    } catch (e: any) {
      setToast({ msg: e.message || 'Cron failed', type: 'error' });
    } finally {
      setRunningCron(false);
    }
  };
  const suggestFix = async (id: string) => {
    setSuggesting(true);
    try {
      const updated = await apiFetch(`/admin/errors/${id}/suggest-fix`, {
        method: 'POST',
      }) as ErrorReport;
      setToast({ msg: 'AI fix proposal saved', type: 'success' });
      await load();
      if (selected?.id === id) setSelected(updated);
    } catch (e: any) {
      setToast({ msg: e.message || 'Suggestion failed', type: 'error' });
    } finally {
      setSuggesting(false);
    }
  };

  return (
    <div className="p-6 max-w-[1600px] mx-auto">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-2xl font-bold text-white">Error Reports</h1>
          <p className="text-sm text-[var(--muted)] mt-1">
            {total} total &middot; Bug fingerprints from all client platforms.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={runAiCron}
            disabled={runningCron}
            title="Run the AI fix-proposal cron right now (also runs hourly automatically)"
            className="rounded-md bg-[var(--card)] hover:bg-[var(--primary)]/30 disabled:opacity-50 text-[var(--text)] text-sm font-medium px-4 py-2 border border-[var(--border)] transition-colors"
          >
            {runningCron ? '🧠 Cron running…' : '🧠 Run AI on new errors'}
          </button>
          <button
            onClick={load}
            className="rounded-md bg-[var(--primary)] hover:bg-[#3b6cff] text-white text-sm font-medium px-4 py-2 transition-colors"
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 mb-4 p-4 bg-[var(--card)] rounded-lg border border-[var(--border)]">
        <select
          value={platform}
          onChange={(e) => setPlatform(e.target.value)}
          className="bg-[var(--bg)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text)]"
        >
          <option value="">All platforms</option>
          <option value="ios">iOS</option>
          <option value="android">Android</option>
          <option value="macos">macOS</option>
          <option value="windows">Windows</option>
          <option value="linux">Linux</option>
          <option value="web">Web</option>
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-[var(--bg)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text)]"
        >
          <option value="">All statuses</option>
          {Object.entries(STATUS_LABELS).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
        <select
          value={severityFilter}
          onChange={(e) => setSeverityFilter(e.target.value)}
          className="bg-[var(--bg)] border border-[var(--border)] rounded px-3 py-2 text-sm text-[var(--text)]"
        >
          <option value="">All severity</option>
          <option value="fatal">Fatal</option>
          <option value="error">Error</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
      </div>

      {/* Table */}
      <div className="bg-[var(--card)] border border-[var(--border)] rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-[var(--bg)] text-[var(--muted)] text-xs uppercase tracking-wider">
            <tr>
              <th className="px-4 py-3 text-left">Ref</th>
              <th className="px-4 py-3 text-left">Platform</th>
              <th className="px-4 py-3 text-left">Type / Message</th>
              <th className="px-4 py-3 text-left">Severity</th>
              <th className="px-4 py-3 text-left">Count</th>
              <th className="px-4 py-3 text-left">Status</th>
              <th className="px-4 py-3 text-left">Last seen</th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr><td colSpan={7} className="text-center text-[var(--muted)] py-12">Loading…</td></tr>
            )}
            {!loading && items.length === 0 && (
              <tr><td colSpan={7} className="text-center text-[var(--muted)] py-12">No errors collected yet. Either things are going great, or no clients have reported in.</td></tr>
            )}
            {items.map((row) => (
              <tr
                key={row.id}
                onClick={() => setSelected(row)}
                className="border-t border-[var(--border)] hover:bg-[var(--bg)] cursor-pointer"
              >
                <td className="px-4 py-3 font-mono text-xs font-semibold text-[var(--primary)] whitespace-nowrap">
                  {row.display_no != null ? `ERR-${row.display_no}` : '—'}
                </td>
                <td className="px-4 py-3 text-[var(--text)]">
                  {PLATFORM_ICONS[row.platform] || '?'} {row.platform}
                  <div className="text-xs text-[var(--muted)]">{row.app_version || ''}</div>
                </td>
                <td className="px-4 py-3 text-[var(--text)]">
                  <div className="font-mono text-xs">{row.error_type || '(no type)'}</div>
                  <div className="text-xs text-[var(--muted)] truncate max-w-md">{row.message || ''}</div>
                </td>
                <td className="px-4 py-3">
                  <span
                    className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold"
                    style={severityStyle(row.severity)}
                  >
                    {row.severity}
                  </span>
                </td>
                <td className="px-4 py-3 text-[var(--text)] font-mono">{row.count}</td>
                <td className="px-4 py-3">
                  <span
                    className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold"
                    style={statusStyle(row.status)}
                  >
                    {STATUS_LABELS[row.status] || row.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-[var(--muted)] text-xs">{timeAgo(row.last_seen_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Detail drawer */}
      {selected && (
        <div className="fixed inset-0 bg-black/50 z-40 flex justify-end" onClick={() => setSelected(null)}>
          <div
            className="bg-[var(--bg)] border-l border-[var(--border)] w-full max-w-2xl h-full overflow-y-auto p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between mb-4">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  {selected.display_no != null && (
                    <span className="font-mono text-xs font-semibold text-[var(--primary)] bg-[var(--primary)]/15 px-2 py-0.5 rounded">
                      ERR-{selected.display_no}
                    </span>
                  )}
                </div>
                <h2 className="text-lg font-bold text-white">
                  {PLATFORM_ICONS[selected.platform]} {selected.error_type || 'Error'}
                </h2>
                <p className="text-xs text-[var(--muted)] mt-1 font-mono">{selected.fingerprint.slice(0, 16)}…</p>
              </div>
              <button
                onClick={() => setSelected(null)}
                className="text-[var(--muted)] hover:text-white text-2xl leading-none"
              >&times;</button>
            </div>

            <div className="grid grid-cols-2 gap-4 mb-4">
              <Stat label="Count" value={selected.count} />
              <Stat label="Severity" value={selected.severity} />
              <Stat label="Platform" value={selected.platform} />
              <Stat label="App version" value={selected.app_version || '—'} />
              <Stat label="First seen" value={timeAgo(selected.first_seen_at)} />
              <Stat label="Last seen" value={timeAgo(selected.last_seen_at)} />
            </div>

            {selected.message && (
              <Block label="Message">
                <pre className="whitespace-pre-wrap text-sm text-[var(--text)]">{selected.message}</pre>
              </Block>
            )}

            {selected.stack_trace && (
              <Block label="Stack trace">
                <pre className="whitespace-pre-wrap text-xs text-[var(--text)] font-mono overflow-x-auto">{selected.stack_trace}</pre>
              </Block>
            )}

            {selected.context && (
              <Block label="Context">
                <pre className="whitespace-pre-wrap text-xs text-[var(--text)] font-mono overflow-x-auto">{JSON.stringify(selected.context, null, 2)}</pre>
              </Block>
            )}

            {selected.ai_fix_proposal && (
              <Block label="AI fix proposal">
                <pre className="whitespace-pre-wrap text-sm text-[var(--text)]">{selected.ai_fix_proposal}</pre>
              </Block>
            )}

            <div className="mt-4 pt-4 border-t border-[var(--border)]">
              <button
                onClick={() => suggestFix(selected.id)}
                disabled={suggesting}
                className="w-full rounded-md bg-[var(--primary)] hover:bg-[#3b6cff] disabled:opacity-50 text-white text-sm font-medium px-4 py-2 transition-colors"
              >
                {suggesting ? '🧠 Asking AI…' : selected.ai_fix_proposal ? '🧠 Re-ask AI for new proposal' : '🧠 Suggest fix with AI'}
              </button>
              <p className="text-xs text-[var(--muted)] mt-2">
                Sends the error type, message, stack trace, and context to the configured AI provider. Sets status to FIX PROPOSED.
              </p>
            </div>

            <div className="mt-6 pt-4 border-t border-[var(--border)]">
              <p className="text-xs text-[var(--muted)] uppercase tracking-wider mb-2">Mark status</p>
              <div className="flex flex-wrap gap-2">
                {Object.entries(STATUS_LABELS).map(([k, v]) => (
                  <button
                    key={k}
                    disabled={selected.status === Number(k)}
                    onClick={() => setStatus(selected.id, Number(k))}
                    className="text-xs font-medium px-3 py-1 rounded-full border border-[var(--border)] hover:bg-[var(--primary)]/20 disabled:opacity-30"
                  >{v}</button>
                ))}
              </div>
            </div>

            <div className="mt-6 pt-4 border-t border-[var(--border)]">
              <button
                onClick={() => deleteError(selected.id)}
                disabled={deleting}
                className="w-full rounded-md bg-[var(--danger)] hover:bg-[#e57254] disabled:opacity-50 text-white text-sm font-medium px-4 py-2 transition-colors"
              >
                {deleting ? 'Deleting…' : '🗑 Delete error permanently'}
              </button>
            </div>
          </div>
        </div>
      )}

      {toast && (
        <Toast message={toast.msg} type={toast.type} onClose={() => setToast(null)} />
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: any }) {
  return (
    <div className="bg-[var(--card)] border border-[var(--border)] rounded p-3">
      <div className="text-xs text-[var(--muted)] uppercase tracking-wider">{label}</div>
      <div className="text-sm text-[var(--text)] mt-1">{value}</div>
    </div>
  );
}

function Block({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-4">
      <div className="text-xs text-[var(--muted)] uppercase tracking-wider mb-1">{label}</div>
      <div className="bg-[var(--card)] border border-[var(--border)] rounded p-3 max-h-72 overflow-auto">{children}</div>
    </div>
  );
}
