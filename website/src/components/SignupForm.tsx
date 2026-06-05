import { useState } from 'react';
import GoogleSignInButton from './GoogleSignInButton';
import PasswordInput from './PasswordInput';
import { goToNext } from '../lib/redirect';

const API = (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';

export default function SignupForm() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/auth/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: name.trim(),
          email: email.trim().toLowerCase(),
          password,
        }),
      });
      if (!res.ok) {
        const body: { message?: string | string[] } = await res.json().catch(() => ({}));
        const msg = Array.isArray(body.message) ? body.message[0] : body.message;
        throw new Error(msg || `HTTP ${res.status}`);
      }
      const data: { access_token: string; refresh_token: string } = await res.json();
      localStorage.setItem('dr_access_token', data.access_token);
      localStorage.setItem('dr_refresh_token', data.refresh_token);
      const next = new URLSearchParams(window.location.search).get('next');
      const verifyUrl = `/verify-email?email=${encodeURIComponent(email.trim().toLowerCase())}${next ? `&next=${encodeURIComponent(next)}` : ''}`;
      window.location.href = verifyUrl;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={onSubmit} className="max-w-md mx-auto space-y-4">
      <GoogleSignInButton onSuccess={() => goToNext()} onError={setError} disabled={submitting} label="Sign up with Google" />
      <div className="flex items-center gap-3">
        <div className="flex-1 h-px bg-dark-border" />
        <span className="text-sm text-gray-500">or</span>
        <div className="flex-1 h-px bg-dark-border" />
      </div>
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
        placeholder="Your name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        required
        minLength={2}
        autoComplete="name"
      />
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
        type="email"
        placeholder="Email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        autoComplete="email"
      />
      <PasswordInput
        value={password}
        onChange={setPassword}
        placeholder="Password (min 8 characters)"
        autoComplete="new-password"
        minLength={8}
        required
      />
      {error && <p className="text-red-400 text-sm">{error}</p>}
      <button
        className="w-full rounded-full bg-brand-400 px-5 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        type="submit"
        disabled={submitting}
      >
        {submitting ? 'Creating account…' : 'Create account'}
      </button>
      <p className="text-sm text-center text-gray-400">
        Already have an account?{' '}
        <a href="/download" className="text-brand-400 hover:underline">Download the app</a>
      </p>
    </form>
  );
}
