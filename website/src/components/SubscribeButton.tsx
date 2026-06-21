import { useState } from 'react';

import { API_URL as API } from '../lib/api';

// GET /plans shape (subset we need). Prices/IDs come from the backend so they
// never go stale.
interface ApiPlan {
  id: string;
  name: string;
  currency: string | null;
  billing_period: 'none' | 'monthly' | 'yearly';
  is_active: boolean;
}

/**
 * Resolve the Pro-monthly plan id from the backend. The unified
 * `/payment/checkout` route picks the Lemon Squeezy monthly vs yearly variant
 * from the plan's `billing_period`, so any active Pro-monthly plan works.
 * Prefer the USD plan — Lemon Squeezy charges in USD.
 */
async function resolveProMonthlyPlanId(): Promise<string> {
  const res = await fetch(`${API}/plans`);
  if (!res.ok) throw new Error(`Could not load plans (HTTP ${res.status})`);
  const plans: ApiPlan[] = await res.json();
  const monthly = plans.filter(
    (p) => p.is_active && p.name === 'Pro' && p.billing_period === 'monthly',
  );
  const chosen = monthly.find((p) => (p.currency || '').toUpperCase() === 'USD') ?? monthly[0];
  if (!chosen) throw new Error('No active Pro plan available');
  return chosen.id;
}

export default function SubscribeButton() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onClick = async () => {
    setError(null);
    const token = typeof window !== 'undefined' ? localStorage.getItem('dr_access_token') : null;
    if (!token) {
      window.location.href = '/signup?next=' + encodeURIComponent('/pricing#subscribe');
      return;
    }

    setLoading(true);
    try {
      const planId = await resolveProMonthlyPlanId();
      const res = await fetch(`${API}/payment/checkout`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ plan_id: planId, method: 'lemonsqueezy' }),
      });

      if (res.status === 401) {
        // Token expired — bounce through signup so they can sign in or refresh
        localStorage.removeItem('dr_access_token');
        window.location.href = '/signup?next=' + encodeURIComponent('/pricing#subscribe');
        return;
      }

      if (!res.ok) {
        const body: { message?: string | string[] } = await res.json().catch(() => ({}));
        const msg = Array.isArray(body.message) ? body.message[0] : body.message;
        throw new Error(msg || `HTTP ${res.status}`);
      }
      const data: { redirect_url?: string } = await res.json();
      if (!data.redirect_url) throw new Error('Checkout did not return a redirect URL');
      window.location.href = data.redirect_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start checkout');
      setLoading(false);
    }
  };

  return (
    <div id="subscribe">
      <button
        onClick={onClick}
        disabled={loading}
        className="w-full mt-8 rounded-full bg-brand-400 px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {loading ? 'Opening checkout…' : 'Subscribe to Pro'}
      </button>
      {error && <p className="mt-2 text-xs text-red-400 text-center">{error}</p>}
      <p className="mt-3 text-center text-xs text-gray-500">Card or VietQR · Cancel anytime</p>
    </div>
  );
}
