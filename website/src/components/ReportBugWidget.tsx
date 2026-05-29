import { useEffect, useRef, useState } from 'react';

const API_URL =
  (typeof import.meta !== 'undefined' && import.meta.env?.PUBLIC_API_URL) ||
  'https://api.draftright.info';

const APP_VERSION = 'playground-1.0.0';
const MAX_SCREENSHOT_BYTES = 5 * 1024 * 1024; // 5 MB
const ACCEPTED_TYPES = ['image/png', 'image/jpeg'];

export interface PlaygroundState {
  text: string;
  tone: string;
  sourceLanguage: string;
  targetLanguage: string;
  result: string;
}

interface Props {
  /** Snapshot of the playground at submit time. Used when the user opts to attach context. */
  playgroundState: () => PlaygroundState;
}

function getAccessToken(): string | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.localStorage.getItem('dr_access_token');
  } catch {
    return null;
  }
}

export default function ReportBugWidget({ playgroundState }: Props) {
  const [open, setOpen] = useState(false);
  const [description, setDescription] = useState('');
  const [email, setEmail] = useState('');
  const [includeContext, setIncludeContext] = useState(true);
  const [screenshot, setScreenshot] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const [hasToken, setHasToken] = useState(false);
  // Honeypot — see the hidden <input name="website"> in the JSX below.
  const [website, setWebsite] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // Detect login state when modal opens.
  useEffect(() => {
    if (open) {
      setHasToken(Boolean(getAccessToken()));
    }
  }, [open]);

  // Close on Escape.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeModal();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const resetForm = () => {
    setDescription('');
    setEmail('');
    setScreenshot(null);
    setIncludeContext(true);
    setError('');
    setSuccess(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const openModal = () => {
    resetForm();
    setOpen(true);
  };

  const closeModal = () => {
    if (submitting) return;
    setOpen(false);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setError('');
    const file = e.target.files?.[0] ?? null;
    if (!file) {
      setScreenshot(null);
      return;
    }
    if (!ACCEPTED_TYPES.includes(file.type)) {
      setError('Screenshot must be a PNG or JPEG image.');
      e.target.value = '';
      return;
    }
    if (file.size > MAX_SCREENSHOT_BYTES) {
      setError('Screenshot must be smaller than 5 MB.');
      e.target.value = '';
      return;
    }
    setScreenshot(file);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    const trimmed = description.trim();
    if (trimmed.length < 10) {
      setError('Please describe the issue (at least 10 characters).');
      return;
    }

    setSubmitting(true);
    try {
      const token = getAccessToken();
      const fd = new FormData();
      // Honeypot — bots fill this, humans don't. Backend drops silently.
      fd.append('website', website);
      fd.append('description', trimmed);
      fd.append('source', 'web-playground');
      fd.append('app_version', APP_VERSION);
      if (typeof navigator !== 'undefined') {
        fd.append('os_info', navigator.userAgent);
      }
      if (!token && email.trim()) {
        fd.append('user_email', email.trim());
      }

      const url = typeof window !== 'undefined' ? window.location.href : '';
      const route = typeof window !== 'undefined' ? window.location.pathname : '';
      const ctx: Record<string, unknown> = { url, route };
      if (includeContext) {
        const snap = playgroundState();
        ctx.playground = {
          text: snap.text,
          tone: snap.tone,
          source_language: snap.sourceLanguage,
          target_language: snap.targetLanguage,
          last_result: snap.result,
        };
      }
      fd.append('context', JSON.stringify(ctx));

      if (screenshot) {
        fd.append('screenshot', screenshot, screenshot.name);
      }

      const headers: Record<string, string> = {};
      if (token) headers.Authorization = `Bearer ${token}`;

      const res = await fetch(`${API_URL}/bug-reports`, {
        method: 'POST',
        body: fd,
        headers,
      });

      if (!res.ok) {
        let msg = `Failed to submit (HTTP ${res.status}).`;
        try {
          const data = await res.json();
          if (data?.error) msg = data.error;
          else if (data?.message) msg = Array.isArray(data.message) ? data.message.join(', ') : data.message;
        } catch {
          /* ignore */
        }
        setError(msg);
        return;
      }

      setSuccess(true);
      // Auto-close after a short delay so the user sees the confirmation.
      setTimeout(() => {
        setOpen(false);
      }, 1800);
    } catch (err: any) {
      setError(err?.message || 'Network error. Please try again.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <div className="mt-8 flex justify-center">
        <button
          type="button"
          onClick={openModal}
          className="inline-flex items-center gap-1.5 text-sm text-slate-400 hover:text-brand-400 transition-colors"
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 9v2m0 4h.01M5.07 19h13.86c1.54 0 2.5-1.67 1.73-3L13.73 4a2 2 0 00-3.46 0L3.34 16c-.77 1.33.19 3 1.73 3z"
            />
          </svg>
          Report a playground issue
        </button>
      </div>

      {open && (
        <div
          className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          aria-labelledby="report-bug-title"
          onClick={closeModal}
        >
          <div
            className="w-full max-w-lg rounded-xl border border-dark-border bg-dark-bg p-6 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between mb-4">
              <h3 id="report-bug-title" className="text-xl font-semibold text-white">
                Report a playground issue
              </h3>
              <button
                type="button"
                onClick={closeModal}
                disabled={submitting}
                aria-label="Close"
                className="text-slate-400 hover:text-white transition-colors disabled:opacity-50"
              >
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            {success ? (
              <div className="rounded-lg border border-emerald-700/50 bg-emerald-950/30 p-4 text-emerald-200">
                Thanks! We'll look into it.
              </div>
            ) : (
              <form onSubmit={handleSubmit} className="space-y-4">
                {/* Honeypot — off-screen + aria-hidden so humans never touch it. */}
                <div aria-hidden="true" style={{ position: 'absolute', left: '-10000px', top: 'auto', width: '1px', height: '1px', overflow: 'hidden' }}>
                  <label>
                    Website
                    <input
                      type="text"
                      name="website"
                      tabIndex={-1}
                      autoComplete="off"
                      value={website}
                      onChange={(e) => setWebsite(e.target.value)}
                    />
                  </label>
                </div>
                <div>
                  <label htmlFor="rb-description" className="block text-sm font-medium text-slate-300 mb-1.5">
                    What went wrong? <span className="text-red-400">*</span>
                  </label>
                  <textarea
                    id="rb-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    rows={4}
                    minLength={10}
                    required
                    placeholder="Describe what happened, what you expected, and the steps to reproduce..."
                    className="w-full rounded-lg bg-black/40 border border-dark-border text-white placeholder-slate-500 p-3 resize-y focus:outline-none focus:ring-2 focus:ring-brand-400 focus:border-transparent text-sm"
                  />
                  <p className="mt-1 text-xs text-slate-500">Minimum 10 characters.</p>
                </div>

                {!hasToken && (
                  <div>
                    <label htmlFor="rb-email" className="block text-sm font-medium text-slate-300 mb-1.5">
                      Email <span className="text-slate-500 font-normal">(optional)</span>
                    </label>
                    <input
                      id="rb-email"
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="you@example.com"
                      className="w-full rounded-lg bg-black/40 border border-dark-border text-white placeholder-slate-500 p-2.5 focus:outline-none focus:ring-2 focus:ring-brand-400 focus:border-transparent text-sm"
                    />
                    <p className="mt-1 text-xs text-slate-500">Leave it if you'd like a reply.</p>
                  </div>
                )}

                <div>
                  <label htmlFor="rb-screenshot" className="block text-sm font-medium text-slate-300 mb-1.5">
                    Screenshot <span className="text-slate-500 font-normal">(optional, PNG/JPEG, &lt; 5 MB)</span>
                  </label>
                  <input
                    id="rb-screenshot"
                    ref={fileInputRef}
                    type="file"
                    accept="image/png,image/jpeg"
                    onChange={handleFileChange}
                    className="block w-full text-sm text-slate-400 file:mr-3 file:py-2 file:px-3 file:rounded-md file:border-0 file:text-sm file:font-medium file:bg-brand-400/15 file:text-brand-400 hover:file:bg-brand-400/25 file:cursor-pointer"
                  />
                </div>

                <label className="flex items-start gap-2 text-sm text-slate-300 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={includeContext}
                    onChange={(e) => setIncludeContext(e.target.checked)}
                    className="mt-0.5 h-4 w-4 rounded border-dark-border bg-black/40 text-brand-400 focus:ring-brand-400 focus:ring-offset-0"
                  />
                  <span>
                    Include current playground state
                    <span className="block text-xs text-slate-500">
                      Attaches your input text, selected tone, and last result so we can reproduce.
                    </span>
                  </span>
                </label>

                {error && (
                  <p className="rounded-md border border-red-700/40 bg-red-950/30 p-2.5 text-sm text-red-300">{error}</p>
                )}

                <div className="flex items-center justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={closeModal}
                    disabled={submitting}
                    className="px-4 py-2 rounded-lg border border-dark-border text-slate-300 hover:text-white hover:border-slate-500 transition-colors text-sm font-medium disabled:opacity-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={submitting}
                    className="px-4 py-2 rounded-lg bg-brand-400 text-white text-sm font-semibold hover:bg-brand-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center gap-2"
                  >
                    {submitting && (
                      <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                      </svg>
                    )}
                    {submitting ? 'Sending...' : 'Submit'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </>
  );
}
