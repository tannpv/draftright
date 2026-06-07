import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { apiFetch } from '../api';

type Kind = 'error' | 'bug';

interface InboxItem {
  kind: Kind;
  id: string;
  title: string;
  platform: string | null;
  app_version: string | null;
  status: string;
  created_at: string;
  ai_fix_proposal: string | null;
  error_type?: string | null;
  severity?: string | null;
  occurrence_count?: number;
  user_email?: string | null;
  has_screenshot?: boolean;
}

interface InboxResponse {
  items: InboxItem[];
  counts: { errors: number; bugs: number; returned: number };
}

const KIND_BADGE: Record<Kind, string> = {
  error: 'bg-[var(--danger)]/15 text-[var(--danger)]',
  bug:   'bg-[var(--primary)]/15 text-[var(--primary)]',
};

const STATUS_BADGE: Record<string, string> = {
  new:           'bg-[var(--warning)]/15 text-[var(--warning)]',
  reviewing:     'bg-[var(--primary)]/15 text-[var(--primary)]',
  fix_proposed:  'bg-[var(--success)]/15 text-[var(--success)]',
  resolved:      'bg-[var(--muted)]/15 text-[var(--muted)]',
  wont_fix:      'bg-[var(--muted)]/15 text-[var(--muted)]',
};

function ageString(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  const min = Math.floor(ms / 60000);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  return `${day}d ago`;
}

