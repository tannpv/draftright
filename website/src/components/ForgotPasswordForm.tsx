import { useState } from 'react';

import { API_URL as API } from '../lib/api';

export default function ForgotPasswordForm() {
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      // Endpoint always returns success — never reveals whether the
      // email exists. We mirror that in the UI.
      await fetch(`${API}/auth/forgot-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim().toLowerCase() }),
      });
      setSent(true);
    } catch {
      // Even on a network blip, don't leak — show the neutral message.
      setSent(true);
    } finally {
      setSubmitting(false);
    }
  };

  if (sent) {
    const resetHref = `/reset-password?email=${encodeURIComponent(email.trim().toLowerCase())}`;
    return (
      <div className="max-w-md mx-auto text-center space-y-6">
        <div className="text-5xl">✉️</div>
        <p className="text-2xl font-semibold text-white">Check your email</p>
        <p className="text-gray-400">
          If an account exists for <strong className="text-white">{email}</strong>, we sent a 6-digit reset code. It expires in 15 minutes.
        </p>
        <a
          href={resetHref}
          className="inline-block rounded-full bg-brand-400 px-6 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors"
        >
          Enter reset code
        </a>
      </div>
    );
  }

  return (
    <form onSubmit={submit} className="max-w-md mx-auto space-y-4">
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
        type="email"
        placeholder="Your email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        autoComplete="email"
      />
      <button
        className="w-full rounded-full bg-brand-400 px-5 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        type="submit"
        disabled={submitting}
      >
        {submitting ? 'Sending…' : 'Send reset code'}
      </button>
      <p className="text-sm text-center text-gray-400">
        Remembered it? <a href="/login" className="text-brand-400 hover:underline">Log in</a>
      </p>
    </form>
  );
}
