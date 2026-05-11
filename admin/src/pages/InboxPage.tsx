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
  error: 'bg-[#fa896b]/15 text-[#fa896b]',
  bug:   'bg-[#5d87ff]/15 text-[#5d87ff]',
};

const STATUS_BADGE: Record<string, string> = {
  new:           'bg-[#ffae1f]/15 text-[#ffae1f]',
  reviewing:     'bg-[#5d87ff]/15 text-[#5d87ff]',
  fix_proposed:  'bg-[#13deb9]/15 text-[#13deb9]',
  resolved:      'bg-[#7c8fac]/15 text-[#7c8fac]',
  wont_fix:      'bg-[#7c8fac]/15 text-[#7c8fac]',
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
        <h1 className="text-2xl font-semibold text-[#eaeff4]">Inbox</h1>
        <button
          onClick={load}
          className="px-3 py-1.5 text-sm bg-[#5d87ff] text-white rounded hover:bg-[#5d87ff]/90"
        >
          Refresh
        </button>
      </div>

      <p className="text-sm text-[#7c8fac]">
        Unified feed of errors (auto-collected crashes) and bugs (user-submitted feedback).
        AI fix proposals run hourly via cron — click <em>Run AI Fix</em> to trigger immediately.
      </p>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="flex gap-1 bg-[#202936] rounded p-1">
          {(['all', 'error', 'bug'] as const).map(k => (
            <button
              key={k}
              onClick={() => setKindFilter(k)}
              className={`px-3 py-1 text-xs rounded ${
                kindFilter === k ? 'bg-[#5d87ff] text-white' : 'text-[#7c8fac] hover:text-[#eaeff4]'
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
        <div className="flex gap-1 bg-[#202936] rounded p-1">
          {(['open', 'all'] as const).map(s => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={`px-3 py-1 text-xs rounded ${
                statusFilter === s ? 'bg-[#5d87ff] text-white' : 'text-[#7c8fac] hover:text-[#eaeff4]'
              }`}
            >
              {s === 'open' ? 'Open only' : 'All statuses'}
            </button>
          ))}
        </div>
        <div className="ml-auto text-xs text-[#7c8fac]">
          {data ? `${data.counts.returned} of ${data.counts.errors + data.counts.bugs}` : ''}
        </div>
      </div>

      {error && <div className="p-3 bg-[#fa896b]/10 text-[#fa896b] rounded text-sm">{error}</div>}
      {loading && <div className="text-[#7c8fac] text-sm">Loading…</div>}

      {/* Feed */}
      <div className="space-y-2">
        {data?.items.map(item => {
          const key = `${item.kind}-${item.id}`;
          const isOpen = expanded.has(key);
          const detailLink = item.kind === 'error' ? `/errors` : `/bug-reports`;
          return (
            <div key={key} className="bg-[#2a3547] border border-[#333f55] rounded-lg p-3">
              <div className="flex items-start gap-3">
                <span className={`text-[10px] uppercase px-2 py-1 rounded font-mono ${KIND_BADGE[item.kind]}`}>
                  {item.kind}
                </span>
                <span className={`text-[10px] uppercase px-2 py-1 rounded ${STATUS_BADGE[item.status] ?? STATUS_BADGE.new}`}>
                  {item.status.replace('_', ' ')}
                </span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-[#eaeff4] truncate">{item.title}</div>
                  <div className="text-xs text-[#7c8fac] mt-0.5 flex items-center gap-3 flex-wrap">
                    <span>{ageString(item.created_at)}</span>
                    {item.platform && <span>{item.platform}</span>}
                    {item.app_version && <span>v{item.app_version}</span>}
                    {item.kind === 'error' && item.severity && <span>{item.severity}</span>}
                    {item.kind === 'error' && item.occurrence_count != null && item.occurrence_count > 1 && (
                      <span className="text-[#ffae1f]">×{item.occurrence_count}</span>
                    )}
                    {item.kind === 'bug' && item.user_email && <span>{item.user_email}</span>}
                    {item.kind === 'bug' && item.has_screenshot && <span>📷</span>}
                    {item.ai_fix_proposal && (
                      <span className="text-[#13deb9]">AI fix ready</span>
                    )}
                  </div>
                </div>
                <button
                  onClick={() => toggleExpand(key)}
                  className="text-xs text-[#5d87ff] hover:underline"
                >
                  {isOpen ? 'Hide' : (item.ai_fix_proposal ? 'Show fix' : 'Details')}
                </button>
              </div>

              {isOpen && (
                <div className="mt-3 pt-3 border-t border-[#333f55] text-xs text-[#7c8fac] space-y-3">
                  {item.ai_fix_proposal ? (
                    <pre className="whitespace-pre-wrap font-mono text-[11px] text-[#eaeff4] bg-[#202936] p-3 rounded">
                      {item.ai_fix_proposal}
                    </pre>
                  ) : (
                    <div className="italic">No AI analysis yet.</div>
                  )}
                  <div className="flex items-center gap-2">
                    {!item.ai_fix_proposal && (
                      <button
                        onClick={() => requestAiFix(item)}
                        className="px-2.5 py-1 bg-[#13deb9]/15 text-[#13deb9] text-xs rounded hover:bg-[#13deb9]/25"
                      >
                        Run AI Fix
                      </button>
                    )}
                    <Link
                      to={detailLink}
                      className="px-2.5 py-1 bg-[#5d87ff]/15 text-[#5d87ff] text-xs rounded hover:bg-[#5d87ff]/25"
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
          <div className="text-[#7c8fac] text-sm italic p-4">No items match the current filter.</div>
        )}
      </div>
    </div>
  );
}
