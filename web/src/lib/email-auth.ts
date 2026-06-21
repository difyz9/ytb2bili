/**
 * 邮箱认证 API — 对接本项目 Go 后端的 /auth/* 路由。
 */

import { buildBackendUrl } from './backend-url';
import { DEFAULT_PROJECT_ID, getDefaultProjectHeaders } from './project-id';

const TOKEN_KEY = 'email_auth_token';
const SESSION_KEY = 'email_auth_session';
const REFRESH_SKEW_MS = 60 * 1000;

let pendingRefresh: Promise<StoredEmailSession | null> | null = null;

/**
 * 统一的用户信息结构（与 AuthContext 字段保持兼容）。
 * ytb2bili-api 返回 name/avatar/role，此处映射为 display_name/photo_url/provider。
 */
export interface UserInfoResponse {
  id: string;
  display_name: string;
  email: string;
  photo_url: string;
  /** 登录来源 */
  provider: 'email';
}

/** @deprecated 请使用 UserInfoResponse */
export type EmailUser = UserInfoResponse;

export interface LoginResponse {
  token: string;
  refreshToken?: string;
  expires_at: string;
  user: UserInfoResponse;
}

export interface VerifyEmailResponse {
  verified: boolean;
  login?: LoginResponse;
}

export interface StoredEmailSession {
  token: string;
  refreshToken?: string;
  expiresAt: string;
  user: UserInfoResponse;
}

export class EmailAuthRequestError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'EmailAuthRequestError';
    this.status = status;
  }
}

// ytb2bili-api 原始用户结构
interface MembershipUser {
  id: string;
  email: string;
  name: string;
  avatar: string | null;
  role: string;
}

// ytb2bili-api 原始登录/注册响应（后端包一层 { code, message, data }）
interface MembershipAuthResponse {
  code: number;
  message: string;
  data: {
    user?: MembershipUser;
    accessToken?: string;
    refreshToken?: string;
    expiresIn?: number;
    requiresEmailVerification?: boolean;
    email?: string;
    verificationExpiresIn?: number;
  };
}

interface MembershipMeResponse {
  code: number;
  data: {
    user: MembershipUser;
  };
}

function hasCompleteAuthPayload(
  data: MembershipAuthResponse['data'],
): data is {
  user: MembershipUser;
  accessToken: string;
  refreshToken?: string;
  expiresIn: number;
} {
  return Boolean(data.user && data.accessToken && typeof data.expiresIn === 'number');
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isMembershipUser(value: unknown): value is MembershipUser {
  return isRecord(value)
    && typeof value.id === 'string'
    && typeof value.email === 'string'
    && typeof value.name === 'string';
}

function extractLoginResponse(payload: unknown): LoginResponse | null {
  if (!isRecord(payload)) {
    return null;
  }

  const data = isRecord(payload.data) ? payload.data : payload;
  const accessToken = typeof data.accessToken === 'string' ? data.accessToken.trim() : '';
  const expiresInRaw = typeof data.expiresIn === 'number' ? data.expiresIn : Number(data.expiresIn);
  const user = isMembershipUser(data.user) ? data.user : null;

  if (!accessToken || !Number.isFinite(expiresInRaw) || !user) {
    return null;
  }

  return {
    token: accessToken,
    refreshToken: typeof data.refreshToken === 'string' ? data.refreshToken : undefined,
    expires_at: new Date(Date.now() + expiresInRaw * 1000).toISOString(),
    user: mapUser(user),
  };
}

/** 将 ytb2bili-api 用户字段映射为本项目通用结构 */
function mapUser(u: MembershipUser): UserInfoResponse {
  return {
    id: u.id,
    display_name: u.name,
    email: u.email,
    photo_url: u.avatar ?? '',
    provider: 'email',
  };
}

async function request<T>(path: string, init: RequestInit & { authToken?: string }): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...getDefaultProjectHeaders(),
    ...((init.headers as Record<string, string>) ?? {}),
  };
  if (init.authToken) {
    headers['Authorization'] = `Bearer ${init.authToken}`;
  }

  const res = await fetch(buildBackendUrl(path), {
    ...init,
    headers,
    credentials: 'include',
  });

  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new EmailAuthRequestError(
      res.status,
      (body as Record<string, string>)?.error ||
      (body as Record<string, string>)?.message ||
      `请求失败 (${res.status})`
    );
  }
  return body as T;
}