export default function InboxPage() {
  const [data, setData] = useState<InboxResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [kindFilter, setKindFilter] = useState<'all' | Kind>('all');
  const [statusFilter, setStatusFilter] = useState<'all' | 'open'>('open');
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const params = new URLSearchParams();
      if (kindFilter !== 'all') params.set('kind', kindFilter);
      if (statusFilter !== 'all') params.set('status', statusFilter);
      params.set('limit', '100');
      const result = await apiFetch(`/admin/inbox?${params.toString()}`) as InboxResponse;
      setData(result);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Load failed');
    } finally {
      setLoading(false);
    }
  }, [kindFilter, statusFilter]);

  useEffect(() => { load(); }, [load]);

  function toggleExpand(key: string) {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  async function requestAiFix(item: InboxItem) {
    try {
      const path = item.kind === 'error'
        ? `/admin/errors/${item.id}/suggest-fix`
        : `/admin/bug-reports/${item.id}/fix-proposal`;
      await apiFetch(path, { method: 'POST' });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'AI fix request failed');
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-[var(--text)]">Inbox</h1>
        <button
          onClick={load}
          className="px-3 py-1.5 text-sm bg-[var(--primary)] text-white rounded hover:bg-[var(--primary)]/90"
        >
          Refresh
        </button>
      </div>

      <p className="text-sm text-[var(--muted)]">
        Unified feed of errors (auto-collected crashes) and bugs (user-submitted feedback).
        AI fix proposals run hourly via cron — click <em>Run AI Fix</em> to trigger immediately.
      </p>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="flex gap-1 bg-[var(--bg)] rounded p-1">
          {(['all', 'error', 'bug'] as const).map(k => (
            <button
              key={k}
              onClick={() => setKindFilter(k)}
              className={`px-3 py-1 text-xs rounded ${
                kindFilter === k ? 'bg-[var(--primary)] text-white' : 'text-[var(--muted)] hover:text-[var(--text)]'
              }`}
            >
              {k === 'all' ? 'All' : k === 'error' ? 'Errors' : 'Bugs'}
              {data && k !== 'all' && (
                <span className="ml-1 opacity-60">
                  ({k === 'error' ? data.counts.errors : data.counts.bugs})
                </span>
              )}
            </button>
          ))}
        </div>
        <div className="flex gap-1 bg-[var(--bg)] rounded p-1">
          {(['open', 'all'] as const).map(s => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={`px-3 py-1 text-xs rounded ${
                statusFilter === s ? 'bg-[var(--primary)] text-white' : 'text-[var(--muted)] hover:text-[var(--text)]'
              }`}
            >
              {s === 'open' ? 'Open only' : 'All statuses'}
            </button>
          ))}
        </div>
        <div className="ml-auto text-xs text-[var(--muted)]">
          {data ? `${data.counts.returned} of ${data.counts.errors + data.counts.bugs}` : ''}
        </div>
      </div>

      {error && <div className="p-3 bg-[var(--danger)]/10 text-[var(--danger)] rounded text-sm">{error}</div>}
      {loading && <div className="text-[var(--muted)] text-sm">Loading…</div>}

      {/* Feed */}
      <div className="space-y-2">
        {data?.items.map(item => {
          const key = `${item.kind}-${item.id}`;
          const isOpen = expanded.has(key);
          const detailLink = item.kind === 'error' ? `/errors` : `/bug-reports`;
          return (
            <div key={key} className="bg-[var(--card)] border border-[var(--border)] rounded-lg p-3">
              <div className="flex items-start gap-3">
                <span className={`text-[10px] uppercase px-2 py-1 rounded font-mono ${KIND_BADGE[item.kind]}`}>
                  {item.kind}
                </span>
                <span className={`text-[10px] uppercase px-2 py-1 rounded ${STATUS_BADGE[item.status] ?? STATUS_BADGE.new}`}>
                  {item.status.replace('_', ' ')}
                </span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-[var(--text)] truncate">{item.title}</div>
                  <div className="text-xs text-[var(--muted)] mt-0.5 flex items-center gap-3 flex-wrap">
                    <span>{ageString(item.created_at)}</span>
                    {item.platform && <span>{item.platform}</span>}
                    {item.app_version && <span>v{item.app_version}</span>}
                    {item.kind === 'error' && item.severity && <span>{item.severity}</span>}
                    {item.kind === 'error' && item.occurrence_count != null && item.occurrence_count > 1 && (
                      <span className="text-[var(--warning)]">×{item.occurrence_count}</span>
                    )}
                    {item.kind === 'bug' && item.user_email && <span>{item.user_email}</span>}
                    {item.kind === 'bug' && item.has_screenshot && <span>📷</span>}
                    {item.ai_fix_proposal && (
                      <span className="text-[var(--success)]">AI fix ready</span>
                    )}
                  </div>
                </div>
                <span
                  className="text-xs text-[var(--muted)] whitespace-nowrap self-start"
                  title={ageString(item.created_at)}
                >
                  {new Date(item.created_at).toLocaleString()}
                </span>
                <button
                  onClick={() => toggleExpand(key)}
                  className="text-xs text-[var(--primary)] hover:underline"
                >
                  {isOpen ? 'Hide' : (item.ai_fix_proposal ? 'Show fix' : 'Details')}
                </button>
              </div>

              {isOpen && (
                <div className="mt-3 pt-3 border-t border-[var(--border)] text-xs text-[var(--muted)] space-y-3">
                  {item.ai_fix_proposal ? (
                    <pre className="whitespace-pre-wrap font-mono text-[11px] text-[var(--text)] bg-[var(--bg)] p-3 rounded">
                      {item.ai_fix_proposal}
                    </pre>
                  ) : (
                    <div className="italic">No AI analysis yet.</div>
                  )}
                  <div className="flex items-center gap-2">
                    {!item.ai_fix_proposal && (
                      <button
                        onClick={() => requestAiFix(item)}
                        className="px-2.5 py-1 bg-[var(--success)]/15 text-[var(--success)] text-xs rounded hover:bg-[var(--success)]/25"
                      >
                        Run AI Fix
                      </button>
                    )}
                    <Link
                      to={detailLink}
                      className="px-2.5 py-1 bg-[var(--primary)]/15 text-[var(--primary)] text-xs rounded hover:bg-[var(--primary)]/25"
                    >
                      Open in {item.kind === 'error' ? 'Errors' : 'Bug Reports'} →
                    </Link>
                  </div>
                </div>
              )}
            </div>
          );
        })}
        {data && data.items.length === 0 && (
          <div className="text-[var(--muted)] text-sm italic p-4">No items match the current filter.</div>
        )}
      </div>
    </div>
  );
}
