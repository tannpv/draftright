import { useCallback, useEffect, useMemo, useState } from 'react';
import FeedbackBoardCard, { type BoardRow } from './FeedbackBoardCard';

interface BoardPayload { rows: BoardRow[]; total: number }

interface Props {
  apiUrl: string;
  initial: BoardPayload;
  initialStatus: string;
  initialPlatform: string;
}

const STATUS_TABS: Array<{ value: string; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'new', label: 'Open' },
  { value: 'planned', label: 'Planned' },
  { value: 'fix_proposed', label: 'In progress' },
  { value: 'resolved', label: 'Done' },
];

const PLATFORM_OPTIONS: Array<{ value: string; label: string }> = [
  { value: 'all', label: 'All platforms' },
  { value: 'playground', label: 'Playground (web)' },
  { value: 'mobile', label: 'Mobile (iOS / Android)' },
  { value: 'windows', label: 'Windows' },
  { value: 'mac', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
];

const PAGE_SIZE = 20;

function readToken(): string | null {
  try { return localStorage.getItem('dr_access_token'); } catch { return null; }
}

function buildUrl(api: string, status: string, platform: string, page: number) {
  const q = new URLSearchParams({ page: String(page), limit: String(PAGE_SIZE) });
  if (status !== 'all') q.set('status', status);
  if (platform !== 'all') q.set('target_platform', platform);
  return `${api}/feedback?${q.toString()}`;
}

/**
 * Public board for feature requests. Re-fetches when filters change,
 * toggles upvotes against POST /feedback/:id/vote (JWT required), and
 * lets signed-in users (and anonymous-with-email) submit new requests.
 *
 * The initial page is server-fetched in feedback.astro so the page has
 * content on first paint; this component takes over for interactivity.
 */
export default function FeedbackBoard({ apiUrl, initial, initialStatus, initialPlatform }: Props) {
  const [rows, setRows] = useState<BoardRow[]>(initial.rows);
  const [total, setTotal] = useState(initial.total);
  const [status, setStatus] = useState(initialStatus);
  const [platform, setPlatform] = useState(initialPlatform);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);

  useEffect(() => { setToken(readToken()); }, []);

  // Re-fetch from page 1 when filters change. Skip the very first render
  // for the initial-server-fetched values (we already have those rows).
  const initialFiltersRef = useMemo(
    () => `${initialStatus}|${initialPlatform}`, [initialStatus, initialPlatform],
  );
  useEffect(() => {
    const key = `${status}|${platform}`;
    if (key === initialFiltersRef && page === 1) return;
    let cancelled = false;
    setLoading(true); setLoadError(null);
    (async () => {
      try {
        const headers: Record<string, string> = {};
        if (token) headers['Authorization'] = `Bearer ${token}`;
        const res = await fetch(buildUrl(apiUrl, status, platform, page), { headers });
        if (!res.ok) throw new Error(`server returned ${res.status}`);
        const data = (await res.json()) as BoardPayload;
        if (cancelled) return;
        setRows(prev => (page === 1 ? data.rows : [...prev, ...data.rows]));
        setTotal(data.total);
      } catch (err) {
        if (!cancelled) setLoadError(err instanceof Error ? err.message : 'fetch failed');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [apiUrl, status, platform, page, token, initialFiltersRef]);

  const changeStatus = (v: string) => { setStatus(v); setPage(1); };
  const changePlatform = (v: string) => { setPlatform(v); setPage(1); };

  const vote = useCallback(async (id: string) => {
    if (!token) return;
    setRows(prev => prev.map(r => r.id === id
      ? { ...r, viewerHasVoted: !r.viewerHasVoted,
          vote_count: r.vote_count + (r.viewerHasVoted ? -1 : 1) }
      : r));
    try {
      const res = await fetch(`${apiUrl}/feedback/${id}/vote`, {
        method: 'POST', headers: { 'Authorization': `Bearer ${token}` },
      });
      if (!res.ok) throw new Error('vote failed');
      const data = (await res.json()) as { vote_count: number; hasVoted: boolean };
      setRows(prev => prev.map(r => r.id === id
        ? { ...r, vote_count: data.vote_count, viewerHasVoted: data.hasVoted } : r));
    } catch {
      setRows(prev => prev.map(r => r.id === id
        ? { ...r, viewerHasVoted: !r.viewerHasVoted,
            vote_count: r.vote_count + (r.viewerHasVoted ? -1 : 1) }
        : r));
    }
  }, [apiUrl, token]);

  const canLoadMore = rows.length < total && !loading;

  return (
    <div className="mx-auto max-w-4xl px-5 py-10 text-zinc-100">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">Feature Requests</h1>
          <p className="mt-1 text-sm text-zinc-400">Vote on what we build next.</p>
        </div>
        <SubmitFeatureForm apiUrl={apiUrl} hasToken={Boolean(token)} onCreated={() => {
          setStatus('all'); setPlatform('all'); setPage(1);
        }} />
      </header>

      <div className="mb-5 flex flex-wrap items-center gap-2">
        <div className="flex gap-1 rounded-xl border border-zinc-700 bg-zinc-900/60 p-1">
          {STATUS_TABS.map(t => (
            <button key={t.value} type="button" onClick={() => changeStatus(t.value)}
              className={`rounded-lg px-3 py-1.5 text-sm transition ${status === t.value
                ? 'bg-sky-600 text-white' : 'text-zinc-400 hover:text-zinc-200'}`}>
              {t.label}
            </button>
          ))}
        </div>
        <select value={platform} onChange={(e) => changePlatform(e.target.value)}
          className="ml-auto rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-sm text-zinc-200">
          {PLATFORM_OPTIONS.map(p => <option key={p.value} value={p.value}>{p.label}</option>)}
        </select>
      </div>

      {loadError && (
        <p className="mb-3 text-sm text-rose-400">Couldn't load — {loadError}</p>
      )}

      <ul className="flex flex-col gap-3">
        {rows.map(r => <FeedbackBoardCard key={r.id} row={r} isSignedIn={Boolean(token)} onVote={vote} />)}
      </ul>

      {rows.length === 0 && !loading && (
        <p className="mt-10 text-center text-sm text-zinc-500">No requests match these filters yet.</p>
      )}

      <div className="mt-6 flex justify-center">
        {canLoadMore && (
          <button type="button" onClick={() => setPage(p => p + 1)}
            className="rounded-lg border border-zinc-700 px-4 py-2 text-sm text-zinc-300 hover:border-sky-500">
            Load more ({total - rows.length} remaining)
          </button>
        )}
        {loading && <span className="text-sm text-zinc-500">Loading…</span>}
      </div>
    </div>
  );
}

interface SubmitProps { apiUrl: string; hasToken: boolean; onCreated: () => void }

/** Inline "Suggest a feature" form rendered on the board page. */
function SubmitFeatureForm({ apiUrl, hasToken, onCreated }: SubmitProps) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState('');
  const [platform, setPlatform] = useState('playground');
  const [description, setDescription] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSubmit = title.trim() && description.trim() && !busy;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true); setError(null);
    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      const token = readToken();
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const body: Record<string, unknown> = {
        kind: 'feature', title: title.trim(), target_platform: platform,
        description: description.trim(), source: 'web',
      };
      if (!token && email.trim()) body.user_email = email.trim();
      const res = await fetch(`${apiUrl}/feedback`, {
        method: 'POST', headers, body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`server returned ${res.status}`);
      setOpen(false); setTitle(''); setDescription(''); setEmail(''); setPlatform('playground');
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'something went wrong');
    } finally { setBusy(false); }
  }

  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)}
        className="rounded-lg bg-sky-600 px-4 py-2 text-sm font-semibold text-white hover:bg-sky-500">
        + Suggest a feature
      </button>
    );
  }

  return (
    <form onSubmit={submit}
      className="w-full max-w-md rounded-xl border border-zinc-700 bg-zinc-900/80 p-4 text-sm">
      <h3 className="mb-2 text-base font-semibold">Suggest a feature</h3>
      <label className="text-xs text-zinc-400">Title</label>
      <input value={title} maxLength={80} onChange={(e) => setTitle(e.target.value)}
        placeholder="One line — what should we build?"
        className="mb-3 mt-1 w-full rounded-md border border-zinc-700 bg-zinc-800 px-3 py-2" />
      <label className="text-xs text-zinc-400">Which platform is this for?</label>
      <select value={platform} onChange={(e) => setPlatform(e.target.value)}
        className="mb-3 mt-1 w-full rounded-md border border-zinc-700 bg-zinc-800 px-3 py-2">
        {PLATFORM_OPTIONS.filter(p => p.value !== 'all').map(p =>
          <option key={p.value} value={p.value}>{p.label}</option>)}
      </select>
      <label className="text-xs text-zinc-400">Details</label>
      <textarea value={description} maxLength={2000} onChange={(e) => setDescription(e.target.value)}
        placeholder="What problem does it solve?"
        className="mb-3 mt-1 min-h-[80px] w-full rounded-md border border-zinc-700 bg-zinc-800 px-3 py-2" />
      {!hasToken && (
        <>
          <label className="text-xs text-zinc-400">Email (optional)</label>
          <input value={email} type="email" onChange={(e) => setEmail(e.target.value)}
            className="mb-3 mt-1 w-full rounded-md border border-zinc-700 bg-zinc-800 px-3 py-2" />
        </>
      )}
      {error && <p className="mb-2 text-xs text-rose-400">{error}</p>}
      <div className="flex justify-end gap-2">
        <button type="button" onClick={() => setOpen(false)}
          className="rounded-md border border-zinc-700 px-3 py-1.5 text-zinc-400">Cancel</button>
        <button type="submit" disabled={!canSubmit}
          className={`rounded-md px-4 py-1.5 font-semibold ${canSubmit ? 'bg-emerald-500 text-emerald-950' : 'bg-zinc-800 text-zinc-500'}`}>
          {busy ? 'Submitting…' : 'Submit'}
        </button>
      </div>
    </form>
  );
}
