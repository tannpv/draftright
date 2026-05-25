import { useState, useEffect } from 'react';
import Toast from '../components/Toast';
import { apiFetch } from '../api';

interface Settings {
  environment: string;
  trial_limit: number;
  token_expiry_minutes: number;
  refresh_token_expiry_days: number;
  max_input_length: number;
  supported_languages: string;
  // Payment
  stripe_secret_key: string;
  stripe_webhook_secret: string;
  stripe_mode: string;
  paypal_client_id: string;
  paypal_client_secret: string;
  paypal_mode: string;
  momo_partner_code: string;
  momo_access_key: string;
  momo_secret_key: string;
  momo_mode: string;
  vietqr_bank_id: string;
  vietqr_account_number: string;
  vietqr_account_name: string;
  casso_api_key: string;
  sepay_api_key: string;
  sepay_mode: string;
  // Email
  resend_api_key: string;
  email_from: string;
  // Google
  google_client_id: string;
  google_client_secret: string;
  // Apple
  apple_client_id: string;
  apple_team_id: string;
  apple_key_id: string;
  // Diagnostics
  client_log_level: string;
  [key: string]: any;
}

const ALL_LANGUAGES = [
  'Arabic', 'Chinese (Simplified)', 'Chinese (Traditional)',
  'Czech', 'Danish', 'Dutch', 'English', 'Finnish', 'French',
  'German', 'Greek', 'Hebrew', 'Hindi', 'Hungarian',
  'Indonesian', 'Italian', 'Japanese', 'Korean', 'Malay',
  'Norwegian', 'Polish', 'Portuguese', 'Romanian', 'Russian',
  'Spanish', 'Swedish', 'Thai', 'Turkish', 'Ukrainian', 'Vietnamese',
];

const BANKS = [
  { id: 'MB', name: 'MB Bank' }, { id: 'VCB', name: 'Vietcombank' },
  { id: 'ACB', name: 'ACB' }, { id: 'TCB', name: 'Techcombank' },
  { id: 'VPB', name: 'VPBank' }, { id: 'TPB', name: 'TPBank' },
  { id: 'BIDV', name: 'BIDV' }, { id: 'VTB', name: 'VietinBank' },
  { id: 'SCB', name: 'Sacombank' },
];

interface ToastState { message: string; type: 'success' | 'error'; }

function SecretField({ label, value, onChange, placeholder, hint }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string; hint?: string;
}) {
  const [show, setShow] = useState(false);
  return (
    <div>
      <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>{label}</label>
      <div style={{ position: 'relative' }}>
        <input
          type={show ? 'text' : 'password'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="dark-input"
          style={{ paddingRight: 40 }}
        />
        <button
          type="button"
          onClick={() => setShow(!show)}
          style={{ position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', color: '#7c8fac', cursor: 'pointer', fontSize: 13 }}
        >
          {show ? '🙈' : '👁'}
        </button>
      </div>
      {hint && <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 4 }}>{hint}</p>}
    </div>
  );
}

function TextField({ label, value, onChange, placeholder, hint }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string; hint?: string;
}) {
  return (
    <div>
      <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>{label}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="dark-input"
      />
      {hint && <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 4 }}>{hint}</p>}
    </div>
  );
}

function StatusDot({ configured }: { configured: boolean }) {
  return (
    <span style={{
      display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
      background: configured ? '#13deb9' : '#333f55',
      marginRight: 8,
    }} />
  );
}

