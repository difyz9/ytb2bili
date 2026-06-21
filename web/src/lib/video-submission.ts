import { buildBackendUrl } from './backend-url';
import { getValidEmailAccessToken } from './email-auth';

export type VideoSubmissionPlatform = 'youtube' | 'douyin' | 'bilibili' | 'unknown';
export type SubmissionTargetKind = 'video' | 'playlist' | 'unknown';

interface SubmissionInspectionBase {
  normalizedUrl: string;
  platform: VideoSubmissionPlatform;
  targetKind: SubmissionTargetKind;
}

export interface SupportedSubmissionInspection extends SubmissionInspectionBase {
  isSupported: true;
  description: string;
}

export interface UnsupportedSubmissionInspection extends SubmissionInspectionBase {
  isSupported: false;
  reason: string;
}

export type VideoSubmissionInspection = SupportedSubmissionInspection | UnsupportedSubmissionInspection;

export interface VideoQueueSubmissionResult {
  videoId: string;
  message: string;
  platform: 'youtube' | 'douyin';
  submissionMode: 'single' | 'playlist';
  submittedCount: number;
  playlistId?: string;
}

export type PreferredResolution = 'best' | '720p' | '1080p' | '1440p' | '2160p';

export interface SpeechSynthesisConfig {
  language: string;
  voice_name: string;
  format: string;
  provider?: string;
  search?: string;
  rate?: number;
  volume?: number;
  pitch?: number;
}

export interface PlaylistSubmissionConfig {
  enabled: boolean;
  start_index: number;
  max_items: number;
}

export interface BilibiliVideoZone {
  id: number;
  name: string;
  parent_id?: number;
  children?: BilibiliVideoZone[];
}

export const USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG = 'playlist_submission_config';
export const USER_SETTING_KEY_BILIBILI_SUBMISSION_TID = 'bilibili_submission_tid';
export const USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT = 'bilibili_submission_copyright';
export const USER_SETTING_KEY_WATERMARK_PROMO_ENABLED = 'watermark_promo_enabled';
export const USER_SETTING_KEY_SUBTITLE_AUDIO_TTS_CONFIG = 'subtitle_audio_tts_config';
export const LEGACY_USER_SETTING_KEY_SUBTITLE_AUDIO_VOICE = 'subtitle_audio_voice';

export const BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS = [
  { value: '2', label: '转载', description: '需要带转载来源，适合搬运或二次整理内容。' },
  { value: '1', label: '自制', description: '以原创稿件提交，不附带转载来源字段。' },
] as const;

export const DEFAULT_SPEECH_SYNTHESIS_CONFIG: SpeechSynthesisConfig = {
  language: 'zh-CN',
  voice_name: 'zh-CN-XiaoxiaoNeural',
  format: 'mp3',
  provider: 'auto',
  search: '',
  rate: 1,
  volume: 100,
  pitch: 0,
};

export const SPEECH_SYNTHESIS_PROVIDER_OPTIONS: Array<{ value: string; label: string }> = [
  { value: 'auto', label: '自动选择' },
  { value: 'azure', label: 'Azure' },
  { value: 'tencent', label: 'Tencent' },
];

export const SPEECH_SYNTHESIS_FORMAT_OPTIONS: Array<{ value: string; label: string }> = [
  { value: 'mp3', label: 'MP3' },
  { value: 'wav', label: 'WAV' },
];

export const DEFAULT_PLAYLIST_SUBMISSION_CONFIG: PlaylistSubmissionConfig = {
  enabled: false,
  start_index: 1,
  max_items: 10,
};

export const SUBTITLE_AUDIO_VOICE_OPTIONS: Array<{ value: string; label: string; description: string }> = [
  { value: 'zh-CN-XiaoxiaoNeural', label: '晓晓', description: '女声，默认自然叙述' },
  { value: 'zh-CN-XiaoyiNeural', label: '晓伊', description: '女声，明亮轻快' },
  { value: 'zh-CN-YunyangNeural', label: '云扬', description: '男声，沉稳解说' },
  { value: 'zh-CN-YunxiNeural', label: '云希', description: '男声，清晰偏年轻' },
];

export interface TaskChainSettings {
  download_thumbnail: boolean;
  transcribe: boolean;
  translate_subtitles: boolean;
  synthesize_subtitle_audio: boolean;
}

export const DEFAULT_TASK_CHAIN_SETTINGS: TaskChainSettings = {
  download_thumbnail: true,
  transcribe: true,
  translate_subtitles: true,
  synthesize_subtitle_audio: true,
};

