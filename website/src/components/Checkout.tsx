import { useCallback, useEffect, useRef, useState } from 'react';

const API =
  (import.meta.env.PUBLIC_API_URL as string | undefined) || 'http://localhost:3000';

type PlanId = string;

interface Plan {
  id: PlanId;
  name: string;
  price: number;
  currency: string;
  period: 'month' | 'year';
  badge: string | null;
}

// Plans come from the backend (GET /plans) so prices + IDs never go stale.
interface ApiPlan {
  id: string;
  name: string;
  price_cents: number;
  currency: string | null;
  billing_period: 'none' | 'monthly' | 'yearly';
  is_active: boolean;
}

const toPlan = (p: ApiPlan): Plan => ({
  id: p.id,
  name: p.name,
  price: p.price_cents,
  currency: (p.currency || 'VND').toUpperCase(),
  period: p.billing_period === 'yearly' ? 'year' : 'month',
  badge: p.billing_period === 'yearly' ? 'Best value' : null,
});

interface Method {
  key: 'stripe' | 'vietqr' | 'bank_transfer';
  icon: string;
  label: string;
  sub: string;
}

const METHODS: Method[] = [
  { key: 'stripe', icon: '💳', label: 'Credit/Debit Card', sub: 'Visa, Mastercard, Apple Pay, Google Pay' },
  { key: 'vietqr', icon: '📱', label: 'VietQR', sub: 'Scan QR with any Vietnamese banking app' },
  { key: 'bank_transfer', icon: '🏦', label: 'Bank Transfer', sub: 'Manual transfer to MB Bank' },
];

const formatVnd = (n: number) =>
  new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(n);

// price_cents is whole VND for VND plans (99000 = 99.000₫) but minor units for
// currencies with cents (499 USD = $4.99). Format by each plan's own currency.
const formatPrice = (priceCents: number, currency: string) => {
  const c = (currency || 'VND').toUpperCase();
  if (c === 'VND') return formatVnd(priceCents);
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: c }).format(priceCents / 100);
};

type Step = 'plan' | 'method' | 'auth' | 'processing' | 'success';
type AuthMode = 'login' | 'register';

interface CheckoutResponse {
  redirect_url?: string;
  qr_data?: string;
  bank_info?: {
    bank_name: string;
    account_number: string;
    account_name: string;
    amount: number;
    reference_code: string;
  };
  payment?: { reference_code?: string };
}

const STEP_ORDER: Step[] = ['plan', 'method', 'auth', 'processing', 'success'];
const stepIndex = (s: Step) => STEP_ORDER.indexOf(s);
const stepLabel = (s: Step) =>
  ({ plan: 'Plan', method: 'Payment', auth: 'Account', processing: 'Confirm' }[s as Exclude<Step, 'success'>] ?? s);

declare global {
  interface Window {
    google?: any;
  }
}

