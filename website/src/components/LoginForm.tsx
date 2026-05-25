import { useState } from 'react';

const API = (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';

export default function LoginForm() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim().toLowerCase(), password }),
      });
      if (!res.ok) {
        const body: { message?: string | string[] } = await res.json().catch(() => ({}));
        const msg = Array.isArray(body.message) ? body.message[0] : body.message;
        throw new Error(msg || (res.status === 401 ? 'Wrong email or password' : `HTTP ${res.status}`));
      }
      const data: { access_token: string; refresh_token?: string } = await res.json();
      localStorage.setItem('dr_access_token', data.access_token);
      if (data.refresh_token) localStorage.setItem('dr_refresh_token', data.refresh_token);
      const next = new URLSearchParams(window.location.search).get('next');
      window.location.href = next && next.startsWith('/') ? next : '/account';
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setSubmitting(false);
    }
  };

  const next = typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('next') : null;
  const signupHref = `/signup${next ? `?next=${encodeURIComponent(next)}` : ''}`;

  return (
    <form onSubmit={onSubmit} className="max-w-md mx-auto space-y-4">
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
        type="email"
        placeholder="Email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        autoComplete="email"
      />
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
        type="password"
        placeholder="Password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        autoComplete="current-password"
      />
      {error && <p className="text-red-400 text-sm">{error}</p>}
      <button
        className="w-full rounded-full bg-brand-400 px-5 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        type="submit"
        disabled={submitting}
      >
        {submitting ? 'Signing in…' : 'Log in'}
      </button>
      <p className="text-sm text-center text-gray-400">
        Don&apos;t have an account?{' '}
        <a href={signupHref} className="text-brand-400 hover:underline">Sign up</a>
      </p>
    </form>
  );
}
