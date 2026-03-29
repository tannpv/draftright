import { apiFetch } from './api';

interface LoginResponse {
  token: string;
  user?: {
    email: string;
    role: string;
  };
}

export async function login(email: string, password: string): Promise<void> {
  const data = await apiFetch('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  }) as LoginResponse;

  if (!data.token) {
    throw new Error('No token received');
  }

  localStorage.setItem('token', data.token);

  if (data.user?.email) {
    localStorage.setItem('adminEmail', data.user.email);
  }
}

export function logout(): void {
  localStorage.removeItem('token');
  localStorage.removeItem('adminEmail');
  window.location.href = '/login';
}

export function getToken(): string | null {
  return localStorage.getItem('token');
}

export function isAuthenticated(): boolean {
  return !!localStorage.getItem('token');
}

export function getAdminEmail(): string {
  return localStorage.getItem('adminEmail') || 'Admin';
}
