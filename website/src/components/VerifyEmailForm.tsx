import { useEffect, useState } from 'react';
import { safeNextPath } from '../lib/redirect';

import { API_URL as API } from '../lib/api';

export default function VerifyEmailForm() {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [resending, setResending] = useState(false);
  const [resendSent, setResendSent] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const e = params.get('email');
    if (e) setEmail(e);
  }, []);

  const verify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API}/auth/verify-email`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim().toLowerCase(), code }),
      });
      if (!res.ok) {
        const body: { message?: string | string[] } = await res.json().catch(() => ({}));
        const msg = Array.isArray(body.message) ? body.message[0] : body.message;
        throw new Error(msg || `HTTP ${res.status}`);
      }
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed');
    } finally {
      setSubmitting(false);
    }
  };

  const resend = async () => {
    if (!email) {
      setError('Enter your email first');
      return;
    }
    setError(null);
    setResending(true);
    setResendSent(false);
    try {
      await fetch(`${API}/auth/resend-verification`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim().toLowerCase() }),
      });
      setResendSent(true);
    } finally {
      setResending(false);
    }
  };

  if (done) {
    const next = safeNextPath();
    if (next) {
      window.location.href = next;
      return null;
    }
    return (
      <div className="max-w-md mx-auto text-center space-y-6">
        <div className="text-5xl">✓</div>
        <p className="text-2xl font-semibold text-white">Email verified</p>
        <p className="text-gray-400">You're all set. Download the app or upgrade to Pro.</p>
        <div className="flex gap-3 justify-center">
          <a
            href="/download"
            className="rounded-full bg-brand-400 px-6 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors"
          >
            Download DraftRight
          </a>
          <a
            href="/account"
            className="rounded-full border border-brand-400 px-6 py-3 text-sm font-semibold text-brand-400 hover:bg-brand-400/10 transition-colors"
          >
            My account
          </a>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={verify} className="max-w-md mx-auto space-y-4">
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
          Code sent to <strong className="text-white">{email}</strong>
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
      {error && <p className="text-red-400 text-sm text-center">{error}</p>}
      {resendSent && <p className="text-green-400 text-sm text-center">A new code has been sent.</p>}
      <button
        className="w-full rounded-full bg-brand-400 px-5 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        type="submit"
        disabled={submitting || code.length !== 6}
      >
        {submitting ? 'Verifying…' : 'Verify email'}
      </button>
      <button
        type="button"
        onClick={resend}
        disabled={resending}
        className="w-full text-sm text-gray-400 hover:text-white transition-colors disabled:opacity-50"
      >
        {resending ? 'Sending…' : "Didn't get the code? Resend"}
      </button>
    </form>
  );
}
