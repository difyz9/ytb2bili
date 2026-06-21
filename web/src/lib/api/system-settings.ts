import { buildBackendUrl } from '@/lib/backend-url';
import { getValidEmailAccessToken } from '@/lib/email-auth';

export type SystemSettingsMap = Record<string, string>;

async function resolveAuthToken(): Promise<string | null> {
  const emailToken = await getValidEmailAccessToken();
  if (emailToken) {
    return emailToken;
  }

  if (typeof window !== 'undefined') {
    const cachedToken = localStorage.getItem('auth_token');
    if (cachedToken) {
      return cachedToken;
    }
  }

  return null;
}

function isStringRecord(value: unknown): value is SystemSettingsMap {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false;
  }

  return Object.values(value).every((entry) => typeof entry === 'string');
}

export const systemSettingsApi = {
  getSettings: async (): Promise<SystemSettingsMap> => {
    const token = await resolveAuthToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers.Authorization = `Bearer ${token}`;

    const response = await fetch(buildBackendUrl('/api/v1/system/settings'), {
      headers,
      credentials: 'include',
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error((payload as { message?: string; error?: string }).message ?? (payload as { error?: string }).error ?? `请求失败 (${response.status})`);
    }

    const data = (payload as { data?: unknown }).data ?? payload;
    if (
      data &&
      typeof data === 'object' &&
      'settings' in data &&
      isStringRecord((data as { settings?: unknown }).settings)
    ) {
      return (data as { settings: SystemSettingsMap }).settings;
    }

    return isStringRecord(data) ? data : {};
  },

  updateSettings: async (settings: SystemSettingsMap): Promise<SystemSettingsMap> => {
    const token = await resolveAuthToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers.Authorization = `Bearer ${token}`;

    const response = await fetch(buildBackendUrl('/api/v1/system/settings'), {
      method: 'PUT',
      headers,
      credentials: 'include',
      body: JSON.stringify(settings),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error((payload as { message?: string; error?: string }).message ?? (payload as { error?: string }).error ?? `请求失败 (${response.status})`);
    }

    const data = (payload as { data?: { settings?: SystemSettingsMap } }).data;
    if (data?.settings && isStringRecord(data.settings)) {
      return data.settings;
    }

    return settings;
  },
};