function buildAuthPath(path: string): string {
  return path;
}

/** 注册新账户，成功后自动返回 JWT（本地模式默认禁用） */
export async function emailRegister(email: string, password: string, name?: string): Promise<LoginResponse> {
  const resp = await request<MembershipAuthResponse>(buildAuthPath('/auth/register'), {
    method: 'POST',
    body: JSON.stringify({ email, password, name, projectId: DEFAULT_PROJECT_ID || undefined }),
  });

  if (!hasCompleteAuthPayload(resp.data)) {
    throw new Error(resp.message || '注册响应缺少登录信息');
  }

  return {
    token: resp.data.accessToken,
    refreshToken: resp.data.refreshToken,
    expires_at: new Date(Date.now() + resp.data.expiresIn * 1000).toISOString(),
    user: mapUser(resp.data.user),
  };
}

/** 邮箱登录，返回 JWT */
export async function emailLogin(email: string, password: string): Promise<LoginResponse> {
  const resp = await request<MembershipAuthResponse>(buildAuthPath('/auth/login'), {
    method: 'POST',
    body: JSON.stringify({ email, password, projectId: DEFAULT_PROJECT_ID || undefined }),
  });
  if (!hasCompleteAuthPayload(resp.data)) {
    throw new Error(resp.message || '登录响应缺少登录信息');
  }
  return {
    token: resp.data.accessToken,
    refreshToken: resp.data.refreshToken,
    expires_at: new Date(Date.now() + resp.data.expiresIn * 1000).toISOString(),
    user: mapUser(resp.data.user),
  };
}

/** 获取当前登录用户信息 */
export async function emailGetMe(token?: string | null): Promise<UserInfoResponse> {
  const resp = await request<MembershipMeResponse>(buildAuthPath('/auth/me'), {
    authToken: token ?? undefined,
  });
  return mapUser(resp.data.user);
}

/** 注销当前邮箱会话 */
export async function emailLogout(token: string): Promise<void> {
  await request('/auth/logout', {
    method: 'POST',
    authToken: token,
  });
}

/**
 * 请求发送“重置密码”邮件。
 *
 * 说明：不同历史部署可能使用不同路由命名；这里对 404 做兜底，
 * 按常见路径顺序尝试。
 */
export async function emailRequestPasswordReset(email: string): Promise<void> {
  const normalizedEmail = String(email ?? '').trim();
  if (!normalizedEmail) {
    throw new Error('请输入邮箱地址');
  }

  const body = JSON.stringify({
    email: normalizedEmail,
    projectId: DEFAULT_PROJECT_ID || undefined,
  });

  const candidates = [
    '/auth/forgot-password',
    '/auth/password/forgot',
    '/auth/reset-password/request',
  ];

  let lastError: unknown = null;
  for (const path of candidates) {
    try {
      await request<unknown>(buildAuthPath(path), {
        method: 'POST',
        body,
      });
      return;
    } catch (error) {
      lastError = error;
      if (error instanceof EmailAuthRequestError && error.status === 404) {
        continue;
      }
      throw error;
    }
  }

  throw lastError instanceof Error ? lastError : new Error('忘记密码接口不可用');
}

/**
 * 请求发送“重置密码验证码”到邮箱。
 *
 * 注意：不同部署可能有不同路由命名；这里对 404 做兜底按常见路径尝试。
 */
