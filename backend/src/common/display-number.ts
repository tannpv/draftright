/**
 * Format a numeric `display_no` into the user-facing short identifier used
 * in admin UI, snackbars, and email confirmations.
 *
 * Reports are spread across two tables (bug_reports also stores feature
 * requests, distinguished by `kind`), so the prefix is a small enum the
 * caller picks. Keeping the formatter in one place means every surface
 * shows the same prefix style (no admin saying "BUG #123" while the API
 * returns "B-123").
 *
 * Used by:
 *   - BugReportsController response shaping (BUG / FR)
 *   - ErrorsController response shaping (ERR)
 *   - Admin UI list / detail views
 */
export type ReportKind = 'bug' | 'feature' | 'error';

const PREFIXES: Record<ReportKind, string> = {
  bug: 'BUG',
  feature: 'FR',
  error: 'ERR',
};

export function formatDisplayNumber(kind: ReportKind, displayNo: number | string | null | undefined): string | null {
  if (displayNo == null) return null;
  const n = typeof displayNo === 'string' ? Number.parseInt(displayNo, 10) : displayNo;
  if (!Number.isFinite(n)) return null;
  return `${PREFIXES[kind]}-${n}`;
}
