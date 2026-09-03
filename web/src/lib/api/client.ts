// ============================================================
// API Client — Core HTTP layer
// Single source of truth for base URL, headers, auth token
// ============================================================

import { GuestSession, SessionUser } from '@/lib/types';

const TOKEN_LOCAL_STORAGE_KEY = 'dongdo_token';
const AUTH_COOKIE_NAME = 'dongdo_auth_token';

function getApiBase(): string {
  if (typeof window === 'undefined') {
    // SSR: use default
    return 'http://localhost:8080';
  }
  if (process.env.NEXT_PUBLIC_API_URL) {
    return process.env.NEXT_PUBLIC_API_URL;
  }
  if (window.location.port === '3000') {
    return `${window.location.protocol}//${window.location.hostname}:8080`;
  }
  return window.location.origin;
}

export const API_BASE = getApiBase();

export class ApiError extends Error {
  constructor(message: string, public status: number, public body?: any) {
    super(message);
    this.name = 'ApiError';
  }
}

export function getAuthToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(TOKEN_LOCAL_STORAGE_KEY);
}

export function setAuthToken(token: string | null): void {
  if (typeof window === 'undefined') return;
  if (token) {
    localStorage.setItem(TOKEN_LOCAL_STORAGE_KEY, token);
    document.cookie = `${AUTH_COOKIE_NAME}=${encodeURIComponent(token)};path=/;max-age=${7 * 24 * 60 * 60};samesite=strict`;
  } else {
    localStorage.removeItem(TOKEN_LOCAL_STORAGE_KEY);
    document.cookie = `${AUTH_COOKIE_NAME}=;path=/;max-age=0;samesite=strict`;
  }
}

export function setAuthUser(user: SessionUser | null): void {
  if (typeof window === 'undefined') return;
  if (user) {
    localStorage.setItem('dongdo_user', JSON.stringify(user));
  } else {
    localStorage.removeItem('dongdo_user');
  }
}

function buildHeaders(token?: string, isFormData?: boolean): HeadersInit {
  const headers: HeadersInit = {};
  if (!isFormData) {
    headers['Content-Type'] = 'application/json';
  }
  const authToken = token ?? getAuthToken();
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }
  return headers;
}

async function parseResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let errorMsg = `Lỗi HTTP ${res.status}`;
    let body: any = null;
    try {
      body = await res.json();
      errorMsg = body?.detail || body?.error || body?.message || errorMsg;
    } catch {
      try {
        errorMsg = (await res.text()) || errorMsg;
      } catch {
        /* noop */
      }
    }
    throw new ApiError(errorMsg, res.status, body);
  }
  // 204 No Content
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface RequestOptions {
  token?: string;
  isFormData?: boolean;
  signal?: AbortSignal;
}

export const apiClient = {
  async get<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'GET',
      headers: buildHeaders(opts.token, opts.isFormData),
      signal: opts.signal,
    });
    return parseResponse<T>(res);
  },

  async post<T>(path: string, body?: any, opts: RequestOptions = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'POST',
      headers: buildHeaders(opts.token, opts.isFormData),
      body: body !== undefined ? (opts.isFormData ? body : JSON.stringify(body)) : undefined,
      signal: opts.signal,
    });
    return parseResponse<T>(res);
  },

  async put<T>(path: string, body?: any, opts: RequestOptions = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'PUT',
      headers: buildHeaders(opts.token, opts.isFormData),
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: opts.signal,
    });
    return parseResponse<T>(res);
  },

  async patch<T>(path: string, body?: any, opts: RequestOptions = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'PATCH',
      headers: buildHeaders(opts.token, opts.isFormData),
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: opts.signal,
    });
    return parseResponse<T>(res);
  },

  async delete<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'DELETE',
      headers: buildHeaders(opts.token, opts.isFormData),
      signal: opts.signal,
    });
    return parseResponse<T>(res);
  },

  async uploadFile<T>(path: string, file: File, fieldName = 'file'): Promise<T> {
    const formData = new FormData();
    formData.append(fieldName, file);
    return this.post<T>(path, formData, { isFormData: true });
  },
};

export type { GuestSession };
