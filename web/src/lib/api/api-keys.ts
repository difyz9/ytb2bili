import { buildBackendUrl } from '@/lib/backend-url';
import { getValidEmailAccessToken } from '@/lib/email-auth';
import { getDefaultProjectHeaders } from '@/lib/project-id';

export interface UserApiKeyRecord {
  id: string;
  name: string;
  keyPrefix: string;
  secret?: string;
  active: boolean;
  lastUsedAt?: number | null;
  createdAt: number;
  updatedAt: number;
}

type PartialApiKeyRecord = {
  id?: unknown;
  name?: unknown;
  keyPrefix?: unknown;
  secret?: unknown;
  active?: unknown;
  lastUsedAt?: unknown;
  createdAt?: unknown;
  updatedAt?: unknown;
};

function normalizeApiKeyRecord(value: unknown): UserApiKeyRecord | null {
  if (!value || typeof value !== 'object') {
    return null;
  }

  const record = value as PartialApiKeyRecord;
  if (
    typeof record.id !== 'string' ||
    typeof record.name !== 'string' ||
    typeof record.keyPrefix !== 'string' ||
    typeof record.createdAt !== 'number'
  ) {
    return null;
  }

  return {
    id: record.id,
    name: record.name,
    keyPrefix: record.keyPrefix,
    secret: typeof record.secret === 'string' && record.secret.trim() ? record.secret : undefined,
    active: typeof record.active === 'boolean' ? record.active : true,
    lastUsedAt: typeof record.lastUsedAt === 'number' ? record.lastUsedAt : null,
    createdAt: record.createdAt,
    updatedAt: typeof record.updatedAt === 'number' ? record.updatedAt : record.createdAt,
  };
}

function parseApiKeyRecords(value: unknown): UserApiKeyRecord[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value
    .map((item) => normalizeApiKeyRecord(item))
    .filter((item): item is UserApiKeyRecord => item !== null);
}

async function resolveAuthToken(): Promise<string | null> {
  const emailToken = await getValidEmailAccessToken();
  if (emailToken) {
    return emailToken;
  }

  if (typeof window !== 'undefined') {
    const cachedToken = localStorage.getItem('auth_token') || localStorage.getItem('email_auth_token');
    if (cachedToken) {
      return cachedToken;
    }
  }

  return null;
}

function getErrorMessage(payload: unknown, status: number): string {
  if (payload && typeof payload === 'object') {
    const data = payload as Record<string, unknown>;
    if (typeof data.message === 'string' && data.message.trim()) {
      return data.message;
    }
    if (typeof data.error === 'string' && data.error.trim()) {
      return data.error;
    }
  }

  return `请求失败 (${status})`;
}

async function requestBackend<T>(path: string, init: RequestInit, parser: (payload: unknown) => T): Promise<T> {
  const token = await resolveAuthToken();
  const headers = {
    'Content-Type': 'application/json',
    ...getDefaultProjectHeaders(),
    ...((init.headers as Record<string, string> | undefined) ?? {}),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };

  const response = await fetch(buildBackendUrl(path), {
    ...init,
    headers,
    credentials: 'include',
  });

  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(getErrorMessage(payload, response.status));
  }

  return parser(payload);
}

export const apiKeysApi = {
  list: async (): Promise<UserApiKeyRecord[]> => requestBackend('/api/v1/user/api-keys', {
    method: 'GET',
  }, (payload) => {
    const data = (payload as { data?: unknown }).data ?? payload;
    const keys = data && typeof data === 'object' && 'keys' in data
      ? (data as { keys?: unknown }).keys
      : data;

    return parseApiKeyRecords(keys);
  }),

  create: async (name: string): Promise<UserApiKeyRecord> => requestBackend('/api/v1/user/api-keys', {
    method: 'POST',
    body: JSON.stringify({ name }),
  }, (payload) => {
    const data = (payload as { data?: unknown }).data ?? payload;
    const key = data && typeof data === 'object' && 'key' in data
      ? (data as { key?: unknown }).key
      : data;

    const normalizedKey = normalizeApiKeyRecord(key);
    if (!normalizedKey) {
      throw new Error('创建 API 密钥成功，但后端返回数据格式不符合预期');
    }

    return normalizedKey;
  }),

  remove: async (keyId: string): Promise<void> => {
    await requestBackend(`/api/v1/user/api-keys/${encodeURIComponent(keyId)}`, {
      method: 'DELETE',
    }, () => undefined);
  },
};