function parseNumberOrFallback(value: unknown, fallback: number): number {
  const numeric = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(numeric) ? numeric : fallback;
}

export function normalizeSpeechSynthesisConfig(config?: Partial<SpeechSynthesisConfig> | null): SpeechSynthesisConfig {
  const normalized: SpeechSynthesisConfig = {
    ...DEFAULT_SPEECH_SYNTHESIS_CONFIG,
    ...config,
  };

  normalized.language = String(config?.language ?? normalized.language ?? DEFAULT_SPEECH_SYNTHESIS_CONFIG.language).trim() || DEFAULT_SPEECH_SYNTHESIS_CONFIG.language;
  normalized.voice_name = String(config?.voice_name ?? normalized.voice_name ?? DEFAULT_SPEECH_SYNTHESIS_CONFIG.voice_name).trim() || DEFAULT_SPEECH_SYNTHESIS_CONFIG.voice_name;
  normalized.format = String(config?.format ?? normalized.format ?? DEFAULT_SPEECH_SYNTHESIS_CONFIG.format).trim() || DEFAULT_SPEECH_SYNTHESIS_CONFIG.format;
  normalized.provider = String(config?.provider ?? normalized.provider ?? DEFAULT_SPEECH_SYNTHESIS_CONFIG.provider ?? 'auto').trim() || 'auto';
  normalized.search = String(config?.search ?? normalized.search ?? '').trim();
  normalized.rate = parseNumberOrFallback(config?.rate ?? normalized.rate, DEFAULT_SPEECH_SYNTHESIS_CONFIG.rate ?? 1);
  normalized.volume = parseNumberOrFallback(config?.volume ?? normalized.volume, DEFAULT_SPEECH_SYNTHESIS_CONFIG.volume ?? 100);
  normalized.pitch = parseNumberOrFallback(config?.pitch ?? normalized.pitch, DEFAULT_SPEECH_SYNTHESIS_CONFIG.pitch ?? 0);

  return normalized;
}

export function parseSpeechSynthesisConfig(value?: string | null): SpeechSynthesisConfig | null {
  const trimmed = value?.trim();
  if (!trimmed) {
    return null;
  }

  if (!trimmed.startsWith('{')) {
    return normalizeSpeechSynthesisConfig({ voice_name: trimmed });
  }

  try {
    return normalizeSpeechSynthesisConfig(JSON.parse(trimmed) as Partial<SpeechSynthesisConfig>);
  } catch {
    return null;
  }
}

export function serializeSpeechSynthesisConfig(config?: Partial<SpeechSynthesisConfig> | null): string {
  const normalized = normalizeSpeechSynthesisConfig(config);
  return JSON.stringify({
    language: normalized.language,
    voice_name: normalized.voice_name,
    format: normalized.format,
    provider: normalized.provider,
    search: normalized.search,
    rate: normalized.rate,
    volume: normalized.volume,
    pitch: normalized.pitch,
  });
}

export function getSpeechSynthesisVoiceLabel(config?: Partial<SpeechSynthesisConfig> | string | null): string {
  const voiceName = typeof config === 'string'
    ? config
    : normalizeSpeechSynthesisConfig(config).voice_name;

  return SUBTITLE_AUDIO_VOICE_OPTIONS.find((option) => option.value === voiceName)?.label ?? voiceName;
}

export function getStoredSubtitleAudioTTSConfigValue(settings?: Record<string, string> | null): string {
  if (!settings) {
    return '';
  }
  return settings[USER_SETTING_KEY_SUBTITLE_AUDIO_TTS_CONFIG]
    ?? settings[LEGACY_USER_SETTING_KEY_SUBTITLE_AUDIO_VOICE]
    ?? '';
}

