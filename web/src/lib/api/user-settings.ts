import { buildBackendUrl } from '@/lib/backend-url';
import { getValidEmailAccessToken } from '@/lib/email-auth';
import {
  type BilibiliVideoZone,
  LEGACY_USER_SETTING_KEY_SUBTITLE_AUDIO_VOICE,
  USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT,
  USER_SETTING_KEY_BILIBILI_SUBMISSION_TID,
  USER_SETTING_KEY_SUBTITLE_AUDIO_TTS_CONFIG,
  USER_SETTING_KEY_WATERMARK_PROMO_ENABLED,
} from '@/lib/video-submission';

export type UserSettingsMap = Record<string, string>;

const SUPPORTED_LOCAL_SETTINGS = new Set([
  'preferred_ai_model',
  'preferred_ai_model_name',
  'preferred_resolution',
  'task_chain_settings',
  USER_SETTING_KEY_BILIBILI_SUBMISSION_TID,
  USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT,
  USER_SETTING_KEY_WATERMARK_PROMO_ENABLED,
  USER_SETTING_KEY_SUBTITLE_AUDIO_TTS_CONFIG,
  LEGACY_USER_SETTING_KEY_SUBTITLE_AUDIO_VOICE,
  'translation_model',
  'metadata_model',
  'auto_upload',
  'auto_upload_interval_minutes',
  'translation_source_lang',
  'translation_target_lang',
  'bid_default_language',
  'bid_default_tone',
  'bid_template_style',
  'assistant_system_prompt',
]);

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

function isStringRecord(value: unknown): value is UserSettingsMap {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false;
  }

  return Object.values(value).every((entry) => typeof entry === 'string');
}

function isBilibiliVideoZoneArray(value: unknown): value is BilibiliVideoZone[] {
  if (!Array.isArray(value)) {
    return false;
  }

  return value.every((entry) => {
    if (!entry || typeof entry !== 'object') {
      return false;
    }

    const zone = entry as BilibiliVideoZone;
    const hasValidChildren = zone.children === undefined || isBilibiliVideoZoneArray(zone.children);
    return typeof zone.id === 'number' && typeof zone.name === 'string' && hasValidChildren;
  });
}

export const userSettingsApi = {
  getBilibiliVideoZones: async (): Promise<BilibiliVideoZone[]> => {
    const token = await resolveAuthToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers.Authorization = `Bearer ${token}`;

    const response = await fetch(buildBackendUrl('/api/v1/user/settings/bilibili-video-zones'), {
      headers,
      credentials: 'include',
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error((payload as { message?: string; error?: string }).message ?? (payload as { error?: string }).error ?? `请求失败 (${response.status})`);
    }

    const data = (payload as { data?: unknown }).data ?? payload;
    const zones = data && typeof data === 'object' && 'zones' in data
      ? (data as { zones?: unknown }).zones
      : data;

    return isBilibiliVideoZoneArray(zones) ? zones : [];
  },

  getSettings: async (): Promise<UserSettingsMap> => {
    const token = await resolveAuthToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers.Authorization = `Bearer ${token}`;

    const response = await fetch(buildBackendUrl('/api/v1/user/settings'), {
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
      return (data as { settings: UserSettingsMap }).settings;
    }
    return isStringRecord(data) ? data : {};
  },

  updateSettings: async (settings: UserSettingsMap): Promise<UserSettingsMap> => {
    const token = await resolveAuthToken();
    const filteredSettings = Object.fromEntries(
      Object.entries(settings).filter(([key]) => SUPPORTED_LOCAL_SETTINGS.has(key)),
    ) as UserSettingsMap;

    if (Object.keys(filteredSettings).length === 0) {
      return settings;
    }

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers.Authorization = `Bearer ${token}`;

    const response = await fetch(buildBackendUrl('/api/v1/user/settings'), {
      method: 'PUT',
      headers,
      credentials: 'include',
      body: JSON.stringify(filteredSettings),
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error((payload as { message?: string; error?: string }).message ?? (payload as { error?: string }).error ?? `请求失败 (${response.status})`);
    }

    const data = (payload as { data?: { settings?: UserSettingsMap } }).data;
    if (data?.settings && isStringRecord(data.settings)) {
      return data.settings;
    }

    return filteredSettings;
  },
};