export default function SettingsPage() {
  const defaults: Settings = {
    environment: 'testing', trial_limit: 3, token_expiry_minutes: 15, refresh_token_expiry_days: 90, max_input_length: 3000,
    supported_languages: ALL_LANGUAGES.join(','),
    stripe_secret_key: '', stripe_webhook_secret: '', stripe_mode: 'test',
    paypal_client_id: '', paypal_client_secret: '', paypal_mode: 'sandbox',
    momo_partner_code: '', momo_access_key: '', momo_secret_key: '', momo_mode: 'sandbox',
    vietqr_bank_id: 'MB', vietqr_account_number: '', vietqr_account_name: '',
    casso_api_key: '', sepay_api_key: '', sepay_mode: 'sandbox',
    resend_api_key: '', email_from: 'DraftRight <noreply@draftright.info>',
    google_client_id: '', google_client_secret: '',
    apple_client_id: '', apple_team_id: '', apple_key_id: '',
    client_log_level: 'info',
  };

  const [settings, setSettings] = useState<Settings>(defaults);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);
  const [activeTab, setActiveTab] = useState<'general' | 'payment' | 'auth'>('general');
  const [testEmailTo, setTestEmailTo] = useState('tannpv@gmail.com');
  const [sendingTest, setSendingTest] = useState(false);

  async function sendTestEmail() {
    setSendingTest(true);
    try {
      // Save first so backend reads the latest key/from from the DB.
      await apiFetch('/admin/settings', { method: 'PATCH', body: JSON.stringify(settings) });
      await apiFetch('/admin/settings/test-email', { method: 'POST', body: JSON.stringify({ to: testEmailTo }) });
      setToast({ message: `Test email sent to ${testEmailTo}.`, type: 'success' });
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Test email failed', type: 'error' });
    } finally {
      setSendingTest(false);
    }
  }

  useEffect(() => { loadSettings(); }, []);

  async function loadSettings() {
    setLoading(true);
    try {
      const data = await apiFetch('/admin/settings') as Settings;
      setSettings({ ...defaults, ...data });
    } catch { /* use defaults */ }
    finally { setLoading(false); }
  }

  async function saveSettings() {
    setSaving(true);
    try {
      await apiFetch('/admin/settings', { method: 'PATCH', body: JSON.stringify(settings) });
      setToast({ message: 'Settings saved successfully.', type: 'success' });
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to save settings', type: 'error' });
    } finally { setSaving(false); }
  }

  const set = (key: string) => (val: string | number | boolean) => setSettings({ ...settings, [key]: val });

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', padding: 48 }}><div style={{ color: '#7c8fac' }}>Loading settings...</div></div>;

  const isLive = settings.environment === 'live';
  const tabs = [
    { key: 'general' as const, label: 'General', icon: '⚙️' },
    { key: 'payment' as const, label: 'Payment', icon: '💳' },
    { key: 'auth' as const, label: 'Auth & Social', icon: '🔑' },
  ];

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Settings</h1>
        <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>System configuration, payment providers, and authentication</p>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 4, marginBottom: 24, borderBottom: '1px solid #333f55', paddingBottom: 0 }}>
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => setActiveTab(t.key)}
            style={{
              padding: '10px 20px',
              border: 'none',
              borderBottom: activeTab === t.key ? '2px solid #5d87ff' : '2px solid transparent',
              background: 'none',
              color: activeTab === t.key ? '#eaeff4' : '#7c8fac',
              fontWeight: activeTab === t.key ? 600 : 400,
              fontSize: 14,
              cursor: 'pointer',
              fontFamily: 'inherit',
            }}
          >
            <span style={{ marginRight: 6 }}>{t.icon}</span>{t.label}
          </button>
        ))}
      </div>

      {/* ===== GENERAL TAB ===== */}
      {activeTab === 'general' && (
        <>
          {/* Environment Toggle */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 16 }}>Environment</h2>
            <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
              {(['testing', 'live'] as const).map(env => (
                <button key={env} onClick={() => set('environment')(env)} className="btn" style={{
                  flex: 1, padding: '16px 20px', borderRadius: 12,
                  border: settings.environment === env ? `2px solid ${env === 'live' ? '#13deb9' : '#5d87ff'}` : '2px solid #333f55',
                  background: settings.environment === env ? (env === 'live' ? 'rgba(19,222,185,0.1)' : 'rgba(93,135,255,0.1)') : '#2a3547',
                  color: settings.environment === env ? (env === 'live' ? '#13deb9' : '#5d87ff') : '#7c8fac',
                  fontWeight: 600, fontSize: 15, cursor: 'pointer', textAlign: 'center' as const,
                }}>
                  <div style={{ fontSize: 24, marginBottom: 4 }}>{env === 'testing' ? '🧪' : '🚀'}</div>
                  {env === 'testing' ? 'Testing' : 'Live'}
                  <div style={{ fontSize: 11, fontWeight: 400, marginTop: 4, opacity: 0.7 }}>
                    {env === 'testing' ? 'Relaxed limits, debug logging' : 'Production limits, secure'}
                  </div>
                </button>
              ))}
            </div>
            {isLive && <div style={{ background: 'rgba(255,174,31,0.1)', border: '1px solid rgba(255,174,31,0.2)', borderRadius: 8, padding: '10px 14px', color: '#ffae1f', fontSize: 13 }}>⚠️ Live mode applies strict rate limits and shorter token expiry.</div>}
          </div>

          {/* Logging & Diagnostics */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 8 }}>Logging &amp; Diagnostics</h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>Minimum severity that desktop &amp; mobile apps write to their local logs. Applied on each client's next health check (~30s).</p>
            <div style={{ maxWidth: 360 }}>
              <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Client log level</label>
              <select value={settings.client_log_level} onChange={e => set('client_log_level')(e.target.value)} className="dark-input">
                <option value="info">Info — everything (default)</option>
                <option value="warnings">Warnings — warnings + errors</option>
                <option value="errors">Errors only</option>
                <option value="off">Off — log nothing</option>
              </select>
              <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 4 }}>“Off” is an absolute kill-switch — clients stop writing logs entirely, including errors.</p>
            </div>
          </div>

          {/* Limits */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 16 }}>Limits</h2>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <div><label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Trial Rewrites/Day</label><input type="number" value={settings.trial_limit} onChange={e => set('trial_limit')(parseInt(e.target.value) || 3)} className="dark-input" min={1} /></div>
              <div><label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Max Input Length</label><input type="number" value={settings.max_input_length} onChange={e => set('max_input_length')(parseInt(e.target.value) || 3000)} className="dark-input" min={100} /></div>
            </div>
          </div>

          {/* Session */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 8 }}>Session</h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>How long users stay signed in. Access token is short-lived and silently refreshed; session lifetime is the maximum idle window before re-login is required.</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Access Token Lifetime (minutes)</label>
                <input type="number" value={settings.token_expiry_minutes} onChange={e => set('token_expiry_minutes')(parseInt(e.target.value) || 15)} className="dark-input" min={1} max={1440} />
                <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 4 }}>Default: 15. Auto-refreshed silently — users don't notice expiry.</p>
              </div>
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Session Lifetime (days)</label>
                <input type="number" value={settings.refresh_token_expiry_days} onChange={e => set('refresh_token_expiry_days')(parseInt(e.target.value) || 90)} className="dark-input" min={1} max={365} />
                <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 4 }}>Default: 90. Forces re-login after this many days of inactivity. Industry standard: 30–90.</p>
              </div>
            </div>
          </div>

          {/* Languages */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 8 }}>Translation Languages</h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>Default language is auto-detected from browser/region.</p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {ALL_LANGUAGES.map(lang => {
                const enabled = (settings.supported_languages || '').split(',').includes(lang);
                return <button key={lang} onClick={() => {
                  const cur = (settings.supported_languages || '').split(',').filter(Boolean);
                  const upd = enabled ? cur.filter(l => l !== lang) : [...cur, lang];
                  set('supported_languages')(upd.join(','));
                }} style={{ padding: '6px 12px', borderRadius: 8, border: enabled ? '1px solid #5d87ff' : '1px solid #333f55', background: enabled ? 'rgba(93,135,255,0.15)' : '#2a3547', color: enabled ? '#5d87ff' : '#7c8fac', fontSize: 12, fontWeight: 500, cursor: 'pointer' }}>{lang}</button>;
              })}
            </div>
            <div style={{ marginTop: 12, color: '#7c8fac', fontSize: 12 }}>{(settings.supported_languages || '').split(',').filter(Boolean).length} of {ALL_LANGUAGES.length} enabled</div>
          </div>
        </>
      )}

      {/* ===== PAYMENT TAB ===== */}
      {activeTab === 'payment' && (
        <>
          <p style={{ color: '#7c8fac', fontSize: 13, marginBottom: 20 }}>
            Each provider has its own <strong style={{ color: '#eaeff4' }}>mode</strong> (sandbox/test vs live). A pending
            payment whose provider is in sandbox/test can be completed with the “🧪 Simulate paid” button on the Payments page.
          </p>

          {/* Stripe */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.stripe_secret_key} />💳 Stripe
              <span style={{ marginLeft: 10, fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4,
                  background: settings.stripe_mode === 'live' ? 'rgba(19,222,185,0.12)' : 'rgba(255,174,31,0.12)',
                  color: settings.stripe_mode === 'live' ? '#13deb9' : '#ffae1f',
                  textTransform: 'uppercase' as const }}>
                {settings.stripe_mode}
              </span>
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>Credit/debit cards, Apple Pay, Google Pay</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16 }}>
              <SecretField label="Secret Key" value={settings.stripe_secret_key} onChange={set('stripe_secret_key')} placeholder="sk_live_..." hint="From Stripe Dashboard → Developers → API Keys" />
              <SecretField label="Webhook Secret" value={settings.stripe_webhook_secret} onChange={set('stripe_webhook_secret')} placeholder="whsec_..." hint="From Stripe Dashboard → Webhooks" />
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Mode</label>
                <select value={settings.stripe_mode} onChange={e => set('stripe_mode')(e.target.value)} className="dark-input">
                  <option value="test">Test</option>
                  <option value="live">Live</option>
                </select>
                <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 4 }}>Switch when keys are sk_live_/whsec_ from live mode.</p>
              </div>
            </div>
          </div>

          {/* PayPal */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.paypal_client_id} />🅿️ PayPal
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>PayPal payments</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <SecretField label="Client ID" value={settings.paypal_client_id} onChange={set('paypal_client_id')} placeholder="AX..." />
              <SecretField label="Client Secret" value={settings.paypal_client_secret} onChange={set('paypal_client_secret')} placeholder="EL..." />
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Mode</label>
                <select value={settings.paypal_mode} onChange={e => set('paypal_mode')(e.target.value)} className="dark-input">
                  <option value="sandbox">Sandbox (Testing)</option>
                  <option value="live">Live (Production)</option>
                </select>
              </div>
            </div>
          </div>

          {/* Momo */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.momo_partner_code} />💜 Momo
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>Momo e-wallet payments (Vietnam)</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <SecretField label="Partner Code" value={settings.momo_partner_code} onChange={set('momo_partner_code')} placeholder="MOMO..." />
              <SecretField label="Access Key" value={settings.momo_access_key} onChange={set('momo_access_key')} />
              <SecretField label="Secret Key" value={settings.momo_secret_key} onChange={set('momo_secret_key')} />
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Mode</label>
                <select value={settings.momo_mode} onChange={e => set('momo_mode')(e.target.value)} className="dark-input">
                  <option value="sandbox">Sandbox (Testing)</option>
                  <option value="live">Live (Production)</option>
                </select>
              </div>
            </div>
          </div>

          {/* VietQR / Bank */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.vietqr_account_number} />📱 VietQR / Bank Transfer
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>QR code payments via Vietnamese banks</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Bank</label>
                <select value={settings.vietqr_bank_id} onChange={e => set('vietqr_bank_id')(e.target.value)} className="dark-input">
                  {BANKS.map(b => <option key={b.id} value={b.id}>{b.name}</option>)}
                </select>
              </div>
              <TextField label="Account Number" value={settings.vietqr_account_number} onChange={set('vietqr_account_number')} placeholder="1234567890" />
              <TextField label="Account Name" value={settings.vietqr_account_name} onChange={set('vietqr_account_name')} placeholder="DRAFTRIGHT" hint="As shown on bank statement" />
            </div>
          </div>

          {/* Casso / SePay */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.casso_api_key || !!settings.sepay_api_key} />🔔 Auto-verification (Casso / SePay)
              <span style={{ marginLeft: 10, fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4,
                  background: settings.sepay_mode === 'live' ? 'rgba(19,222,185,0.12)' : 'rgba(255,174,31,0.12)',
                  color: settings.sepay_mode === 'live' ? '#13deb9' : '#ffae1f',
                  textTransform: 'uppercase' as const }}>
                {settings.sepay_mode}
              </span>
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>Automatic bank transfer verification — configure one or both</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16 }}>
              <SecretField label="Casso API Key" value={settings.casso_api_key} onChange={set('casso_api_key')} placeholder="casso_..." hint="From casso.vn dashboard" />
              <SecretField label="SePay API Key" value={settings.sepay_api_key} onChange={set('sepay_api_key')} placeholder="sepay_..." hint="From sepay.vn dashboard" />
              <div>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>SePay Mode</label>
                <select value={settings.sepay_mode} onChange={e => set('sepay_mode')(e.target.value)} className="dark-input">
                  <option value="sandbox">Sandbox</option>
                  <option value="live">Live</option>
                </select>
              </div>
            </div>
          </div>

          {/* Resend (transactional email) */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.resend_api_key} />📧 Resend (transactional email)
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>Sends verification, renewal reminder, and payment-failed emails. Domain must be verified in Resend before sending.</p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
              <SecretField label="API Key" value={settings.resend_api_key} onChange={set('resend_api_key')} placeholder="re_..." hint="From resend.com dashboard" />
              <TextField label="From Address" value={settings.email_from} onChange={set('email_from')} placeholder="DraftRight <noreply@draftright.info>" hint="Domain must be DKIM-verified in Resend" />
            </div>
            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', paddingTop: 12, borderTop: '1px solid #333f55' }}>
              <div style={{ flex: 1 }}>
                <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Send test email to</label>
                <input
                  type="email"
                  value={testEmailTo}
                  onChange={(e) => setTestEmailTo(e.target.value)}
                  placeholder="you@example.com"
                  className="dark-input"
                />
              </div>
              <button
                onClick={sendTestEmail}
                disabled={sendingTest || !settings.resend_api_key || !testEmailTo.includes('@')}
                className="btn btn-primary"
                style={{ minWidth: 130, opacity: !settings.resend_api_key ? 0.5 : 1 }}
              >
                {sendingTest ? 'Sending...' : 'Send test'}
              </button>
            </div>
            {!settings.resend_api_key && (
              <p style={{ color: '#7c8fac', fontSize: 11, marginTop: 8 }}>
                Enter an API key first. Test sends after auto-save.
              </p>
            )}
          </div>
        </>
      )}

      {/* ===== AUTH TAB ===== */}
      {activeTab === 'auth' && (
        <>
          {/* Google */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.google_client_id} />
              <svg width="16" height="16" viewBox="0 0 24 24" style={{ verticalAlign: 'middle', marginRight: 6 }}>
                <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/>
                <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
                <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
              </svg>
              Google Sign-In
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>For web and mobile Google login. Get credentials from <a href="https://console.cloud.google.com/apis/credentials" target="_blank" rel="noopener" style={{ color: '#5d87ff' }}>Google Cloud Console</a></p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <TextField label="Client ID" value={settings.google_client_id} onChange={set('google_client_id')} placeholder="123456-abc.apps.googleusercontent.com" hint="OAuth 2.0 Client ID (Web application)" />
              <SecretField label="Client Secret" value={settings.google_client_secret} onChange={set('google_client_secret')} placeholder="GOCSPX-..." hint="Only needed for server-side auth" />
            </div>
          </div>

          {/* Apple */}
          <div className="card" style={{ marginBottom: 24 }}>
            <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              <StatusDot configured={!!settings.apple_client_id} />
              <span style={{ fontSize: 16, marginRight: 6 }}></span>
              Apple Sign-In
            </h2>
            <p style={{ color: '#7c8fac', fontSize: 12, marginBottom: 16 }}>For iOS and web Apple login. Get credentials from <a href="https://developer.apple.com/account/resources/identifiers/list" target="_blank" rel="noopener" style={{ color: '#5d87ff' }}>Apple Developer Portal</a></p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <TextField label="Service ID" value={settings.apple_client_id} onChange={set('apple_client_id')} placeholder="com.draftright.signin" hint="Services ID for Sign in with Apple" />
              <TextField label="Team ID" value={settings.apple_team_id} onChange={set('apple_team_id')} placeholder="ABC123XYZ" hint="From Apple Developer account" />
              <SecretField label="Key ID" value={settings.apple_key_id} onChange={set('apple_key_id')} placeholder="ABC123" hint="Sign in with Apple private key ID" />
            </div>
          </div>
        </>
      )}

      {/* Save */}
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button onClick={saveSettings} disabled={saving} className="btn btn-primary" style={{ minWidth: 140 }}>
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
      </div>

      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  );
}
