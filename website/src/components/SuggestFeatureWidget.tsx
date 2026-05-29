import { useState } from 'react';

const API_URL =
  (typeof import.meta !== 'undefined' && (import.meta as any).env?.PUBLIC_API_URL) ||
  'https://api.draftright.info';
const BOARD_URL = 'https://draftright.info/feedback';

const PLATFORMS: Array<{ value: string; label: string }> = [
  { value: 'playground', label: 'Playground (web)' },
  { value: 'mobile', label: 'Mobile (iOS / Android)' },
  { value: 'windows', label: 'Windows' },
  { value: 'mac', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
];

function getAccessToken(): string | null {
  try { return localStorage.getItem('dr_access_token'); } catch { return null; }
}

/**
 * Compact "Suggest a feature" launcher + modal for the web playground.
 * Mirrors ReportBugWidget but posts JSON to POST /feedback with
 * kind:"feature" and a target_platform the user picks from a dropdown.
 */
export default function SuggestFeatureWidget() {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState('');
  const [platform, setPlatform] = useState('playground');
  const [description, setDescription] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [website, setWebsite] = useState(''); // honeypot

  const token = getAccessToken();
  const canSubmit = title.trim().length > 0 && description.trim().length > 0 && !busy;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true); setError(null);
    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const body: Record<string, unknown> = {
        kind: 'feature',
        title: title.trim(),
        target_platform: platform,
        description: description.trim(),
        source: 'web',
        website, // honeypot — empty for humans, filled by bots; server drops if set
      };
      if (!token && email.trim()) body.user_email = email.trim();
      const res = await fetch(`${API_URL}/feedback`, {
        method: 'POST', headers, body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`server returned ${res.status}`);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'something went wrong');
    } finally {
      setBusy(false);
    }
  }

  function reset() {
    setOpen(false); setDone(false); setError(null);
    setTitle(''); setDescription(''); setEmail(''); setPlatform('playground');
  }

  return (
    <div className="suggest-feature-widget" style={{ marginTop: '0.75rem' }}>
      <button type="button" onClick={() => setOpen(true)}
        style={{ fontSize: '0.85rem', background: 'transparent', border: '1px solid #334155',
                 color: '#94a3b8', borderRadius: 8, padding: '6px 12px', cursor: 'pointer' }}>
        💡 Suggest a feature
      </button>
      {open && (
        <div role="dialog" aria-modal="true"
          style={{ position: 'fixed', inset: 0, background: '#0008', display: 'flex',
                   alignItems: 'center', justifyContent: 'center', padding: 20, zIndex: 1000 }}
          onClick={(e) => { if (e.target === e.currentTarget) reset(); }}>
          <div style={{ background: '#161b22', border: '1px solid #2a3240', borderRadius: 14,
                        width: '100%', maxWidth: 460, padding: 20, color: '#e6edf3' }}>
            {done ? (
              <>
                <h3 style={{ marginBottom: 8 }}>Thanks!</h3>
                <p style={{ color: '#94a3b8', fontSize: 14, marginBottom: 16 }}>
                  Your feature request was submitted. Track it on the board.
                </p>
                <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                  <a href={BOARD_URL} target="_blank" rel="noopener"
                     style={{ color: '#6aa6ff', alignSelf: 'center', fontSize: 13 }}>See all requests →</a>
                  <button onClick={reset} style={{ background: '#3b82f6', color: '#fff',
                    border: 0, borderRadius: 9, padding: '8px 14px', cursor: 'pointer' }}>Close</button>
                </div>
              </>
            ) : (
              <form onSubmit={submit}>
                {/* Honeypot — off-screen so humans never touch it. */}
                <div aria-hidden="true" style={{ position: 'absolute', left: '-10000px', width: '1px', height: '1px', overflow: 'hidden' }}>
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
                <h3 style={{ marginBottom: 4 }}>Suggest a feature</h3>
                <p style={{ color: '#94a3b8', fontSize: 12, marginBottom: 16 }}>
                  {token ? 'Submitted under your account. ' : ''}Public on the feature board.
                </p>
                <label style={{ fontSize: 12, color: '#94a3b8' }}>Title</label>
                <input value={title} maxLength={80} onChange={(e) => setTitle(e.target.value)}
                  placeholder="One line — what should we build?"
                  style={{ width: '100%', margin: '5px 0 14px', background: '#1c2230',
                           border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px' }} />
                <label style={{ fontSize: 12, color: '#94a3b8' }}>Which platform is this for?</label>
                <select value={platform} onChange={(e) => setPlatform(e.target.value)}
                  style={{ width: '100%', margin: '5px 0 14px', background: '#1c2230',
                           border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px' }}>
                  {PLATFORMS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
                </select>
                <label style={{ fontSize: 12, color: '#94a3b8' }}>Details</label>
                <textarea value={description} maxLength={2000} onChange={(e) => setDescription(e.target.value)}
                  placeholder="What problem does it solve? How would it work?"
                  style={{ width: '100%', margin: '5px 0 14px', minHeight: 90, background: '#1c2230',
                           border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px', resize: 'vertical' }} />
                {!token && (
                  <>
                    <label style={{ fontSize: 12, color: '#94a3b8' }}>Email (optional — to follow up)</label>
                    <input value={email} type="email" onChange={(e) => setEmail(e.target.value)}
                      style={{ width: '100%', margin: '5px 0 14px', background: '#1c2230',
                               border: '1px solid #2a3240', borderRadius: 9, color: '#e6edf3', padding: '9px 11px' }} />
                  </>
                )}
                {error && <p style={{ color: '#f87171', fontSize: 13, marginBottom: 10 }}>{error}</p>}
                <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', alignItems: 'center' }}>
                  <a href={BOARD_URL} target="_blank" rel="noopener" style={{ color: '#6aa6ff', fontSize: 13, marginRight: 'auto' }}>See all requests →</a>
                  <button type="button" onClick={reset} style={{ background: 'transparent',
                    border: '1px solid #2a3240', color: '#94a3b8', borderRadius: 9, padding: '8px 14px', cursor: 'pointer' }}>Cancel</button>
                  <button type="submit" disabled={!canSubmit} style={{ background: canSubmit ? '#22c55e' : '#1c2230',
                    color: canSubmit ? '#04210f' : '#475569', border: 0, borderRadius: 9, padding: '8px 16px',
                    fontWeight: 700, cursor: canSubmit ? 'pointer' : 'not-allowed' }}>
                    {busy ? 'Submitting…' : 'Submit request'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
