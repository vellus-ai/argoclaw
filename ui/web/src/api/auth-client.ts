/** Auth API client for pre-authentication endpoints (login, register, refresh, logout). */

export interface AuthUser {
  id: string;
  email: string;
  display_name: string;
  role: string;
  status: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: AuthUser;
}

export interface AuthError {
  error: string;
  code?: string;
}

async function authFetch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const err: AuthError = await res.json().catch(() => ({ error: res.statusText }));
    throw new AuthApiError(res.status, err.error, err.code);
  }

  return res.json() as Promise<T>;
}

export class AuthApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly code?: string,
  ) {
    super(message);
    this.name = "AuthApiError";
  }
}

export function login(email: string, password: string): Promise<AuthResponse> {
  return authFetch("/v1/auth/login", { email, password });
}

export function register(
  email: string,
  password: string,
  displayName?: string,
): Promise<AuthResponse> {
  return authFetch("/v1/auth/register", {
    email,
    password,
    ...(displayName ? { display_name: displayName } : {}),
  });
}

export function refresh(refreshToken: string): Promise<AuthResponse> {
  return authFetch("/v1/auth/refresh", { refresh_token: refreshToken });
}

export function changePassword(
  currentPassword: string,
  newPassword: string,
  accessToken: string,
): Promise<AuthResponse> {
  return authFetchWithAuth<AuthResponse>(
    "/v1/auth/change-password",
    { current_password: currentPassword, new_password: newPassword },
    accessToken,
  );
}

async function authFetchWithAuth<T>(path: string, body: unknown, accessToken: string): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const err: AuthError = await res.json().catch(() => ({ error: res.statusText }));
    throw new AuthApiError(res.status, err.error, err.code);
  }

  return res.json() as Promise<T>;
}

export async function logout(refreshToken: string, accessToken: string): Promise<void> {
  await fetch("/v1/auth/logout", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}
