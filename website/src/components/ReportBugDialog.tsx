import { useEffect, useRef, useState } from 'react';

const API = (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';
const MAX_FILE_BYTES = 5 * 1024 * 1024;
const ACCEPTED_MIME = ['image/png', 'image/jpeg'];

type SendState = 'idle' | 'sending' | 'success' | 'error';

export default function ReportBugDialog() {
  const [open, setOpen] = useState(false);
  const [description, setDescription] = useState('');
  const [email, setEmail] = useState('');
  const [file, setFile] = useState<File | null>(null);
  // Honeypot — see the hidden <input name="website"> below.
  const [website, setWebsite] = useState('');
  const [fileError, setFileError] = useState<string | null>(null);
  const [sendState, setSendState] = useState<SendState>('idle');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dialogRef = useRef<HTMLDivElement | null>(null);

  // Close on Escape; lock body scroll while open.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeDialog();
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = previousOverflow;
    };
  }, [open]);

  function resetForm() {
    setDescription('');
    setEmail('');
    setFile(null);
    setFileError(null);
    setErrorMessage(null);
    setSendState('idle');
    if (fileInputRef.current) fileInputRef.current.value = '';
  }

  function openDialog() {
    resetForm();
    setOpen(true);
  }

  function closeDialog() {
    setOpen(false);
    // small delay to avoid flicker mid-transition; reset on next open instead
  }

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    setFileError(null);
    const f = e.target.files?.[0] || null;
    if (!f) {
      setFile(null);
      return;
    }
    if (!ACCEPTED_MIME.includes(f.type)) {
      setFileError('Please choose a PNG or JPEG image.');
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      return;
    }
    if (f.size > MAX_FILE_BYTES) {
      setFileError('Screenshot must be 5 MB or smaller.');
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      return;
    }
    setFile(f);
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (sendState === 'sending') return;
    setErrorMessage(null);
    setSendState('sending');
    try {
      const fd = new FormData();
      // Honeypot — humans never see/edit this field, so a non-empty value is
      // a reliable bot signal. The backend silently drops the submission
      // without inserting a row.
      fd.append('website', website);
      fd.append('description', description.trim());
      fd.append('source', 'marketing-site');
      fd.append('user_email', email.trim().toLowerCase());
      fd.append('app_version', 'marketing-1.0.0');
      fd.append('os_info', navigator.userAgent);
      fd.append(
        'context',
        JSON.stringify({
          url: window.location.href,
          referrer: document.referrer,
        }),
      );
      if (file) fd.append('screenshot', file);

      const res = await fetch(`${API}/bug-reports`, {
        method: 'POST',
        body: fd,
      });
      if (!res.ok) {
        let msg = `HTTP ${res.status}`;
        try {
          const body: { error?: string; message?: string | string[] } = await res.json();
          const m = body.error || (Array.isArray(body.message) ? body.message[0] : body.message);
          if (m) msg = m;
        } catch {
          /* ignore json parse */
        }
        throw new Error(msg);
      }
      setSendState('success');
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : 'Something went wrong');
      setSendState('error');
    }
  }

  return (
    <>
      {/* Floating trigger */}
      <button
        type="button"
        onClick={openDialog}
        aria-label="Report a bug"
        className="fixed bottom-5 right-5 z-50 inline-flex items-center gap-2 rounded-full bg-dark-card/90 backdrop-blur border border-dark-border px-4 py-2 text-sm font-medium text-gray-200 shadow-lg hover:text-white hover:border-brand-400 hover:bg-dark-card transition-colors"
      >
        <span aria-hidden="true">🐛</span>
        <span>Report</span>
      </button>

      {/* Dialog */}
      {open && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="report-bug-title"
          className="fixed inset-0 z-50 flex items-center justify-center px-4 py-6"
        >
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/70 backdrop-blur-sm"
            onClick={closeDialog}
            aria-hidden="true"
          />

          {/* Panel */}
          <div
            ref={dialogRef}
            className="relative w-full max-w-lg rounded-2xl border border-dark-border bg-dark-card text-gray-200 shadow-2xl"
          >
            {sendState === 'success' ? (
              <div className="p-6 sm:p-8">
                <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-400 text-2xl">
                  ✓
                </div>
                <h2 id="report-bug-title" className="text-center text-xl font-semibold text-white">
                  Thanks! We'll look into it.
                </h2>
                <p className="mt-2 text-center text-sm text-gray-400">
                  Your report has been received. If we need more details, we'll email you.
                </p>
                <div className="mt-6 flex justify-center">
                  <button
                    type="button"
                    onClick={closeDialog}
                    className="rounded-full bg-brand-400 px-6 py-2 text-sm font-semibold text-white hover:bg-brand-500 transition-colors"
                  >
                    Close
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={onSubmit} className="p-6 sm:p-8">
                {/*
                  Honeypot: rendered off-screen with aria-hidden + autocomplete="off"
                  so screen readers + real users never interact with it, but
                  scrapers + form-fillers see it as a standard "website" field
                  and fill it. Filled value → backend silently drops the submission.
                */}
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
                <div className="mb-5">
                  <h2 id="report-bug-title" className="text-xl font-semibold text-white">
                    Report a bug
                  </h2>
                  <p className="mt-1 text-sm text-gray-400">
                    Found something broken? Tell us about it.
                  </p>
                </div>

                {sendState === 'error' && errorMessage && (
                  <div className="mb-4 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-300">
                    {errorMessage}
                  </div>
                )}

                <div className="space-y-4">
                  <label className="block">
                    <span className="block text-sm font-medium text-gray-300 mb-1">
                      What happened? <span className="text-red-400">*</span>
                    </span>
                    <textarea
                      className="w-full rounded-lg bg-dark-bg border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400 min-h-[120px]"
                      placeholder="e.g. Clicked Try It Free on the homepage and the page never loaded."
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      required
                      minLength={10}
                      disabled={sendState === 'sending'}
                    />
                    <span className="mt-1 block text-xs text-gray-500">Minimum 10 characters.</span>
                  </label>

                  <label className="block">
                    <span className="block text-sm font-medium text-gray-300 mb-1">
                      Your email <span className="text-red-400">*</span>
                    </span>
                    <input
                      type="email"
                      className="w-full rounded-lg bg-dark-bg border border-dark-border text-white placeholder-gray-500 p-3 focus:outline-none focus:ring-2 focus:ring-brand-400"
                      placeholder="you@example.com"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      required
                      autoComplete="email"
                      disabled={sendState === 'sending'}
                    />
                    <span className="mt-1 block text-xs text-gray-500">
                      So we can follow up if we need more info.
                    </span>
                  </label>

                  <div className="block">
                    <span className="block text-sm font-medium text-gray-300 mb-1">
                      Attach screenshot <span className="text-gray-500 font-normal">(optional)</span>
                    </span>
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="image/png,image/jpeg"
                      onChange={onFileChange}
                      disabled={sendState === 'sending'}
                      className="block w-full text-sm text-gray-400 file:mr-3 file:rounded-full file:border-0 file:bg-dark-bg file:px-4 file:py-2 file:text-sm file:font-medium file:text-gray-200 file:cursor-pointer hover:file:bg-dark-border"
                    />
                    {file && (
                      <span className="mt-1 block text-xs text-gray-400">
                        Selected: <span className="text-gray-200">{file.name}</span>{' '}
                        <span className="text-gray-500">
                          ({Math.round((file.size / 1024) * 10) / 10} KB)
                        </span>
                      </span>
                    )}
                    {fileError && (
                      <span className="mt-1 block text-xs text-red-400">{fileError}</span>
                    )}
                    {!file && !fileError && (
                      <span className="mt-1 block text-xs text-gray-500">
                        PNG or JPEG, max 5 MB.
                      </span>
                    )}
                  </div>
                </div>

                <div className="mt-6 flex items-center justify-end gap-3">
                  <button
                    type="button"
                    onClick={closeDialog}
                    disabled={sendState === 'sending'}
                    className="rounded-full px-5 py-2 text-sm font-medium text-gray-300 hover:text-white transition-colors disabled:opacity-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={sendState === 'sending'}
                    className="inline-flex items-center gap-2 rounded-full bg-brand-400 px-5 py-2 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                  >
                    {sendState === 'sending' && (
                      <svg
                        className="h-4 w-4 animate-spin"
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                      >
                        <circle
                          className="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          strokeWidth="4"
                        />
                        <path
                          className="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4H4z"
                        />
                      </svg>
                    )}
                    {sendState === 'sending' ? 'Sending…' : 'Send'}
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
