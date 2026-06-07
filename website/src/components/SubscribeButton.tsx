import { useState } from 'react';

const API = (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';

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
      const res = await fetch(`${API}/lemonsqueezy/checkout`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
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
      const data: { url: string } = await res.json();
      window.location.href = data.url;
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