const YOUTUBE_URL_RE = /^(https?:\/\/)?(?:www\.)?(?:youtube\.com\/(?:watch\?[^\s#]*v=|shorts\/|embed\/)|youtu\.be\/)[^\s]+$/i;
const DOUYIN_URL_RE = /^(https?:\/\/)?(?:(?:v|www)\.douyin\.com\/[^\s]+|iesdouyin\.com\/[^\s]+)$/i;
const BILIBILI_URL_RE = /^(https?:\/\/)?(?:www\.)?bilibili\.com\/video\/[A-Za-z0-9]+(?:[/?#].*)?$/i;

export function normalizePlaylistSubmissionConfig(config?: Partial<PlaylistSubmissionConfig> | null): PlaylistSubmissionConfig {
  const startIndex = Number(config?.start_index ?? DEFAULT_PLAYLIST_SUBMISSION_CONFIG.start_index);
  const maxItems = Number(config?.max_items ?? DEFAULT_PLAYLIST_SUBMISSION_CONFIG.max_items);

  return {
    enabled: Boolean(config?.enabled),
    start_index: Number.isFinite(startIndex) && startIndex >= 1
      ? Math.floor(startIndex)
      : DEFAULT_PLAYLIST_SUBMISSION_CONFIG.start_index,
    max_items: Number.isFinite(maxItems) && maxItems >= 1
      ? Math.min(50, Math.floor(maxItems))
      : DEFAULT_PLAYLIST_SUBMISSION_CONFIG.max_items,
  };
}

export function parsePlaylistSubmissionConfig(value?: string | null): PlaylistSubmissionConfig | null {
  if (!value) {
    return null;
  }

  try {
    return normalizePlaylistSubmissionConfig(JSON.parse(value) as Partial<PlaylistSubmissionConfig>);
  } catch {
    return null;
  }
}

function tryParseUrl(rawUrl: string): URL | null {
  try {
    return new URL(rawUrl);
  } catch {
    try {
      return new URL(`https://${rawUrl}`);
    } catch {
      return null;
    }
  }
}

function extractYouTubePlaylistId(rawUrl: string): string | null {
  const parsed = tryParseUrl(rawUrl.trim());
  if (!parsed) {
    return null;
  }

  const hostname = parsed.hostname.toLowerCase();
  if (!(hostname === 'youtu.be' || hostname.endsWith('youtube.com') || hostname.endsWith('youtu.be'))) {
    return null;
  }

  const playlistId = parsed.searchParams.get('list')?.trim();
  return playlistId || null;
}

export class VideoSubmissionError extends Error {
  status?: number;
  code?: string;

  constructor(message: string, options?: { status?: number; code?: string }) {
    super(message);
    this.name = 'VideoSubmissionError';
    this.status = options?.status;
    this.code = options?.code;
  }
}

export function inspectVideoSubmissionUrl(
  rawUrl: string,
  playlistConfig: PlaylistSubmissionConfig = DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
): VideoSubmissionInspection {
  const normalizedUrl = rawUrl.trim();
  const normalizedPlaylistConfig = normalizePlaylistSubmissionConfig(playlistConfig);
  const playlistId = extractYouTubePlaylistId(normalizedUrl);

  if (!normalizedUrl) {
    return {
      normalizedUrl,
      platform: 'unknown',
      targetKind: 'unknown',
      isSupported: false,
      reason: '请输入完整的 YouTube 视频链接或抖音分享链接。',
    };
  }

  if (YOUTUBE_URL_RE.test(normalizedUrl)) {
    if (playlistId && normalizedPlaylistConfig.enabled) {
      return {
        normalizedUrl,
        platform: 'youtube',
        targetKind: 'playlist',
        isSupported: true,
        description: `将按播放列表模式批量入队，从第 ${normalizedPlaylistConfig.start_index} 条开始，最多导入 ${normalizedPlaylistConfig.max_items} 条视频。`,
      };
    }

    return {
      normalizedUrl,
      platform: 'youtube',
      targetKind: 'video',
      isSupported: true,
      description: '提交后会立即加入任务队列，并在后台执行下载、转录、翻译和 B 站元数据生成。',
    };
  }

  if (playlistId) {
    if (normalizedPlaylistConfig.enabled) {
      return {
        normalizedUrl,
        platform: 'youtube',
        targetKind: 'playlist',
        isSupported: true,
        description: `将按播放列表模式批量入队，从第 ${normalizedPlaylistConfig.start_index} 条开始，最多导入 ${normalizedPlaylistConfig.max_items} 条视频。`,
      };
    }

    return {
      normalizedUrl,
      platform: 'youtube',
      targetKind: 'playlist',
      isSupported: false,
      reason: '检测到 YouTube 播放列表链接。请先在设置中开启“播放列表批量提交”。',
    };
  }

  if (DOUYIN_URL_RE.test(normalizedUrl)) {
    return {
      normalizedUrl,
      platform: 'douyin',
      targetKind: 'video',
      isSupported: true,
      description: '提交后会立即加入任务队列，并在后台执行抖音视频下载、音频提取、字幕转录与后续处理。',
    };
  }

  if (BILIBILI_URL_RE.test(normalizedUrl)) {
    return {
      normalizedUrl,
      platform: 'bilibili',
      targetKind: 'video',
      isSupported: false,
      reason: '当前页面支持提交 YouTube 链接和抖音分享链接。Bilibili 链接暂不支持直接进入处理队列。',
    };
  }

  return {
    normalizedUrl,
    platform: 'unknown',
    targetKind: 'unknown',
    isSupported: false,
    reason: '链接格式无效。请输入可访问的 YouTube 视频链接或抖音分享链接。',
  };
}

async function getAuthorizationHeader(): Promise<string | null> {
  const emailToken = await getValidEmailAccessToken();
  if (emailToken) {
    return `Bearer ${emailToken}`;
  }

  return null;
}

export async function estimateCredits(service: string, units: number): Promise<number> {
  const normalizedService = String(service || '').trim();
  const normalizedUnits = Number.isFinite(units) ? units : Number(units);

  if (!normalizedService) {
    return 0;
  }

  const headers = new Headers();
  const authorization = await getAuthorizationHeader();
  if (authorization) {
    headers.set('Authorization', authorization);
  }

  try {
    const params = new URLSearchParams({
      service: normalizedService,
      units: String(Number.isFinite(normalizedUnits) ? normalizedUnits : 0),
    });
    const response = await fetch(buildBackendUrl(`/api/credits/estimate?${params.toString()}`), {
      method: 'GET',
      headers,
      credentials: 'include',
    });
    if (!response.ok) {
      return 0;
    }
    const payload = await response.json().catch(() => null) as { credits?: number } | null;
    const credits = payload?.credits;
    return typeof credits === 'number' && Number.isFinite(credits) ? credits : 0;
  } catch {
    return 0;
  }
}

export async function submitVideoToQueue(
  url: string,
  userId?: string,
  preferredResolution?: PreferredResolution,
  taskChainSettings?: TaskChainSettings,
  speechSynthesisConfig?: SpeechSynthesisConfig,
  playlistConfig: PlaylistSubmissionConfig = DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
): Promise<VideoQueueSubmissionResult> {
  const inspection = inspectVideoSubmissionUrl(url, playlistConfig);
  if (!inspection.isSupported) {
    throw new VideoSubmissionError(inspection.reason, { code: inspection.platform });
  }

  const headers = new Headers({ 'Content-Type': 'application/json' });
  const authorization = await getAuthorizationHeader();
  if (!authorization) {
    throw new VideoSubmissionError('未检测到有效登录令牌，请先登录后再提交视频。', {
      code: 'missing-auth-token',
    });
  }

  headers.set('Authorization', authorization);

  const body: Record<string, unknown> = {
    url: inspection.normalizedUrl,
    user_id: userId,
  };

  if (preferredResolution) {
    body.preferred_resolution = preferredResolution;
  }

  if (taskChainSettings) {
    body.task_chain_settings = taskChainSettings;
  }

  if (speechSynthesisConfig) {
    body.speech_synthesis_config = speechSynthesisConfig;
  }

  if (inspection.platform === 'youtube' && normalizePlaylistSubmissionConfig(playlistConfig).enabled && extractYouTubePlaylistId(url)) {
    body.playlist_config = normalizePlaylistSubmissionConfig(playlistConfig);
  }

  const response = await fetch(buildBackendUrl('/api/v1/video-process/async-submit-link'), {
    method: 'POST',
    headers,
    credentials: 'include',
    body: JSON.stringify(body),
  });

  const payload = await response.json().catch(() => null) as {
    code?: number;
    error?: string;
    message?: string;
    data?: {
      video_id?: string;
      playlist_id?: string;
      submitted_count?: number;
      submission_mode?: 'single' | 'playlist';
    };
  } | null;

  if (response.ok && payload?.code === 0 && payload.data?.video_id) {
    return {
      videoId: payload.data.video_id,
      message: payload.message ?? '已加入处理队列',
      platform: inspection.platform === 'douyin' ? 'douyin' : 'youtube',
      submissionMode: payload.data.submission_mode === 'playlist' ? 'playlist' : 'single',
      submittedCount: typeof payload.data.submitted_count === 'number' ? payload.data.submitted_count : 1,
      playlistId: payload.data.playlist_id,
    };
  }

  throw new VideoSubmissionError(
    payload?.message ?? '提交失败，请稍后重试。',
    {
      status: response.status,
      code: payload?.error ?? (typeof payload?.code === 'number' ? String(payload.code) : undefined),
    },
  );
}

export function getVideoSubmissionErrorMessage(error: unknown): string {
  if (error instanceof VideoSubmissionError) {
    return error.message;
  }

  if (error instanceof Error) {
    return error.message;
  }

  return '提交失败，请稍后重试。';
}