export async function emailRequestPasswordResetCode(email: string): Promise<void> {
  const normalizedEmail = String(email ?? '').trim();
  if (!normalizedEmail) {
    throw new Error('请输入邮箱地址');
  }

  const body = JSON.stringify({
    email: normalizedEmail,
    projectId: DEFAULT_PROJECT_ID || undefined,
  });

  const candidates = [
    // Legacy compatibility path
    '/auth/forgot-password',
    '/auth/password-reset/request-code',
    '/auth/reset-password/request-code',
    '/auth/password/reset/code',
  ];

  let lastError: unknown = null;
  for (const path of candidates) {
    try {
      await request<unknown>(buildAuthPath(path), {
        method: 'POST',
        body,
      });
      return;
    } catch (error) {
      lastError = error;
      if (error instanceof EmailAuthRequestError && error.status === 404) {
        continue;
      }
      throw error;
    }
  }

  throw lastError instanceof Error ? lastError : new Error('发送验证码接口不可用');
}

/**
 * 使用邮箱验证码重置密码。
 *
 * - 不要求已登录
 * - 不返回 token（用户可用新密码重新登录）
 */
export async function emailResetPasswordWithCode(params: {
  email: string;
  code: string;
  newPassword: string;
}): Promise<void> {
  const normalizedEmail = String(params.email ?? '').trim();
  const code = String(params.code ?? '').trim();
  const newPassword = String(params.newPassword ?? '');

  if (!normalizedEmail) {
    throw new Error('请输入邮箱地址');
  }
  if (!code) {
    throw new Error('请输入验证码');
  }
  if (!newPassword) {
    throw new Error('请输入新密码');
  }

  const body = JSON.stringify({
    email: normalizedEmail,
    code,
    newPassword,
    projectId: DEFAULT_PROJECT_ID || undefined,
  });

  const candidates = [
    // Legacy compatibility path
    '/auth/reset-password',
    '/auth/password-reset/confirm',
    '/auth/reset-password/confirm',
    '/auth/password/reset',
  ];

  let lastError: unknown = null;
  for (const path of candidates) {
    try {
      await request<unknown>(buildAuthPath(path), {
        method: 'POST',
        body,
      });
      return;
    } catch (error) {
      lastError = error;
      if (error instanceof EmailAuthRequestError && error.status === 404) {
        continue;
      }
      throw error;
    }
  }

  throw lastError instanceof Error ? lastError : new Error('重置密码接口不可用');
}

export async function emailVerifyEmail(params: {
  email?: string;
  code?: string;
  token?: string;
}): Promise<VerifyEmailResponse> {
  const email = String(params.email ?? '').trim();
  const code = String(params.code ?? '').trim();
  const token = String(params.token ?? '').trim();

  if (!email && !code && !token) {
    throw new Error('请输入验证码或验证链接');
  }

  const body = JSON.stringify({
    email: email || undefined,
    code: code || undefined,
    token: token || undefined,
    projectId: DEFAULT_PROJECT_ID || undefined,
  });

  const candidates = [
    '/auth/verify-email',
    '/auth/verify-email-code',
    '/auth/email/verify',
  ];

  let lastError: unknown = null;
  for (const path of candidates) {
    try {
      const resp = await request<Record<string, unknown>>(buildAuthPath(path), {
        method: 'POST',
        body,
      });
      return {
        verified: true,
        login: extractLoginResponse(resp) ?? undefined,
      };
    } catch (error) {
      lastError = error;
      if (error instanceof EmailAuthRequestError && error.status === 404) {
        continue;
      }
      throw error;
    }
  }

  throw lastError instanceof Error ? lastError : new Error('邮箱验证接口不可用');
}

