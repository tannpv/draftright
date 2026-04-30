import { useEffect, useState } from 'react';

const API = (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';

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

export default function AccountPage() {
  const [account, setAccount] = useState<Account | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);

  useEffect(() => {
    const t = localStorage.getItem('dr_access_token');
    if (!t) {
      window.location.href = '/signup?next=' + encodeURIComponent('/account');
      return;
    }
    setToken(t);
    void load(t);

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
        localStorage.removeItem('dr_access_token');
        window.location.href = '/signup?next=' + encodeURIComponent('/account');
        return;
      }
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setAccount(await res.json());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load account');
    } finally {
      setLoading(false);
    }
  };

  const subscribe = async () => {
    if (!token) return;
    setActionLoading('subscribe');
    try {
      const res = await fetch(`${API}/lemonsqueezy/checkout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: { url: string } = await res.json();
      window.location.href = data.url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start checkout');
      setActionLoading(null);
    }
  };

  const manage = async () => {
    if (!token) return;
    setActionLoading('manage');
    try {
      const res = await fetch(`${API}/lemonsqueezy/portal`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: { url: string } = await res.json();
      window.location.href = data.url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load portal');
      setActionLoading(null);
    }
  };

  const signOut = () => {
    localStorage.removeItem('dr_access_token');
    localStorage.removeItem('dr_refresh_token');
    window.location.href = '/';
  };

  if (loading) {
    return <p className="text-center text-gray-400">Loading…</p>;
  }
  if (!account) {
    return <p className="text-center text-red-400">{error ?? 'Account not found'}</p>;
  }

  const isPro = account.subscription?.plan_name === 'Pro' && account.subscription.status === 'active';

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
          <span className="text-3xl font-bold text-white">{account.subscription?.plan_name ?? 'No plan'}</span>
          {isPro && <span className="text-xs uppercase tracking-wider text-brand-400">Active</span>}
        </div>

        {account.subscription && (
          <div className="mt-4 space-y-1 text-sm text-gray-400">
            <p>
              Daily limit: <span className="text-white">{account.subscription.daily_limit === -1 ? 'Unlimited' : account.subscription.daily_limit}</span> · Used today: <span className="text-white">{account.subscription.usage_today}</span>
            </p>
            {account.subscription.expires_at && (
              <p>Renews / expires: <span className="text-white">{new Date(account.subscription.expires_at).toLocaleDateString()}</span></p>
            )}
          </div>
        )}

        <div className="mt-6 flex gap-3">
          {isPro ? (
            <button
              onClick={manage}
              disabled={actionLoading === 'manage'}
              className="rounded-full bg-brand-400 px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-500 disabled:opacity-50"
            >
              {actionLoading === 'manage' ? 'Opening…' : 'Manage subscription'}
            </button>
          ) : (
            <button
              onClick={subscribe}
              disabled={actionLoading === 'subscribe'}
              className="rounded-full bg-brand-400 px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-500 disabled:opacity-50"
            >
              {actionLoading === 'subscribe' ? 'Opening checkout…' : 'Upgrade to Pro · $4.99/mo'}
            </button>
          )}
          <a href="/download" className="rounded-full border border-brand-400 px-6 py-2.5 text-sm font-semibold text-brand-400 hover:bg-brand-400/10">
            Download app
          </a>
        </div>

        {error && <p className="mt-3 text-sm text-red-400">{error}</p>}
      </div>
    </div>
  );
}
