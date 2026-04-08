const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

export async function apiFetch(path: string, options?: RequestInit): Promise<unknown> {
  const token = localStorage.getItem('token');
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers,
    },
  });

  if (res.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }

  const text = await res.text();
  return text ? JSON.parse(text) : null;
}

export async function verifyBackend(): Promise<'ok' | 'wrong_server' | 'unreachable'> {
  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';
  try {
    const res = await fetch(`${API_URL}/health`, {
      headers: { Accept: 'application/json' },
    });
    if (!res.ok) return 'unreachable';
    const data = await res.json();
    return data.app === 'draftright' ? 'ok' : 'wrong_server';
  } catch {
    return 'unreachable';
  }
}