export async function emailResendVerification(email: string): Promise<void> {
  const normalizedEmail = String(email ?? '').trim();
  if (!normalizedEmail) {
    throw new Error('请输入邮箱地址');
  }

  const body = JSON.stringify({
    email: normalizedEmail,
    projectId: DEFAULT_PROJECT_ID || undefined,
  });

  const candidates = [
    '/auth/resend-verification',
    '/auth/verification/resend',
    '/auth/email/resend-verification',
  ];

  let lastError: unknown = null;
  for (const path of candidates) {
    try {
      await request<unknown>(buildAuthPath(path), {
        method: 'POST',
        body,
      });
      return;
    } catch (error) {
      lastError = error;
      if (error instanceof EmailAuthRequestError && error.status === 404) {
        continue;
      }
      throw error;
    }
  }

  throw lastError instanceof Error ? lastError : new Error('重发验证码接口不可用');
}

// ─── Token storage helpers ───────────────────────────────────────────────────

export function saveToken(token: string) {
  if (typeof window !== 'undefined') {
    localStorage.setItem(TOKEN_KEY, token);
  }
}

export function saveSession(session: StoredEmailSession) {
  if (typeof window === 'undefined') {
    return;
  }

  localStorage.setItem(TOKEN_KEY, session.token);
  localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export function loadSession(): StoredEmailSession | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const raw = localStorage.getItem(SESSION_KEY);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<StoredEmailSession>;
    if (!parsed || typeof parsed.token !== 'string' || typeof parsed.expiresAt !== 'string' || !parsed.user) {
      return null;
    }

    return {
      token: parsed.token,
      refreshToken: parsed.refreshToken,
      expiresAt: parsed.expiresAt,
      user: parsed.user,
    };
  } catch {
    return null;
  }
}

export function updateStoredUser(user: UserInfoResponse) {
  const session = loadSession();
  if (!session) {
    return;
  }

  saveSession({
    ...session,
    user,
  });
}

function isSessionExpiringSoon(session: StoredEmailSession): boolean {
  const expiresAt = Date.parse(session.expiresAt);
  if (!Number.isFinite(expiresAt)) {
    return true;
  }

  return expiresAt - Date.now() <= REFRESH_SKEW_MS;
}

export async function refreshEmailSession(force = false): Promise<StoredEmailSession | null> {
  const session = loadSession();
  if (!session) {
    return null;
  }

  if (!force && !isSessionExpiringSoon(session)) {
    return session;
  }

  if (!session.refreshToken) {
    return session;
  }

  if (pendingRefresh) {
    return pendingRefresh;
  }

  pendingRefresh = (async () => {
    try {
      const resp = await request<MembershipAuthResponse>(buildAuthPath('/auth/refresh'), {
        method: 'POST',
        body: JSON.stringify({
          refreshToken: session.refreshToken,
          projectId: DEFAULT_PROJECT_ID || undefined,
        }),
      });

      if (!hasCompleteAuthPayload(resp.data)) {
        throw new Error(resp.message || '刷新响应缺少登录信息');
      }

      const nextSession: StoredEmailSession = {
        token: resp.data.accessToken,
        refreshToken: resp.data.refreshToken || session.refreshToken,
        expiresAt: new Date(Date.now() + resp.data.expiresIn * 1000).toISOString(),
        user: mapUser(resp.data.user),
      };
      saveSession(nextSession);
      return nextSession;
    } catch (error) {
      if (error instanceof EmailAuthRequestError && (error.status === 401 || error.status === 403)) {
        clearToken();
        return null;
      }

      return session;
    } finally {
      pendingRefresh = null;
    }
  })();

  return pendingRefresh;
}

export async function getValidEmailAccessToken(): Promise<string | null> {
  const session = loadSession();
  if (!session) {
    return loadToken();
  }

  const ensuredSession = await refreshEmailSession();
  return ensuredSession?.token ?? null;
}

export function loadToken(): string | null {
  if (typeof window !== 'undefined') {
    return localStorage.getItem(TOKEN_KEY) || loadSession()?.token || null;
  }
  return null;
}

export function clearToken() {
  if (typeof window !== 'undefined') {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(SESSION_KEY);
  }
}
