import { useState, useEffect, useCallback } from 'react';
import { apiFetch, SEARCH_DEBOUNCE_MS, API_URL, DEFAULT_PAGE_SIZE } from '../api';
import DataTable from '../components/DataTable';
import Toast from '../components/Toast';
import { timeAgo } from '../lib/format';
import { toneStyle, type Tone } from '../lib/status';

/* ── Types ────────────────────────────────────────────── */

interface BugReport {
  id: string;
  source: string;
  description: string;
  screenshot_path: string | null;
  screenshot_filename: string | null;
  app_version: string | null;
  os_info: string | null;
  user_id: string | null;
  user_email: string | null;
  context: Record<string, unknown> | null;
  status: string;            // new | reviewing | resolved | wont_fix
  admin_notes: string | null;
  created_at: string;
  updated_at: string;
}

interface BugReportsResponse {
  rows?: BugReport[];
  items?: BugReport[];
  bug_reports?: BugReport[];
  total: number;
}

/* ── Helpers ──────────────────────────────────────────── */

function sourceIcon(source: string): string {
  const s = (source || '').toLowerCase();
  if (s.includes('macos'))    return '🍎';   // 🍎
  if (s.includes('ios'))      return '📱';   // 📱
  if (s.includes('android'))  return '🤖';   // 🤖
  if (s.includes('windows'))  return '🖥️'; // 🖥
  if (s.includes('linux'))    return '🐧';   // 🐧
  // web / admin-portal / marketing-site / web-playground
  return '🌐'; // 🌐
}

const BUG_STATUS: Record<string, { tone: Tone; label: string }> = {
  new:       { tone: 'primary', label: 'New' },
  reviewing: { tone: 'warning', label: 'Reviewing' },
  resolved:  { tone: 'success', label: 'Resolved' },
  wont_fix:  { tone: 'muted',   label: "Won't fix" },
};
function statusStyle(status: string): { color: string; bg: string; label: string } {
  const e = BUG_STATUS[status];
  return e ? { ...toneStyle(e.tone), label: e.label } : { ...toneStyle('muted'), label: status || '—' };
}

function truncate(text: string, max: number): string {
  if (!text) return '';
  return text.length > max ? text.slice(0, max).trimEnd() + '…' : text;
}

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: 'new',       label: 'New' },
  { value: 'reviewing', label: 'Reviewing' },
  { value: 'resolved',  label: 'Resolved' },
  { value: 'wont_fix',  label: "Won't fix" },
];

/* ── Filter tabs ──────────────────────────────────────── */

type StatusFilter = 'all' | 'new' | 'reviewing' | 'resolved' | 'wont_fix';

const FILTER_TABS: { key: StatusFilter; label: string }[] = [
  { key: 'all',       label: 'All' },
  { key: 'new',       label: 'New' },
  { key: 'reviewing', label: 'Reviewing' },
  { key: 'resolved',  label: 'Resolved' },
  { key: 'wont_fix',  label: "Won't fix" },
];

/* ── Screenshot loader (fetch with JWT, return blob URL) ── */