export default function Checkout() {
  const [step, setStep] = useState<Step>('plan');
  const [planId, setPlanId] = useState<PlanId | null>(null);
  const [methodKey, setMethodKey] = useState<Method['key'] | null>(null);
  const [authMode, setAuthMode] = useState<AuthMode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [authError, setAuthError] = useState('');
  const [busy, setBusy] = useState(false);
  const [checkoutData, setCheckoutData] = useState<CheckoutResponse | null>(null);
  const [topError, setTopError] = useState('');
  const [paymentStatus, setPaymentStatus] = useState('');
  const [plans, setPlans] = useState<Plan[]>([]);
  const [plansError, setPlansError] = useState('');
  // null until loaded → show all; otherwise only these methods.
  const [enabledMethods, setEnabledMethods] = useState<string[] | null>(null);
  const visibleMethods = enabledMethods ? METHODS.filter((m) => enabledMethods.includes(m.key)) : METHODS;
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const selectedPlan = plans.find((p) => p.id === planId);

  const getToken = () =>
    typeof window !== 'undefined' ? localStorage.getItem('dr_access_token') : null;

  // Load active plans + enabled payment methods from the backend.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(`${API}/plans`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data: ApiPlan[] = await res.json();
        if (cancelled) return;
        const paid = data.filter((p) => p.billing_period !== 'none').map(toPlan);
        setPlans(paid);
        const wanted = new URLSearchParams(window.location.search).get('plan');
        if (wanted && paid.some((p) => p.id === wanted)) setPlanId(wanted);
      } catch (err) {
        if (!cancelled) setPlansError(err instanceof Error ? err.message : 'Could not load plans');
      }
    })();
    // Enabled payment methods (admin-controlled). On failure, show all.
    (async () => {
      try {
        const res = await fetch(`${API}/payment/methods`);
        if (!res.ok) return;
        const data: { methods?: string[] } = await res.json();
        if (!cancelled && Array.isArray(data.methods)) setEnabledMethods(data.methods);
      } catch {
        /* keep default (all) */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(
    () => () => {
      if (pollRef.current) clearInterval(pollRef.current);
    },
    [],
  );

  const goBack = () => {
    if (step === 'method') setStep('plan');
    else if (step === 'auth') setStep('method');
    else if (step === 'processing') {
      if (pollRef.current) clearInterval(pollRef.current);
      setStep('method');
    }
  };

  const submitAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError('');
    setBusy(true);
    try {
      const path = authMode === 'login' ? '/auth/login' : '/auth/register';
      const body =
        authMode === 'login' ? { email, password } : { email, password, name };
      const res = await fetch(`${API}${path}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => null);
        throw new Error(err?.message || 'Authentication failed');
      }
      const data = await res.json();
      localStorage.setItem('dr_access_token', data.access_token);
      if (data.refresh_token) localStorage.setItem('dr_refresh_token', data.refresh_token);
      void startCheckout(data.access_token);
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setBusy(false);
    }
  };

  const socialLogin = async (provider: string, idToken: string, extra: Record<string, unknown>) => {
    setAuthError('');
    setBusy(true);
    try {
      const res = await fetch(`${API}/auth/social`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider, id_token: idToken, ...extra }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => null);
        throw new Error(err?.message || 'Social login failed');
      }
      const data = await res.json();
      localStorage.setItem('dr_access_token', data.access_token);
      if (data.refresh_token) localStorage.setItem('dr_refresh_token', data.refresh_token);
      void startCheckout(data.access_token);
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setBusy(false);
    }
  };

  const onGoogle = () => {
    const clientId =
      (import.meta.env.PUBLIC_GOOGLE_CLIENT_ID as string | undefined) ||
      '22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com';
    const s = document.createElement('script');
    s.src = 'https://accounts.google.com/gsi/client';
    s.onload = () => {
      window.google.accounts.id.initialize({
        client_id: clientId,
        callback: (resp: { credential: string }) => {
          void socialLogin('google', resp.credential, {});
        },
      });
      window.google.accounts.id.prompt();
    };
    document.head.appendChild(s);
  };

  const onFacebook = () => {
    setAuthError('Facebook login coming soon. Please use email/password or Google.');
  };

  const startCheckout = useCallback(
    async (overrideToken?: string) => {
      setStep('processing');
      setTopError('');
      const token = overrideToken || getToken();
      if (!token || !planId || !methodKey) return;
      try {
        const res = await fetch(`${API}/payment/checkout`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ plan_id: planId, method: methodKey }),
        });
        if (!res.ok) {
          const msg = (await res.json().catch(() => null))?.message || 'Checkout failed';
          setTopError(msg);
          setStep('method');
          return;
        }
        const data: CheckoutResponse = await res.json();
        setCheckoutData(data);
        if (methodKey === 'stripe' && data.redirect_url) {
          window.location.href = data.redirect_url;
          return;
        }
        if (data.payment?.reference_code) {
          pollRef.current = setInterval(async () => {
            try {
              const r = await fetch(`${API}/payment/status/${data.payment!.reference_code}`);
              if (r.ok) {
                const s = await r.json();
                setPaymentStatus(s.status);
                if (s.status === 'completed') {
                  if (pollRef.current) clearInterval(pollRef.current);
                  setStep('success');
                }
              }
            } catch {
              /* ignore */
            }
          }, 5000);
        }
      } catch (err) {
        setTopError(err instanceof Error ? err.message : 'Checkout failed');
        setStep('method');
      }
    },
    [planId, methodKey],
  );

  const onContinue = () => {
    if (step === 'plan' && planId) setStep('method');
    else if (step === 'method' && methodKey) {
      if (getToken()) void startCheckout();
      else setStep('auth');
    }
  };

  const renderPlan = () => (
    <div>
      <h2 className="text-2xl font-bold text-white mb-2">Choose your plan</h2>
      <p className="text-gray-400 mb-8">Unlock unlimited rewrites and all platforms.</p>
      {plansError && <p className="mb-4 text-sm text-red-400">Could not load plans: {plansError}</p>}
      {plans.length === 0 && !plansError && <p className="text-gray-500">Loading plans…</p>}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {plans.map((p) => (
          <button
            key={p.id}
            onClick={() => setPlanId(p.id)}
            className={`relative rounded-xl border-2 p-6 text-left transition-all ${
              planId === p.id
                ? 'border-brand-400 bg-brand-400/10'
                : 'border-dark-border bg-dark-card hover:border-gray-500'
            }`}
          >
            {p.badge && (
              <span className="absolute -top-3 right-4 rounded-full bg-emerald-500 px-3 py-0.5 text-xs font-semibold text-white">
                {p.badge}
              </span>
            )}
            <div className="text-lg font-semibold text-white">{p.name}</div>
            <div className="mt-2">
              <span className="text-3xl font-extrabold text-white">{formatPrice(p.price, p.currency)}</span>
              <span className="text-gray-500 ml-1">/ {p.period}</span>
            </div>
            {p.period === 'year' && (
              <div className="mt-1 text-sm text-emerald-400">
                {formatPrice(Math.round(p.price / 12), p.currency)}/month
              </div>
            )}
          </button>
        ))}
      </div>
    </div>
  );

  const renderMethod = () => (
    <div>
      <h2 className="text-2xl font-bold text-white mb-2">Payment method</h2>
      <p className="text-gray-400 mb-8">
        {selectedPlan?.name} — {selectedPlan ? formatPrice(selectedPlan.price, selectedPlan.currency) : ''}
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {visibleMethods.map((m) => (
          <button
            key={m.key}
            onClick={() => setMethodKey(m.key)}
            className={`rounded-xl border-2 p-5 text-left transition-all ${
              methodKey === m.key
                ? 'border-brand-400 bg-brand-400/10'
                : 'border-dark-border bg-dark-card hover:border-gray-500'
            }`}
          >
            <div className="text-2xl mb-2">{m.icon}</div>
            <div className="font-semibold text-white">{m.label}</div>
            <div className="text-sm text-gray-400 mt-1">{m.sub}</div>
          </button>
        ))}
      </div>
    </div>
  );

  const renderAuth = () => (
    <div className="max-w-md mx-auto">
      <h2 className="text-2xl font-bold text-white mb-2">
        {authMode === 'login' ? 'Log in' : 'Create account'}
      </h2>
      <p className="text-gray-400 mb-8">
        {authMode === 'login'
          ? 'Log in to complete your purchase.'
          : 'Create an account to get started.'}
      </p>
      <form onSubmit={submitAuth} className="space-y-4">
        {authMode === 'register' && (
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              className="w-full rounded-lg border border-dark-border bg-dark-card px-4 py-2.5 text-white placeholder-gray-500 focus:border-brand-400 focus:outline-none focus:ring-1 focus:ring-brand-400"
              placeholder="Your name"
            />
          </div>
        )}
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-1">Email</label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            className="w-full rounded-lg border border-dark-border bg-dark-card px-4 py-2.5 text-white placeholder-gray-500 focus:border-brand-400 focus:outline-none focus:ring-1 focus:ring-brand-400"
            placeholder="you@example.com"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-1">Password</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={6}
            className="w-full rounded-lg border border-dark-border bg-dark-card px-4 py-2.5 text-white placeholder-gray-500 focus:border-brand-400 focus:outline-none focus:ring-1 focus:ring-brand-400"
            placeholder="••••••••"
          />
        </div>
        {authError && <p className="text-red-400 text-sm">{authError}</p>}
        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-full bg-brand-400 px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-500 transition-colors disabled:opacity-50"
        >
          {busy
            ? 'Please wait...'
            : authMode === 'login'
              ? 'Log in & Pay'
              : 'Create Account & Pay'}
        </button>
      </form>

      <div className="flex items-center gap-4 my-6">
        <div className="flex-1 h-px bg-dark-border" />
        <span className="text-sm text-gray-500">or</span>
        <div className="flex-1 h-px bg-dark-border" />
      </div>

      <div className="space-y-3">
        <button
          type="button"
          onClick={onGoogle}
          disabled={busy}
          className="w-full flex items-center justify-center gap-3 rounded-lg border border-dark-border bg-white px-4 py-2.5 text-sm font-medium text-gray-800 hover:bg-gray-100 transition-colors disabled:opacity-50"
        >
          <svg width="18" height="18" viewBox="0 0 24 24">
            <path
              d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
              fill="#4285F4"
            />
            <path
              d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
              fill="#34A853"
            />
            <path
              d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
              fill="#FBBC05"
            />
            <path
              d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
              fill="#EA4335"
            />
          </svg>
          Continue with Google
        </button>
        <button
          type="button"
          onClick={onFacebook}
          disabled={busy}
          className="w-full flex items-center justify-center gap-3 rounded-lg bg-[#1877F2] px-4 py-2.5 text-sm font-medium text-white hover:bg-[#166FE5] transition-colors disabled:opacity-50"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="white">
            <path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z" />
          </svg>
          Continue with Facebook
        </button>
      </div>

      <p className="mt-6 text-center text-sm text-gray-400">
        {authMode === 'login' ? "Don't have an account? " : 'Already have an account? '}
        <button
          type="button"
          onClick={() => {
            setAuthMode(authMode === 'login' ? 'register' : 'login');
            setAuthError('');
          }}
          className="text-brand-400 hover:underline"
        >
          {authMode === 'login' ? 'Register' : 'Log in'}
        </button>
      </p>
    </div>
  );

  const renderProcessing = () => {
    if (methodKey === 'stripe' || methodKey === 'paypal') {
      return (
        <div className="text-center py-12">
          <Spinner />
          <p className="mt-4 text-lg text-white">
            Redirecting to{' '}
            {methodKey === 'stripe' ? 'Stripe' : methodKey === 'momo' ? 'Momo' : 'PayPal'}...
          </p>
        </div>
      );
    }
    if (!checkoutData) {
      return (
        <div className="text-center py-12">
          <Spinner />
          <p className="mt-4 text-lg text-white">Processing...</p>
        </div>
      );
    }
    const bank = checkoutData.bank_info;
    return (
      <div className="max-w-lg mx-auto text-center">
        <h2 className="text-2xl font-bold text-white mb-6">Complete your payment</h2>
        {checkoutData.qr_data && (
          <div className="mb-6 inline-block rounded-xl bg-white p-4">
            <img
              src={checkoutData.qr_data}
              alt="Payment QR code"
              className="w-56 h-56 object-contain"
            />
          </div>
        )}
        {bank && (
          <div className="rounded-xl border border-dark-border bg-dark-card p-6 text-left space-y-3 mb-6">
            <Row label="Bank" value={bank.bank_name} />
            <Row label="Account Number" value={bank.account_number} />
            <Row label="Account Name" value={bank.account_name} />
            <Row label="Amount" value={formatVnd(bank.amount)} />
            <Row label="Reference Code" value={bank.reference_code} highlight />
          </div>
        )}
        <div className="flex items-center justify-center gap-2 text-gray-400">
          <Spinner small />
          <span>Waiting for payment...</span>
        </div>
        {paymentStatus && paymentStatus !== 'completed' && (
          <p className="mt-2 text-sm text-gray-500">Status: {paymentStatus}</p>
        )}
      </div>
    );
  };

  const renderSuccess = () => (
    <div className="text-center py-12">
      <div className="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-emerald-500/20">
        <svg
          className="h-10 w-10 text-emerald-400 animate-[scale-in_0.3s_ease-out]"
          fill="none"
          viewBox="0 0 24 24"
          strokeWidth={2.5}
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
        </svg>
      </div>
      <h2 className="text-2xl font-bold text-white mb-2">Payment successful!</h2>
      <p className="text-gray-400 mb-8">Your Pro plan is now active.</p>
      <a
        href="/download"
        className="inline-block rounded-full bg-brand-400 px-8 py-3 text-sm font-semibold text-white hover:bg-brand-500 transition-colors"
      >
        Download the app
      </a>
    </div>
  );

  const canContinue =
    (step === 'plan' && !!planId) || (step === 'method' && !!methodKey);

  return (
    <section className="mx-auto max-w-2xl px-6 pb-24">
      <div className="mb-10 flex items-center justify-center gap-2 text-sm text-gray-500">
        {(['plan', 'method', 'auth', 'processing'] as Step[]).map((s, i) => (
          <span key={s} className="flex items-center gap-2">
            {i > 0 && <span className="text-gray-600">/</span>}
            <span
              className={
                step === s
                  ? 'text-brand-400 font-semibold'
                  : step === 'success' || stepIndex(step) > i
                    ? 'text-gray-300'
                    : ''
              }
            >
              {stepLabel(s)}
            </span>
          </span>
        ))}
      </div>

      {step === 'plan' && renderPlan()}
      {step === 'method' && renderMethod()}
      {step === 'auth' && renderAuth()}
      {step === 'processing' && renderProcessing()}
      {step === 'success' && renderSuccess()}

      {(step === 'plan' || step === 'method' || step === 'processing') && step !== ('success' as Step) && (
        <div className="mt-8 flex items-center justify-between">
          <button
            onClick={goBack}
            className={`text-sm text-gray-400 hover:text-white transition-colors ${
              step === 'plan' ? 'invisible' : ''
            }`}
          >
            ← Back
          </button>
          {canContinue && (
            <button
              onClick={onContinue}
              className="rounded-full bg-brand-400 px-8 py-2.5 text-sm font-semibold text-white hover:bg-brand-500 transition-colors"
            >
              Continue →
            </button>
          )}
        </div>
      )}

      {step === 'auth' && (
        <div className="mt-8">
          <button
            onClick={goBack}
            className="text-sm text-gray-400 hover:text-white transition-colors"
          >
            ← Back
          </button>
        </div>
      )}

      {topError && <p className="mt-4 text-center text-red-400 text-sm">{topError}</p>}
    </section>
  );
}

function Row({
  label,
  value,
  highlight,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <div className="flex justify-between">
      <span className="text-gray-400 text-sm">{label}</span>
      <span
        className={`text-sm font-medium ${
          highlight ? 'text-brand-400 font-mono' : 'text-white'
        }`}
      >
        {value}
      </span>
    </div>
  );
}

function Spinner({ small }: { small?: boolean }) {
  const size = small ? 'h-5 w-5' : 'h-8 w-8';
  return (
    <div
      className={`${size} animate-spin rounded-full border-2 border-gray-600 border-t-brand-400 mx-auto`}
    />
  );
}
