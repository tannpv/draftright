// Shared social-auth helpers for the marketing site. Single source of
// truth for the Google client id, the GSI script bootstrap, and the
// `/auth/social` exchange — reused by the login, signup, and checkout
// flows so none of them re-implement (or drift from) the others.

const API = (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';

/** Google OAuth web client id. Override per-env via PUBLIC_GOOGLE_CLIENT_ID. */
export const GOOGLE_CLIENT_ID =
  (import.meta.env.PUBLIC_GOOGLE_CLIENT_ID as string | undefined) ||
  '22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com';

const GSI_SRC = 'https://accounts.google.com/gsi/client';

export interface SocialAuthTokens {
  access_token: string;
  refresh_token?: string;
}

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: { client_id: string; callback: (resp: { credential: string }) => void }) => void;
          prompt: () => void;
        };
      };
    };
  }
}

let gsiLoad: Promise<void> | null = null;

/** Load the Google Identity Services script exactly once, then resolve. */
function loadGsi(): Promise<void> {
  if (window.google?.accounts?.id) return Promise.resolve();
  if (gsiLoad) return gsiLoad;
  gsiLoad = new Promise((resolve, reject) => {
    const s = document.createElement('script');
    s.src = GSI_SRC;
    s.async = true;
    s.onload = () => resolve();
    s.onerror = () => {
      gsiLoad = null; // allow a retry on a later click
      reject(new Error('Could not load Google sign-in.'));
    };
    document.head.appendChild(s);
  });
  return gsiLoad;
}

/**
 * Bootstraps GSI (idempotently) and opens the Google One-Tap / account
 * chooser. `onCredential` receives the returned id_token.
 */
export async function triggerGoogleSignIn(onCredential: (idToken: string) => void): Promise<void> {
  await loadGsi();
  window.google!.accounts.id.initialize({
    client_id: GOOGLE_CLIENT_ID,
    callback: (resp) => onCredential(resp.credential),
  });
  window.google!.accounts.id.prompt();
}

/**
 * Exchange a provider id_token for DraftRight tokens via `/auth/social`
 * (creates the account on first sign-in, logs in thereafter) and persist
 * them. Returns the tokens so callers can chain (e.g. start a checkout).
 */
export async function postSocial(
  provider: string,
  idToken: string,
  extra: Record<string, unknown> = {},
): Promise<SocialAuthTokens> {
  const res = await fetch(`${API}/auth/social`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ provider, id_token: idToken, ...extra }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    const msg = Array.isArray(err?.message) ? err.message[0] : err?.message;
    throw new Error(msg || 'Social sign-in failed');
  }
  const data: SocialAuthTokens = await res.json();
  localStorage.setItem('dr_access_token', data.access_token);
  if (data.refresh_token) localStorage.setItem('dr_refresh_token', data.refresh_token);
  return data;
}