async function loadScreenshot(id: string): Promise<string> {
  const token = localStorage.getItem('token');
  const res = await fetch(`${API_URL}/admin/bug-reports/${id}/screenshot`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new Error(`Failed to load screenshot (${res.status})`);
  const blob = await res.blob();
  return URL.createObjectURL(blob);
}

/* ── Component ────────────────────────────────────────── */

export default function BugReportsPage() {
  const [reports, setReports] = useState<BugReport[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [sortBy, setSortBy] = useState<string>('created_at');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Detail modal state
  const [selected, setSelected] = useState<BugReport | null>(null);
  const [screenshotUrl, setScreenshotUrl] = useState<string | null>(null);
  const [screenshotError, setScreenshotError] = useState<string | null>(null);
  const [notesDraft, setNotesDraft] = useState('');
  const [savingNotes, setSavingNotes] = useState(false);
  const [savingStatus, setSavingStatus] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  /* ── Fetch list ───────────────────────────────────── */
  const fetchReports = useCallback(async (status: StatusFilter, p: number, limit: number) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(p), limit: String(limit),
        sort_by: sortBy, sort_order: sortOrder,
      });
      if (status !== 'all') params.set('status', status);
      if (search) params.set('search', search);
      const data = await apiFetch(`/admin/bug-reports?${params.toString()}`) as BugReportsResponse;
      const list = data.rows ?? data.items ?? data.bug_reports ?? [];
      setReports(list);
      setTotal(data.total ?? list.length);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load bug reports');
      setReports([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [search, sortBy, sortOrder]);

  useEffect(() => {
    fetchReports(statusFilter, page, pageSize);
  }, [fetchReports, statusFilter, page, pageSize]);

  // Debounce search input.
  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [searchInput]);

  /* ── Open detail modal ────────────────────────────── */
  function openDetail(row: BugReport) {
    setSelected(row);
    setNotesDraft(row.admin_notes ?? '');
    setScreenshotUrl(null);
    setScreenshotError(null);
    setConfirmDelete(false);

    if (row.screenshot_path || row.screenshot_filename) {
      loadScreenshot(row.id)
        .then((url) => setScreenshotUrl(url))
        .catch((err) => setScreenshotError(err instanceof Error ? err.message : 'Failed to load screenshot'));
    }
  }

  function closeDetail() {
    if (screenshotUrl) URL.revokeObjectURL(screenshotUrl);
    setScreenshotUrl(null);
    setScreenshotError(null);
    setSelected(null);
    setConfirmDelete(false);
  }

  // Cleanup blob URL on unmount.
  useEffect(() => {
    return () => {
      if (screenshotUrl) URL.revokeObjectURL(screenshotUrl);
    };
  }, [screenshotUrl]);

  /* ── Update status ────────────────────────────────── */
  async function handleStatusChange(newStatus: string) {
    if (!selected) return;
    setSavingStatus(true);
    try {
      await apiFetch(`/admin/bug-reports/${selected.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ status: newStatus }),
      });
      setSelected({ ...selected, status: newStatus });
      setToast({ message: `Status updated to ${newStatus}.`, type: 'success' });
      fetchReports(statusFilter, page, pageSize);
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to update status', type: 'error' });
    } finally {
      setSavingStatus(false);
    }
  }

  /* ── Save notes (on blur) ─────────────────────────── */
  async function handleNotesSave() {
    if (!selected) return;
    if ((selected.admin_notes ?? '') === notesDraft) return; // no change
    setSavingNotes(true);
    try {
      await apiFetch(`/admin/bug-reports/${selected.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ admin_notes: notesDraft }),
      });
      setSelected({ ...selected, admin_notes: notesDraft });
      setToast({ message: 'Notes saved.', type: 'success' });
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to save notes', type: 'error' });
    } finally {
      setSavingNotes(false);
    }
  }

  /* ── Delete ───────────────────────────────────────── */
  async function handleDelete() {
    if (!selected) return;
    setDeleting(true);
    try {
      await apiFetch(`/admin/bug-reports/${selected.id}`, { method: 'DELETE' });
      setToast({ message: 'Bug report deleted.', type: 'success' });
      closeDetail();
      fetchReports(statusFilter, page, pageSize);
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to delete', type: 'error' });
    } finally {
      setDeleting(false);
    }
  }

  function handleFilterChange(f: StatusFilter) {
    setStatusFilter(f);
    setPage(1);
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  /* ── Table columns ────────────────────────────────── */
  const columns = [
    {
      header: 'Source',
      key: 'source',
      sortKey: 'source',
      render: (row: BugReport) => (
        <span style={{ color: 'var(--text)', fontSize: 13, whiteSpace: 'nowrap' }}>
          <span style={{ marginRight: 6 }}>{sourceIcon(row.source)}</span>
          {row.source}
        </span>
      ),
    },
    {
      header: 'User',
      key: 'user_email',
      render: (row: BugReport) => (
        <span style={{ color: row.user_email ? 'var(--text)' : 'var(--muted)', fontSize: 13 }}>
          {row.user_email || 'anonymous'}
        </span>
      ),
    },
    {
      header: 'Description',
      key: 'description',
      render: (row: BugReport) => (
        <span
          style={{ color: 'var(--text)', fontSize: 13, whiteSpace: 'normal', display: 'block', maxWidth: 480 }}
          title={row.description}
        >
          {truncate(row.description, 80)}
        </span>
      ),
    },
    {
      header: 'Status',
      key: 'status',
      sortKey: 'status',
      render: (row: BugReport) => {
        const s = statusStyle(row.status);
        return (
          <span
            style={{
              display: 'inline-block',
              padding: '3px 10px',
              borderRadius: 4,
              fontSize: 12,
              fontWeight: 600,
              background: s.bg,
              color: s.color,
              whiteSpace: 'nowrap',
            }}
          >
            {s.label}
          </span>
        );
      },
    },
    {
      header: 'Screenshot',
      key: 'screenshot',
      render: (row: BugReport) => (
        <span style={{ fontSize: 14 }} title={row.screenshot_filename || ''}>
          {(row.screenshot_path || row.screenshot_filename) ? '📎' : ''}
        </span>
      ),
    },
    {
      header: 'Created',
      key: 'created_at',
      sortKey: 'created_at',
      render: (row: BugReport) => (
        <span style={{ color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }} title={new Date(row.created_at).toLocaleString()}>
          {timeAgo(row.created_at)}
        </span>
      ),
    },
    {
      header: 'Actions',
      key: 'actions',
      render: (row: BugReport) => (
        <div style={{ display: 'flex', gap: 6 }}>
          <button
            className="btn btn-sm"
            onClick={(e) => {
              e.stopPropagation();
              openDetail(row);
            }}
            style={{
              background: 'rgba(93,135,255,0.1)',
              color: 'var(--primary)',
              border: '1px solid rgba(93,135,255,0.2)',
              padding: '5px 12px',
              borderRadius: 6,
              fontSize: 12,
              fontFamily: 'inherit',
              cursor: 'pointer',
            }}
          >
            View
          </button>
        </div>
      ),
    },
  ];

  /* ── Render ────────────────────────────────────────── */
  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Bug Reports</h1>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
          User-submitted bug reports from every DraftRight surface
        </p>
      </div>

      {/* Toolbar — search + filter tabs */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 20, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="text"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder="Search by description, email, source..."
          style={{
            flex: '1 1 280px', maxWidth: 360,
            padding: '8px 14px 8px 36px',
            borderRadius: 7, border: '1px solid var(--border)', background: 'var(--bg)',
            color: 'var(--text)', fontSize: 13, fontFamily: 'inherit', outline: 'none',
            backgroundImage: "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%237c8fac' stroke-width='2'><circle cx='11' cy='11' r='8'/><path d='M21 21l-4.35-4.35'/></svg>\")",
            backgroundRepeat: 'no-repeat', backgroundPosition: '12px center',
          }}
        />
        <div style={{ display: 'flex', gap: 4 }}>
          {FILTER_TABS.map((tab) => (
            <button
              key={tab.key}
              onClick={() => handleFilterChange(tab.key)}
              style={{
                padding: '7px 18px',
                borderRadius: 7,
                fontSize: 13,
                fontWeight: 600,
                fontFamily: 'inherit',
                border: 'none',
                cursor: 'pointer',
                transition: 'all 0.15s',
                background: statusFilter === tab.key ? 'rgba(93,135,255,0.15)' : 'transparent',
                color: statusFilter === tab.key ? 'var(--primary)' : 'var(--muted)',
              }}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 20, padding: 12, borderRadius: 7, background: 'rgba(250,137,107,0.1)', border: '1px solid rgba(250,137,107,0.3)', color: 'var(--danger)', fontSize: 13 }}>{error}</div>}

      <DataTable
        columns={columns}
        rows={reports}
        onRowClick={openDetail}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
        total={total}
        pageSize={pageSize}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        sortBy={sortBy}
        sortOrder={sortOrder}
        onSortChange={(by, order) => { setSortBy(by); setSortOrder(order); setPage(1); }}
        loading={loading}
        emptyMessage={search ? `No matches for "${search}".` : 'No bug reports yet.'}
      />

      {/* ── Detail Modal ──────────────────────────────── */}
      {selected && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.55)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            padding: 24,
            overflow: 'auto',
          }}
          onClick={() => { if (!deleting && !savingNotes && !savingStatus) closeDetail(); }}
        >
          <div
            style={{
              background: 'var(--card)',
              borderRadius: 10,
              padding: 28,
              width: '100%',
              maxWidth: 720,
              maxHeight: 'calc(100vh - 48px)',
              overflowY: 'auto',
              boxShadow: '0 12px 40px rgba(0,0,0,0.4)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 18, gap: 16 }}>
              <div>
                <h2 style={{ color: 'var(--text)', fontSize: 18, fontWeight: 700, margin: '0 0 4px' }}>
                  Bug Report
                </h2>
                <p style={{ color: 'var(--muted)', fontSize: 12, margin: 0, fontFamily: 'monospace' }}>
                  {selected.id}
                </p>
              </div>
              <button
                onClick={closeDetail}
                style={{
                  background: 'transparent', border: 'none', color: 'var(--muted)',
                  fontSize: 24, lineHeight: 1, cursor: 'pointer', padding: 0, fontFamily: 'inherit',
                }}
                title="Close"
              >
                ×
              </button>
            </div>

            {/* Description */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Description
              </label>
              <div
                style={{
                  background: 'var(--bg)',
                  border: '1px solid var(--border)',
                  borderRadius: 7,
                  padding: 12,
                  color: 'var(--text)',
                  fontSize: 14,
                  lineHeight: 1.5,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}
              >
                {selected.description}
              </div>
            </div>

            {/* Metadata grid */}
            <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '8px 16px', fontSize: 13, marginBottom: 20 }}>
              <span style={{ color: 'var(--muted)' }}>Reporter:</span>
              <span style={{ color: selected.user_email ? 'var(--text)' : 'var(--muted)' }}>
                {selected.user_email || 'anonymous'}
                {selected.user_id && (
                  <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11, marginLeft: 8 }}>
                    ({selected.user_id})
                  </span>
                )}
              </span>

              <span style={{ color: 'var(--muted)' }}>Source:</span>
              <span style={{ color: 'var(--text)' }}>
                {sourceIcon(selected.source)} {selected.source}
              </span>

              <span style={{ color: 'var(--muted)' }}>App version:</span>
              <span style={{ color: 'var(--text)', fontFamily: 'monospace', fontSize: 12 }}>
                {selected.app_version || '—'}
              </span>

              <span style={{ color: 'var(--muted)' }}>OS info:</span>
              <span style={{ color: 'var(--text)' }}>{selected.os_info || '—'}</span>

              <span style={{ color: 'var(--muted)' }}>Created:</span>
              <span style={{ color: 'var(--text)' }}>
                {new Date(selected.created_at).toLocaleString()} <span style={{ color: 'var(--muted)' }}>({timeAgo(selected.created_at)})</span>
              </span>
            </div>

            {/* Context JSON */}
            {selected.context && Object.keys(selected.context).length > 0 && (
              <div style={{ marginBottom: 20 }}>
                <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Context
                </label>
                <pre
                  style={{
                    background: 'var(--bg)',
                    border: '1px solid var(--border)',
                    borderRadius: 7,
                    padding: 12,
                    color: 'var(--text)',
                    fontSize: 12,
                    lineHeight: 1.5,
                    fontFamily: 'monospace',
                    overflowX: 'auto',
                    margin: 0,
                  }}
                >
                  {JSON.stringify(selected.context, null, 2)}
                </pre>
              </div>
            )}

            {/* Screenshot */}
            {(selected.screenshot_path || selected.screenshot_filename) && (
              <div style={{ marginBottom: 20 }}>
                <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Screenshot / Attachment
                </label>
                {screenshotError ? (
                  <div style={{ background: 'rgba(250,137,107,0.1)', border: '1px solid rgba(250,137,107,0.3)', borderRadius: 7, padding: 12, color: 'var(--danger)', fontSize: 13 }}>
                    {screenshotError}
                  </div>
                ) : screenshotUrl ? (
                  <div style={{ border: '1px solid var(--border)', borderRadius: 7, background: 'var(--bg)', overflow: 'hidden' }}>
                    <a
                      href={screenshotUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ display: 'block', cursor: 'zoom-in' }}
                      title="Click to open full size in new tab"
                    >
                      <img
                        src={screenshotUrl}
                        alt={selected.screenshot_filename || 'Bug screenshot'}
                        style={{
                          maxWidth: '100%',
                          maxHeight: 500,
                          display: 'block',
                          margin: '0 auto',
                        }}
                      />
                    </a>
                    <div style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      gap: 12,
                      padding: '8px 12px',
                      borderTop: '1px solid var(--border)',
                      background: 'var(--card)',
                      fontSize: 12,
                      color: 'var(--muted)',
                    }}>
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }} title={selected.screenshot_filename || ''}>
                        📎 {selected.screenshot_filename || 'screenshot'}
                      </span>
                      <div style={{ display: 'flex', gap: 8, flexShrink: 0 }}>
                        <a
                          href={screenshotUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          style={{
                            padding: '4px 10px',
                            borderRadius: 5,
                            background: 'rgba(73,190,255,0.15)',
                            color: 'var(--secondary)',
                            fontSize: 12,
                            textDecoration: 'none',
                          }}
                        >
                          Open
                        </a>
                        <a
                          href={screenshotUrl}
                          download={selected.screenshot_filename || 'screenshot'}
                          style={{
                            padding: '4px 10px',
                            borderRadius: 5,
                            background: 'rgba(93,135,255,0.15)',
                            color: 'var(--primary)',
                            fontSize: 12,
                            textDecoration: 'none',
                          }}
                        >
                          Download
                        </a>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div style={{ color: 'var(--muted)', fontSize: 13, fontStyle: 'italic' }}>Loading screenshot…</div>
                )}
              </div>
            )}

            {/* Status dropdown */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Status
              </label>
              <select
                className="dark-input"
                value={selected.status}
                disabled={savingStatus}
                onChange={(e) => handleStatusChange(e.target.value)}
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  borderRadius: 7,
                  border: '1px solid var(--border)',
                  background: 'var(--bg)',
                  color: 'var(--text)',
                  fontSize: 13,
                  fontFamily: 'inherit',
                  cursor: savingStatus ? 'wait' : 'pointer',
                }}
              >
                {STATUS_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
                {/* Allow showing unknown status that came from server. */}
                {!STATUS_OPTIONS.find((o) => o.value === selected.status) && (
                  <option value={selected.status}>{selected.status}</option>
                )}
              </select>
            </div>

            {/* Admin notes */}
            <div style={{ marginBottom: 24 }}>
              <label style={{ display: 'block', color: 'var(--muted)', fontSize: 12, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Admin notes {savingNotes && <span style={{ color: 'var(--primary)', textTransform: 'none', fontWeight: 400 }}> · saving…</span>}
              </label>
              <textarea
                className="dark-input"
                value={notesDraft}
                onChange={(e) => setNotesDraft(e.target.value)}
                onBlur={handleNotesSave}
                rows={4}
                placeholder="Internal notes (saved on blur)…"
                style={{
                  width: '100%',
                  padding: 12,
                  borderRadius: 7,
                  border: '1px solid var(--border)',
                  background: 'var(--bg)',
                  color: 'var(--text)',
                  fontSize: 13,
                  fontFamily: 'inherit',
                  resize: 'vertical',
                  outline: 'none',
                  boxSizing: 'border-box',
                }}
              />
            </div>

            {/* Footer — delete button */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderTop: '1px solid var(--border)', paddingTop: 18 }}>
              {confirmDelete ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <span style={{ color: 'var(--danger)', fontSize: 13, fontWeight: 600 }}>Delete this report?</span>
                  <button
                    onClick={handleDelete}
                    disabled={deleting}
                    style={{
                      padding: '7px 16px',
                      background: 'var(--danger)',
                      color: '#fff',
                      border: 'none',
                      borderRadius: 7,
                      fontSize: 13,
                      fontWeight: 600,
                      fontFamily: 'inherit',
                      cursor: deleting ? 'wait' : 'pointer',
                    }}
                  >
                    {deleting ? 'Deleting…' : 'Yes, delete'}
                  </button>
                  <button
                    onClick={() => setConfirmDelete(false)}
                    disabled={deleting}
                    style={{
                      padding: '7px 16px',
                      background: 'transparent',
                      color: 'var(--muted)',
                      border: '1px solid var(--border)',
                      borderRadius: 7,
                      fontSize: 13,
                      fontFamily: 'inherit',
                      cursor: 'pointer',
                    }}
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => setConfirmDelete(true)}
                  style={{
                    padding: '8px 18px',
                    background: 'transparent',
                    color: 'var(--danger)',
                    border: '1px solid rgba(250,137,107,0.3)',
                    borderRadius: 7,
                    fontSize: 13,
                    fontWeight: 600,
                    fontFamily: 'inherit',
                    cursor: 'pointer',
                  }}
                >
                  Delete report
                </button>
              )}

              <button
                onClick={closeDetail}
                style={{
                  padding: '8px 18px',
                  background: 'transparent',
                  color: 'var(--muted)',
                  border: '1px solid var(--border)',
                  borderRadius: 7,
                  fontSize: 13,
                  fontFamily: 'inherit',
                  cursor: 'pointer',
                }}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
}
