import { useEffect, useState } from 'react';
import PasswordInput from './PasswordInput';

import { API_URL as API } from '../lib/api';

export default function ResetPasswordForm() {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const e = new URLSearchParams(window.location.search).get('email');
    if (e) setEmail(e);
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/auth/reset-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim().toLowerCase(), code, new_password: password }),
      });
      if (!res.ok) {
        const body: { message?: string | string[]; error?: string } = await res.json().catch(() => ({}));
        const msg = Array.isArray(body.message) ? body.message[0] : (body.message || body.error);
        throw new Error(msg || `HTTP ${res.status}`);
      }
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not reset password');
    } finally {
      setSubmitting(false);
    }
  };

  if (done) {
    return (
      <div className="max-w-md mx-auto text-center space-y-6">
        <div className="text-5xl">✓</div>
        <p className="text-2xl font-semibold text-white">Password updated</p>
        <p className="text-gray-400">You can now log in with your new password.</p>
        <a
          href="/login"
          className="inline-block rounded-full bg-brand-400 px-6 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors"
        >
          Log in
        </a>
      </div>
    );
  }

  return (
    <form onSubmit={submit} className="max-w-md mx-auto space-y-4">
      {!email && (
        <input
          className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
          type="email"
          placeholder="Your email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          autoComplete="email"
        />
      )}
      {email && (
        <p className="text-center text-gray-400">
          Reset code sent to <strong className="text-white">{email}</strong>
        </p>
      )}
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white text-center text-3xl tracking-[0.5em] font-mono p-4 focus:outline-none focus:ring-2 focus:ring-brand-400"
        value={code}
        onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
        placeholder="000000"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        required
      />
      <PasswordInput
        value={password}
        onChange={setPassword}
        placeholder="New password (min 8 characters)"
        autoComplete="new-password"
        minLength={8}
        required
      />
      {error && <p className="text-red-400 text-sm text-center">{error}</p>}
      <button
        className="w-full rounded-full bg-brand-400 px-5 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        type="submit"
        disabled={submitting || code.length !== 6 || password.length < 8}
      >
        {submitting ? 'Updating…' : 'Set new password'}
      </button>
    </form>
  );
}
