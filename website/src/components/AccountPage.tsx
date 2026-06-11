import { useEffect, useState } from 'react';

import { API_URL as API } from '../lib/api';

interface Account {
  id: string;
  email: string;
  name: string;
  email_verified: boolean;
  has_lemonsqueezy_customer: boolean;
  subscription: {
    plan_name: string;
    status: string;
    store_type: string;
    started_at: string;
    expires_at: string | null;
    daily_limit: number;
    usage_today: number;
  } | null;
}

interface ExtensionToken {
  id: string;
  device_id: string;
  device_name: string;
  scopes: string[];
  last_used_at: string | null;
  created_at: string;
  revoked_at: string | null;
}

function formatRelative(iso: string): string {
  const then = new Date(iso).getTime();
  const diffMs = Date.now() - then;
  if (diffMs < 60_000) return 'just now';
  const mins = Math.floor(diffMs / 60_000);
  if (mins < 60) return `${mins} minute${mins === 1 ? '' : 's'} ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} day${days === 1 ? '' : 's'} ago`;
  return new Date(iso).toLocaleDateString();
}

export default function AccountPage() {
  const [account, setAccount] = useState<Account | null>(null);
  const [loading, setLoading] = useState(true);
  const [redirecting, setRedirecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [extTokens, setExtTokens] = useState<ExtensionToken[] | null>(null);
  const [revoking, setRevoking] = useState<string | null>(null);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  const [cancelledMsg, setCancelledMsg] = useState<string | null>(null);

  useEffect(() => {
    const t = localStorage.getItem('dr_access_token');
    if (!t) {
      window.location.href = '/login?next=' + encodeURIComponent('/account');
      return;
    }
    setToken(t);
    void load(t);
    void loadExtTokens(t);

    if (new URLSearchParams(window.location.search).get('subscribed') === '1') {
      // Lemon Squeezy redirected back after checkout — webhook may take a moment.
      setTimeout(() => void load(t), 2000);
    }
  }, []);

  const load = async (t: string) => {
    try {
      const res = await fetch(`${API}/auth/account`, {
        headers: { Authorization: `Bearer ${t}` },
      });
      if (res.status === 401) {
        // Session lost — bounce to login cleanly (no red error flash).
        setRedirecting(true);
        localStorage.removeItem('dr_access_token');
        window.location.href = '/login?next=' + encodeURIComponent('/account');
        return;
      }
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      if (!data) {
        // Valid token but no user (e.g. account deleted) — treat as
        // logged out rather than showing "Account not found".
        setRedirecting(true);
        localStorage.removeItem('dr_access_token');
        window.location.href = '/login?next=' + encodeURIComponent('/account');
        return;
      }
      setAccount(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load account');
    } finally {
      setLoading(false);
    }
  };

  // Subscribe / renew go through the multi-method checkout (VietQR, card, …),
  // the canonical pay flow. (LemonSqueezy "manage" stays only for LS-billed subs.)
  const subscribe = () => {
    window.location.href = '/checkout';
  };

  const manage = async () => {
    if (!token) return;
    setActionLoading('manage');
    try {
      // Unified endpoint — backend dispatches per the user's active
      // subscription source (Lemon Squeezy / Stripe).  Replaces the
      // LS-only /lemonsqueezy/portal route so Stripe-billed users
      // can also Manage; VietQR / bank / admin-granted return 404.
      const res = await fetch(`${API}/payment/portal`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: { url: string } = await res.json();
      // Open the provider portal in a new tab so the user keeps /account
      // instead of being yanked off to an LS/Stripe-branded page.
      window.open(data.url, '_blank', 'noopener,noreferrer');
      setActionLoading(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load portal');
      setActionLoading(null);
    }
  };

  // In-app cancel — same DELETE /payment/subscription the mobile app uses.
  // Cancels at the provider (Stripe / Lemon Squeezy) at period end; Pro is
  // retained until expires_at. Card/plan changes still go via `manage` (portal).
  const cancel = async () => {
    if (!token) return;
    setActionLoading('cancel');
    setError(null);
    try {
      const res = await fetch(`${API}/payment/subscription`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error || body?.message || `HTTP ${res.status}`);
      }
      const data: { cancelled: boolean; expires_at: string | null } = await res.json();
      const until = data.expires_at ? new Date(data.expires_at).toLocaleDateString() : null;
      setCancelledMsg(
        until
          ? `Cancelled. You keep Pro until ${until}, then move to the Free plan.`
          : 'Cancelled. You keep Pro until your current period ends.',
      );
      setConfirmingCancel(false);
      void load(token);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not cancel subscription');
    } finally {
      setActionLoading(null);
    }
  };

  const signOut = () => {
    localStorage.removeItem('dr_access_token');
    localStorage.removeItem('dr_refresh_token');
    window.location.href = '/';
  };

  const loadExtTokens = async (t: string) => {
    try {
      const res = await fetch(`${API}/auth/extension-tokens`, {
        headers: { Authorization: `Bearer ${t}` },
      });
      if (!res.ok) return; // silent — non-critical, log nothing user-visible
      setExtTokens(await res.json());
    } catch {
      // Network blip — don't disrupt the rest of the page.
    }
  };

  const revokeExtToken = async (id: string) => {
    if (!token) return;
    setRevoking(id);
    try {
      const res = await fetch(`${API}/auth/extension-tokens/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok && res.status !== 204) throw new Error(`HTTP ${res.status}`);
      setExtTokens((prev) => (prev ?? []).filter((t) => t.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not revoke device');
    } finally {
      setRevoking(null);
    }
  };

  if (loading || redirecting) {
    return <p className="text-center text-gray-400">Loading…</p>;
  }
  if (!account) {
    return <p className="text-center text-red-400">{error ?? 'Account not found'}</p>;
  }

  const sub = account.subscription;
  const isActive = !!sub && sub.status === 'active';
  const isPro = sub?.plan_name === 'Pro' && isActive;
  const isExpired = !!sub && sub.status !== 'active';
  // Only provider-billed subs can be cancelled / managed in-app.
  const isCancellable = isActive && (sub?.store_type === 'stripe' || sub?.store_type === 'lemonsqueezy');
  const accessUntil = sub?.expires_at ? new Date(sub.expires_at).toLocaleDateString() : null;

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div className="rounded-2xl border border-dark-border bg-dark-card p-8">
        <div className="flex items-baseline justify-between">
          <div>
            <p className="text-sm text-gray-500">Signed in as</p>
            <p className="text-lg text-white font-semibold">{account.name}</p>
            <p className="text-sm text-gray-400">{account.email}</p>
          </div>
          <button onClick={signOut} className="text-sm text-gray-400 hover:text-white">Sign out</button>
        </div>

        {!account.email_verified && (
          <p className="mt-4 rounded-lg bg-yellow-500/10 border border-yellow-500/30 p-3 text-sm text-yellow-300">
            Your email is not verified. <a href={`/verify-email?email=${encodeURIComponent(account.email)}`} className="underline">Verify now</a>
          </p>
        )}
      </div>

      <div className="rounded-2xl border border-dark-border bg-dark-card p-8">
        <p className="text-sm text-gray-500 mb-2">Current plan</p>
        <div className="flex items-baseline gap-3">
          <span className="text-3xl font-bold text-white">{isActive ? (sub?.plan_name ?? 'No plan') : 'No active plan'}</span>
          {isActive && <span className="text-xs uppercase tracking-wider text-brand-400">Active</span>}
          {isExpired && <span className="text-xs uppercase tracking-wider text-yellow-400">Expired</span>}
        </div>

        {sub && (
          <div className="mt-4 space-y-1 text-sm text-gray-400">
            <p>
              Daily limit: <span className="text-white">{sub.daily_limit === -1 ? 'Unlimited' : sub.daily_limit}</span> · Used today: <span className="text-white">{sub.usage_today}</span>
            </p>
            {sub.expires_at && (
              <p>{isActive ? 'Renews / expires' : 'Expired on'}: <span className="text-white">{new Date(sub.expires_at).toLocaleDateString()}</span></p>
            )}
          </div>
        )}

        {isExpired && (
          <p className="mt-4 rounded-lg bg-yellow-500/10 border border-yellow-500/30 p-3 text-sm text-yellow-300">
            Your subscription has ended. Renew to restore unlimited rewrites.
          </p>
        )}

        {cancelledMsg && (
          <p className="mt-4 rounded-lg bg-brand-400/10 border border-brand-400/30 p-3 text-sm text-brand-300">
            {cancelledMsg}
          </p>
        )}

        <div className="mt-6 flex flex-wrap gap-3 items-center">
          {isActive && !cancelledMsg ? (
            confirmingCancel ? (
              <>
                <span className="text-sm text-gray-300">
                  {accessUntil
                    ? `You'll keep Pro until ${accessUntil}, then move to Free. Cancel?`
                    : 'Pro continues until your period ends, then you move to Free. Cancel?'}
                </span>
                <button
                  onClick={() => setConfirmingCancel(false)}
                  className="rounded-full border border-brand-400 px-5 py-2.5 text-sm font-semibold text-brand-400 hover:bg-brand-400/10"
                >
                  Keep Pro
                </button>
                <button
                  onClick={cancel}
                  disabled={actionLoading === 'cancel'}
                  className="rounded-full bg-red-500/90 px-5 py-2.5 text-sm font-semibold text-white hover:bg-red-500 disabled:opacity-50"
                >
                  {actionLoading === 'cancel' ? 'Cancelling…' : 'Confirm cancel'}
                </button>
              </>
            ) : isCancellable ? (
              <>
                <button
                  onClick={() => setConfirmingCancel(true)}
                  className="rounded-full bg-brand-400 px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-500"
                >
                  Cancel subscription
                </button>
                <button
                  onClick={manage}
                  disabled={actionLoading === 'manage'}
                  className="rounded-full border border-brand-400 px-6 py-2.5 text-sm font-semibold text-brand-400 hover:bg-brand-400/10 disabled:opacity-50"
                >
                  {actionLoading === 'manage' ? 'Opening…' : 'Billing & invoices ↗'}
                </button>
              </>
            ) : (
              // VietQR / bank / admin-granted: no provider portal to cancel through.
              <button
                onClick={subscribe}
                className="rounded-full border border-brand-400 px-6 py-2.5 text-sm font-semibold text-brand-400 hover:bg-brand-400/10"
              >
                Change plan
              </button>
            )
          ) : !isActive ? (
            <button
              onClick={subscribe}
              className="rounded-full bg-brand-400 px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-500 disabled:opacity-50"
            >
              {isExpired ? 'Renew subscription' : 'Subscribe to Pro'}
            </button>
          ) : null}
          <a href="/download" className="rounded-full border border-brand-400 px-6 py-2.5 text-sm font-semibold text-brand-400 hover:bg-brand-400/10">
            Download app
          </a>
        </div>

        {isCancellable && !confirmingCancel && !cancelledMsg && (
          <p className="mt-3 text-xs text-gray-500">
            Billing &amp; invoices open our payment provider in a new tab for receipts and card changes.
          </p>
        )}

        {error && <p className="mt-3 text-sm text-red-400">{error}</p>}
      </div>

      {extTokens && extTokens.length > 0 && (
        <div className="rounded-2xl border border-dark-border bg-dark-card p-8">
          <p className="text-sm text-gray-500 mb-1">Active devices</p>
          <p className="text-xs text-gray-500 mb-4">
            Devices that can use DraftRight without going through this site. Revoke any you don't recognize.
          </p>
          <ul className="divide-y divide-dark-border">
            {extTokens.map((t) => (
              <li key={t.id} className="flex items-center justify-between py-3">
                <div>
                  <p className="text-sm text-white">{t.device_name}</p>
                  <p className="text-xs text-gray-500">
                    {t.last_used_at
                      ? `Last used ${formatRelative(t.last_used_at)}`
                      : `Added ${formatRelative(t.created_at)}, never used`}
                  </p>
                </div>
                <button
                  onClick={() => void revokeExtToken(t.id)}
                  disabled={revoking === t.id}
                  className="text-xs text-gray-400 hover:text-red-400 disabled:opacity-50"
                >
                  {revoking === t.id ? 'Revoking…' : 'Revoke'}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
