# Feedback Public Board (Spec C) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the public feature-request board at `https://draftright.info/feedback` — a card list of feature requests with upvote buttons, status + target-platform filters, and an on-page submit form, all wired to the backend `/feedback` endpoints that shipped in Spec A.

**Architecture:** A new Astro page `website/src/pages/feedback.astro` server-fetches the initial board from `GET /feedback` (no auth) so the page is SEO-friendly and renders with content on first paint. A single React island `FeedbackBoard.tsx`, hydrated with `client:load`, takes over for interactive bits: re-fetching when filters change, toggling upvotes (requires the user's Bearer JWT — `localStorage['dr_access_token']` — same pattern `ReportBugWidget` uses), and submitting new requests via the same form shape `SuggestFeatureWidget` already uses. Logged-out visitors see the board read-only and "Sign in to vote" on each upvote button. No new backend work — Spec A's `GET /feedback`, `POST /feedback/:id/vote`, `POST /feedback` are sufficient.

**Tech Stack:** Astro 5, React 18 (island, `client:load`), Tailwind CSS — matches the existing `website/` setup. No test runner is configured (`package.json` has no jest/vitest); verification is `astro check` + `npm run build` + manual smoke.

**Contract reminders (from Spec A):**
- `GET /feedback?status=<state>&target_platform=<plat>&page=<n>&limit=<n>` (optional JWT → adds `viewerHasVoted` per row; no auth → all `false`). Returns `{ rows: BoardRow[], total }`. `BoardRow` includes `id, title, target_platform, description, status, vote_count, source, created_at, viewerHasVoted`.
- `POST /feedback/:id/vote` **requires JWT**. Returns `{ vote_count, hasVoted }`. Idempotent toggle.
- `POST /feedback` body `{ kind:"feature", title, target_platform, description, source:"web", user_email? }`. Returns `{ id, message }`.

**File responsibility split:**
- `website/src/pages/feedback.astro` — server-rendered shell: SEO metadata, server-fetched initial `GET /feedback`, mounts the React island with the initial data + the resolved API base.
- `website/src/components/FeedbackBoard.tsx` — the React island: card list rendering, vote toggle, filter tabs (status + platform), pagination ("Load more"), in-page submit form (reuses the same input UX as `SuggestFeatureWidget`). Reads JWT from `localStorage['dr_access_token']`.
- `website/src/components/FeedbackBoardCard.tsx` — one card row (title, description excerpt, vote button, status + platform badges, source). Pure presentational, takes a row + `onVote`. Split out because cards grow specific concerns (badges, truncation, voted styling) that don't belong inside the parent's state logic.
- `website/src/layouts/BaseLayout.astro` (or `website/src/components/Nav.astro`) — add a "Feedback" nav link to the global header.

---

### Task 1: `FeedbackBoardCard` — one card, presentational

**Files:**
- Create: `website/src/components/FeedbackBoardCard.tsx`

This task is self-contained — no fetching, no state beyond what the parent passes in.

- [ ] **Step 1: Create `website/src/components/FeedbackBoardCard.tsx`**

```tsx
import type { ReactNode } from 'react';

export interface BoardRow {
  id: string;
  title: string | null;
  description: string;
  target_platform: string | null;
  status: string;
  vote_count: number;
  source: string;
  created_at: string;
  viewerHasVoted: boolean;
}

interface Props {
  row: BoardRow;
  isSignedIn: boolean;
  onVote: (id: string) => void;
}

const PLATFORM_STYLES: Record<string, string> = {
  playground: 'bg-purple-900/40 border-purple-700/60',
  mobile: 'bg-emerald-900/40 border-emerald-700/60',
  windows: 'bg-sky-900/40 border-sky-700/60',
  mac: 'bg-zinc-700/40 border-zinc-500/60',
  linux: 'bg-amber-900/40 border-amber-700/60',
};

const PLATFORM_LABEL: Record<string, string> = {
  playground: 'Playground', mobile: 'Mobile', windows: 'Windows', mac: 'macOS', linux: 'Linux',
};

const STATUS_LABEL: Record<string, string> = {
  new: 'open', open: 'open', reviewing: 'reviewing',
  planned: 'planned', in_progress: 'in progress',
  fix_proposed: 'in progress', resolved: 'done', done: 'done',
  declined: 'declined', wont_fix: 'declined',
};

const STATUS_STYLES: Record<string, string> = {
  open: 'text-slate-400 border-slate-600',
  reviewing: 'text-slate-300 border-slate-500',
  planned: 'text-sky-400 border-sky-500',
  'in progress': 'text-amber-400 border-amber-500',
  done: 'text-emerald-400 border-emerald-500',
  declined: 'text-rose-400 border-rose-500',
};

function statusKey(raw: string): string { return STATUS_LABEL[raw] ?? raw; }

export default function FeedbackBoardCard({ row, isSignedIn, onVote }: Props) {
  const plat = row.target_platform ?? '';
  const platLabel = PLATFORM_LABEL[plat] ?? plat;
  const platStyle = PLATFORM_STYLES[plat] ?? 'bg-zinc-800 border-zinc-700';
  const status = statusKey(row.status);
  const statusStyle = STATUS_STYLES[status] ?? STATUS_STYLES.open;

  const voteTitle = isSignedIn
    ? row.viewerHasVoted ? 'Remove your vote' : 'Upvote this request'
    : 'Sign in to vote';
  const voteClasses = [
    'flex flex-col items-center justify-center min-w-[56px] rounded-lg border px-3 py-2 transition select-none',
    isSignedIn ? 'cursor-pointer' : 'cursor-not-allowed opacity-60',
    row.viewerHasVoted
      ? 'bg-sky-900/40 border-sky-500 text-sky-200'
      : 'bg-zinc-800/60 border-zinc-700 text-zinc-300 hover:border-sky-500',
  ].join(' ');

  return (
    <li className="flex gap-4 rounded-xl border border-zinc-700 bg-zinc-900/60 p-4">
      <button
        type="button"
        aria-label={voteTitle}
        title={voteTitle}
        disabled={!isSignedIn}
        onClick={() => onVote(row.id)}
        className={voteClasses}>
        <span className="text-sm leading-none">▲</span>
        <span className="text-base font-bold tabular-nums">{row.vote_count}</span>
      </button>
      <div className="min-w-0 flex-1">
        <h3 className="text-base font-semibold text-zinc-100">{row.title ?? '(untitled)'}</h3>
        <p className="mt-1 line-clamp-3 text-sm text-zinc-400">{row.description}</p>
        <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
          {plat && (
            <Badge className={`${platStyle} text-zinc-100`}>{platLabel}</Badge>
          )}
          <Badge className={`${statusStyle} bg-transparent`}>{status}</Badge>
          <Badge className="ml-auto bg-transparent border-zinc-700 text-zinc-500">via {row.source}</Badge>
        </div>
      </div>
    </li>
  );
}

function Badge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={`rounded-full border px-2 py-0.5 ${className ?? ''}`}>{children}</span>
  );
}
```

- [ ] **Step 2: Type-check**

Run: `cd website && npx astro check`
Expected: 0 errors. (`astro check` runs TypeScript across `.astro` + `.tsx`. If it isn't installed, `npx tsc --noEmit -p tsconfig.json` works as a fallback — check `tsconfig.json` exists.)

- [ ] **Step 3: Commit**

```bash
git add website/src/components/FeedbackBoardCard.tsx
git commit -m "feat(website): FeedbackBoardCard — one card row for the public feedback board"
```

---

### Task 2: `FeedbackBoard` — the React island

**Files:**
- Create: `website/src/components/FeedbackBoard.tsx`

- [ ] **Step 1: Create `website/src/components/FeedbackBoard.tsx`**

```tsx
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
    // page included so "Load more" triggers a fetch; filter changes always reset page to 1 via setPage(1)
  }, [apiUrl, status, platform, page, token, initialFiltersRef]);

  const changeStatus = (v: string) => { setStatus(v); setPage(1); };
  const changePlatform = (v: string) => { setPlatform(v); setPage(1); };

  const vote = useCallback(async (id: string) => {
    if (!token) return;
    // Optimistic update; rollback on failure.
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
      // Rollback.
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
          // Reset filters to show the new row near the top.
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
```

- [ ] **Step 2: Type-check**

Run: `cd website && npx astro check`
Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add website/src/components/FeedbackBoard.tsx
git commit -m "feat(website): FeedbackBoard React island — card list, vote toggle, filters, submit form"
```

---

### Task 3: `feedback.astro` page (server-fetched initial data)

**Files:**
- Create: `website/src/pages/feedback.astro`

- [ ] **Step 1: Create `website/src/pages/feedback.astro`**

```astro
---
import BaseLayout from '../layouts/BaseLayout.astro';
import FeedbackBoard from '../components/FeedbackBoard';

// API base resolution mirrors the rest of the site (ReportBugWidget etc.).
const apiUrl = import.meta.env.PUBLIC_API_URL || 'https://api.draftright.info';

const statusParam = Astro.url.searchParams.get('status') ?? 'all';
const platformParam = Astro.url.searchParams.get('target_platform') ?? 'all';

const qs = new URLSearchParams({ page: '1', limit: '20' });
if (statusParam !== 'all') qs.set('status', statusParam);
if (platformParam !== 'all') qs.set('target_platform', platformParam);

// Server-fetch the first page so the board renders with content on first
// paint (SEO + perceived speed). If the API is down we still render an
// empty shell — the client island will retry on filter change.
let initial = { rows: [], total: 0 };
try {
  const res = await fetch(`${apiUrl}/feedback?${qs.toString()}`);
  if (res.ok) initial = await res.json();
} catch {
  // Render an empty board; client takes over.
}
---
<BaseLayout title="Feature Requests · DraftRight"
            description="Vote on what we build next. Public feature requests across DraftRight's apps for macOS, Windows, Linux, iOS, Android, and the web.">
  <FeedbackBoard client:load
    apiUrl={apiUrl}
    initial={initial}
    initialStatus={statusParam}
    initialPlatform={platformParam} />
</BaseLayout>
```

NOTE: If `BaseLayout.astro`'s props are `title`/`description` already, the above works. If they differ (e.g. `pageTitle`/`metaDescription`), adapt. Read `website/src/layouts/BaseLayout.astro` once before writing this file.

- [ ] **Step 2: Build**

Run: `cd website && npm run build`
Expected: build succeeds; `dist/feedback/index.html` exists.

- [ ] **Step 3: Commit**

```bash
git add website/src/pages/feedback.astro
git commit -m "feat(website): /feedback page — server-fetched initial board + FeedbackBoard island"
```

---

### Task 4: Nav link to `/feedback`

**Files:**
- Modify: `website/src/components/Nav.astro` (or `website/src/layouts/BaseLayout.astro` if the nav lives there)

- [ ] **Step 1: Read** `website/src/components/Nav.astro`. Find the existing `<a>` links (Pricing, Download, etc.). Note the exact Tailwind classes / structure used per link.

- [ ] **Step 2: Add the link** in the same nav `<ul>`/`<div>`, near "Pricing" / before the sign-in/account block. Match the surrounding link markup exactly. Example pattern (adjust classes to whatever the existing links use):

```astro
<a href="/feedback" class="<same classes the other nav links use>">Feedback</a>
```

- [ ] **Step 3: Build**

Run: `cd website && npm run build`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add website/src/components/Nav.astro
git commit -m "feat(website): nav link to /feedback"
```

---

### Task 5: Manual smoke + docs

**Files:**
- Modify: `docs/changelog.md` (prepend a `## 2026-05-13`-or-current section bullet)
- Modify: `website/CLAUDE.md` (document the new page; match existing module-list format)

- [ ] **Step 1: Local smoke**

In one terminal: `cd backend && npm run start:dev` (NODE_ENV not production → synchronize; backend serves on `:3000`).
In another: `cd website && PUBLIC_API_URL=http://localhost:3000 npm run dev` (default port 4000 or 4321 — whatever `astro.config.mjs` says).
Open `http://localhost:<port>/feedback`. Expect: feature requests render (those created in Spec A's smoke test are visible). Click a status tab → the list re-filters. Click `+ Suggest a feature` → submit → the new row appears at the top. Open the page logged-out → upvote buttons show "Sign in to vote" tooltip and are non-clickable. Open in a tab where `localStorage['dr_access_token']` is set (or sign in via the existing flow) → click an upvote → count increments; click again → decrements. (Vote requires a real `user_id` FK in `feature_votes` — sign in as a real user; can't fake it.)

If the local backend has no rows: `curl -X POST http://localhost:3000/feedback -H 'Content-Type: application/json' -d '{"kind":"feature","title":"Sample","target_platform":"mac","description":"hello","source":"web"}'` to seed.

- [ ] **Step 2: Changelog bullet**

Add under the current date section:

```markdown
### Feedback public board (Spec C)
- New page `draftright.info/feedback` — card list of feature requests sorted by votes, status + target-platform filters, "Load more" pagination, inline "+ Suggest a feature" form. Server-fetches the initial page for SEO; React island (`FeedbackBoard`) handles re-fetch, optimistic upvotes (JWT required, `dr_access_token`), and submit. Logged-out visitors see read-only board + "Sign in to vote" tooltip on the upvote buttons.
- Nav link added (`Feedback`); all client "See all requests →" deep-links (Spec B) now land on a real page.
```

- [ ] **Step 3: `website/CLAUDE.md` entry**

Add a one-liner matching the existing entries, e.g. under "Pages":

```markdown
- `feedback.astro` — public feature-request board (server-fetches `GET /feedback`, hydrates `FeedbackBoard` island). Voting requires the logged-in user's JWT (`dr_access_token`). Submit form posts to `POST /feedback`. Spec C of the feedback feature.
```

- [ ] **Step 4: Final build + commit**

Run: `cd website && npm run build`
Expected: success.

```bash
git add docs/changelog.md website/CLAUDE.md
git commit -m "docs(feedback): Spec C — public board page + nav link"
```

---

## Self-review notes

- **Spec coverage:** card list with vote button + status + target_platform badges (Q2 + Q3 — public board with upvotes), status filter tabs + platform filter (design spec § "Website"), inline submit form (design spec — board page has the on-page form too), logged-out read-only with "Sign in to vote" (Q4 — login required to vote), nav link, server-fetched initial render for SEO. The deep-link from Spec B's clients (`https://draftright.info/feedback`) now resolves to a real page.
- **No tests** — `website/` has no jest/vitest configured. Verification is `astro check` + `npm run build` + manual smoke (Tasks 1-2 explicitly include `astro check`).
- **No backend changes** — Spec A's `GET /feedback`, `POST /feedback/:id/vote`, `POST /feedback` are sufficient.
- **Pagination:** keep-it-simple "Load more" rather than page numbers; concatenates pages into the same list. Total comes back with every response.
- **Optimistic vote** + rollback on failure matches the rest of the site's UX feel (no spinner per click).

## After this plan

Feedback feature complete end-to-end (Specs A + B + C). Deploy order: `develop` → testing, apply `backend/sql/2026-05-12-feedback.sql` to prod Postgres + redeploy backend container + redeploy website; then `develop` → `main`; ship each native client in its next release train. Open issues to track: vote endpoint requires a real `users` row (anonymous vote dedupe via IP/cookie is a future option, not in scope).
