import { useEffect, useState } from 'react';
import { apiFetch } from '../api';

/**
 * Per-source new-item counts surfaced by GET /admin/inbox/counts.
 * Keep this type in sync with the controller's response shape.
 */
export interface InboxCounts {
  new_bugs: number;
  new_features: number;
  new_errors: number;
  total: number;
}

/**
 * Subscribe to the admin inbox counts. Returns the latest snapshot (or null
 * before the first fetch). Polls the backend on a 60 s cadence — short
 * enough that a brand-new bug/error/feature shows up quickly, light enough
 * that the API isn't hammered.
 *
 * Extracted from Layout so any other surface (a dashboard widget, a
 * Topbar replacement in a future redesign) can render the same badge
 * without duplicating the fetch/poll loop. Per Rule #1 (reusable).
 */
export function useInboxCounts(): InboxCounts | null {
  const [counts, setCounts] = useState<InboxCounts | null>(null);

  useEffect(() => {
    let alive = true;
    const fetchCounts = async () => {
      try {
        const data = await apiFetch('/admin/inbox/counts') as InboxCounts;
        if (alive) setCounts(data);
      } catch {/* silent — last good value stays on screen */}
    };
    fetchCounts();
    const id = window.setInterval(fetchCounts, 60_000);
    return () => { alive = false; window.clearInterval(id); };
  }, []);

  return counts;
}
