'use client';

import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, Clock, Play, CheckCircle, Upload, AlertCircle, XCircle, ChevronDown, ChevronRight, RotateCcw, PlayCircle, Square, Trash2, SlidersHorizontal } from 'lucide-react';
import VideoPlayerModal from '@/components/VideoPlayerModal';
import { Modal } from '@/components/ui/Modal';
import { VoicePicker } from '@/components/tts/VoicePicker';
import { useI18n } from '@/contexts/I18nContext';
import type { ModelCatalogItem } from '@/lib/agent-models';
import { agentApi } from '@/lib/api/agent';
import { findTTSVoiceByShortName, loadTTSVoiceCatalog } from '@/lib/tts-voice-catalog';
import { guessSubtitleAudioBaseUrl, guessSubtitleUrl, videoPathToUrl } from '@/lib/video-paths';
import { getValidEmailAccessToken } from '@/lib/email-auth';
import { userSettingsApi } from '@/lib/api/user-settings';
import {
  BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS,
  DEFAULT_SPEECH_SYNTHESIS_CONFIG,
  DEFAULT_TASK_CHAIN_SETTINGS,
  USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT,
} from '@/lib/video-submission';

/** 构造带 Authorization 头的 fetch 请求 */
async function authFetch(url: string, init: RequestInit = {}): Promise<Response> {
  const token = await getValidEmailAccessToken();
  const headers: Record<string, string> = {
    ...(init.headers as Record<string, string>),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return fetch(url, { ...init, headers });
}

// ─── Step name localisation ───────────────────────────────────────────────────
const STEP_LABEL_KEYS: Record<string, string> = {
  Initialize: 'Initialize',
  ResolveDouyinShare: 'Resolve Douyin share link',
  DownloadVideo: 'Download video',
  DownloadDouyinVideo: 'Download Douyin video',
  DownloadThumbnail: 'Download thumbnail',
  ExtractAudio: 'Extract audio',
  Transcribe: 'Transcribe subtitles',
  LLMTranslate: 'Translate subtitles',
  SynthesizeSubtitleAudio: 'Synthesize subtitle audio',
  SaveDatabase: 'Save results',
  GenerateSubtitle: 'Generate subtitles',
  GenerateMetadata: 'Generate metadata',
  UploadBilibili: 'Upload to Bilibili',
};
const stepLabelKey = (name: string) => STEP_LABEL_KEYS[name] ?? name;

// ─── Types ────────────────────────────────────────────────────────────────────
interface TaskStep {
  id: number;
  video_id: string;
  step_name: string;
  step_order: number;
  status: string;
  start_time: string | null;
  end_time: string | null;
  duration: number;
  error_msg: string;
  can_retry: boolean;
  progress_percent?: number;
  progress_text?: string;
}

interface Video {
  id: number;
  video_id: string;
  title: string;
  description?: string;
  thumbnail?: string;
  status: string;
  operation_type?: string;
  platform?: string;
  created_at: string;
  updated_at: string;
  task_steps?: TaskStep[];
  bili_bvid?: string;
  url?: string;
  video_path?: string;
  video_size_bytes?: number;
  subtitle_path?: string;
  generated_title?: string;
  generated_desc?: string;
  generated_tags?: string;
  recommended_tags?: string;
  preferred_resolution?: string;
  speech_voice_name?: string;
}

type TabType = 'all' | 'processing' | 'completed' | 'failed';
type RerunMode = 'voice' | 'translation' | 'translation-only';

interface RerunDraft {
  video: Video;
  mode: RerunMode;
  translationModel: string;
  voiceName: string;
}

function buildRerunTaskChainSettings(mode: RerunMode) {
  switch (mode) {
    case 'voice':
      return {
        ...DEFAULT_TASK_CHAIN_SETTINGS,
        download_thumbnail: false,
        transcribe: false,
        translate_subtitles: false,
        synthesize_subtitle_audio: true,
      };
    case 'translation-only':
      return {
        ...DEFAULT_TASK_CHAIN_SETTINGS,
        download_thumbnail: false,
        transcribe: false,
        translate_subtitles: true,
        synthesize_subtitle_audio: false,
      };
    default:
      return {
        ...DEFAULT_TASK_CHAIN_SETTINGS,
        download_thumbnail: false,
        transcribe: false,
        translate_subtitles: true,
        synthesize_subtitle_audio: true,
      };
  }
}

interface UploadDraft {
  video: Video;
  accountId: number;
  copyright: string;
  title: string;
  description: string;
  tags: string;
  cover: string;
}

interface BiliAccountOption {
  id: number;
  bili_mid: number;
  bili_name: string;
  is_primary?: boolean;
  last_used_at?: string;
}

function getErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

// ─── Status helpers ───────────────────────────────────────────────────────────
const VIDEO_STATUS: Record<string, { label: string; color: string; dot: string; tab: TabType }> = {
  '001': { label: 'Pending', color: 'bg-gray-100 text-gray-700 border border-gray-200', dot: 'bg-gray-400', tab: 'processing' },
  '002': { label: 'Processing', color: 'bg-blue-50 text-blue-700 border border-blue-200', dot: 'bg-blue-500 animate-pulse', tab: 'processing' },
  '003': { label: 'Completed', color: 'bg-emerald-50 text-emerald-700 border border-emerald-200', dot: 'bg-emerald-500', tab: 'completed' },
  '004': { label: 'Task failed', color: 'bg-red-50 text-red-700 border border-red-200', dot: 'bg-red-500', tab: 'failed' },
  pending:    { label: 'Pending',  color: 'bg-gray-100 text-gray-700 border border-gray-200',    dot: 'bg-gray-400', tab: 'processing' },
  processing: { label: 'Processing',  color: 'bg-blue-50 text-blue-700 border border-blue-200',     dot: 'bg-blue-500 animate-pulse', tab: 'processing' },
  processed:  { label: 'Processed',  color: 'bg-cyan-50 text-cyan-700 border border-cyan-200',     dot: 'bg-cyan-500', tab: 'completed' },
  ready:      { label: 'Ready',color: 'bg-green-50 text-green-700 border border-green-200', dot: 'bg-green-500', tab: 'completed' },
  completed:  { label: 'Completed',  color: 'bg-emerald-50 text-emerald-700 border border-emerald-200', dot: 'bg-emerald-500', tab: 'completed' },
  failed:     { label: 'Task failed',color: 'bg-red-50 text-red-700 border border-red-200',       dot: 'bg-red-500', tab: 'failed' },
  paused:     { label: 'Stopped',  color: 'bg-amber-50 text-amber-700 border border-amber-200', dot: 'bg-amber-500', tab: 'failed' },
  synced:     { label: 'Synced',  color: 'bg-teal-50 text-teal-700 border border-teal-200',    dot: 'bg-teal-500', tab: 'completed' },
};
const getVideoStatus = (s: string) =>
  VIDEO_STATUS[s] ?? { label: s || 'Unknown', color: 'bg-gray-100 text-gray-700 border border-gray-200', dot: 'bg-gray-400', tab: 'all' as TabType };

const STEP_STYLE: Record<string, { badge: string; label: string; icon: React.ReactNode }> = {
  completed: { badge: 'bg-green-100 text-green-800',  label: 'Completed', icon: <CheckCircle className="w-3.5 h-3.5 text-green-600" /> },
  failed:    { badge: 'bg-red-100 text-red-800',      label: 'Failed',   icon: <XCircle className="w-3.5 h-3.5 text-red-600" /> },
  running:   { badge: 'bg-blue-100 text-blue-800',    label: 'Running', icon: <Play className="w-3.5 h-3.5 text-blue-600" /> },
  skipped:   { badge: 'bg-gray-100 text-gray-500',    label: 'Skipped', icon: <ChevronRight className="w-3.5 h-3.5 text-gray-400" /> },
  pending:   { badge: 'bg-gray-100 text-gray-600',    label: 'Pending execution', icon: <Clock className="w-3.5 h-3.5 text-gray-400" /> },
};
const getStepStyle = (s: string) => STEP_STYLE[s] ?? { badge: 'bg-gray-100 text-gray-600', label: s, icon: <Clock className="w-3.5 h-3.5 text-gray-400" /> };

const formatDuration = (ms: number) => {
  if (!ms) return '';
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
};

const calculateStepProgress = (steps: TaskStep[]) => {
  if (steps.length === 0) return 0;
  const completed = steps.filter(step => step.status === 'completed' || step.status === 'skipped').length;
  const running = steps.find(step => step.status === 'running');
  return Math.round(((completed + ((running?.progress_percent ?? 0) / 100)) / steps.length) * 100);
};

const formatFileSize = (bytes?: number) => {
  if (!bytes || bytes <= 0) return '';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const precision = value >= 100 || unitIndex === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(precision)} ${units[unitIndex]}`;
};

const DEFAULT_BILIBILI_COVER_FILENAME = 'thumbnail_maxresdefault.jpg';

const resolvePreferredUploadCover = (videoPath?: string, thumbnailPath?: string) => {
  const normalizedVideoPath = videoPath?.trim();
  if (normalizedVideoPath) {
    const separatorIndex = Math.max(normalizedVideoPath.lastIndexOf('/'), normalizedVideoPath.lastIndexOf('\\'));
    if (separatorIndex >= 0) {
      return `${normalizedVideoPath.slice(0, separatorIndex + 1)}${DEFAULT_BILIBILI_COVER_FILENAME}`;
    }
  }
  return thumbnailPath?.trim() ?? '';
};

const isBilibiliSubmissionCopyrightValue = (value: string) => (
  BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS.some((option) => option.value === value)
);

const normalizeTagList = (raw?: string) => (
  (raw ?? '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
);

const buildPreferredUploadTags = (video: Video) => {
  const preferred = video.recommended_tags?.trim() || video.generated_tags?.trim() || '';
  return normalizeTagList(preferred).join(', ');
};

// ─── Subcomponent: step list ──────────────────────────────────────────────────
function TaskStepList({ steps, onRetry }: { steps: TaskStep[]; onRetry: (name: string) => void }) {
  const { t } = useI18n();
  const sorted = [...steps].sort((a, b) => a.step_order - b.step_order);
  return (
    <div className="mt-3 space-y-1.5">
      <h5 className="text-sm font-semibold text-gray-700 mb-2">{t('Task steps')}</h5>
      {sorted.map((step, idx) => {
        const style = getStepStyle(step.status);
        const isLast = idx === sorted.length - 1;
        return (
          <div key={step.step_name} className="flex gap-3">
            {/* timeline track */}
            <div className="flex flex-col items-center w-5 shrink-0">
              <div className={`w-5 h-5 rounded-full flex items-center justify-center border-2 ${
                step.status === 'completed' ? 'border-green-400 bg-green-50' :
                step.status === 'failed'    ? 'border-red-400 bg-red-50' :
                step.status === 'running'   ? 'border-blue-400 bg-blue-50' :
                step.status === 'skipped'   ? 'border-gray-300 bg-gray-50' :
                'border-gray-300 bg-white'
              }`}>
                {style.icon}
              </div>
              {!isLast && <div className="w-px flex-1 mt-1 bg-gray-200" />}
            </div>

            {/* step card */}
            <div className={`flex-1 mb-1.5 rounded-lg border px-3 py-2 ${
              step.status === 'failed'  ? 'border-red-100 bg-red-50/50' :
              step.status === 'running' ? 'border-blue-100 bg-blue-50/50' :
              step.status === 'completed' ? 'border-green-100 bg-green-50/30' :
              'border-gray-100 bg-white'
            }`}>
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-gray-800">{t(stepLabelKey(step.step_name))}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${style.badge}`}>{t(style.label)}</span>
                  {step.duration > 0 && (
                    <span className="text-xs text-gray-400">{formatDuration(step.duration)}</span>
                  )}
                </div>
                {step.can_retry && step.status === 'failed' && (
                  <button
                    onClick={() => onRetry(step.step_name)}
                    className="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-800 bg-blue-50 hover:bg-blue-100 px-2 py-1 rounded transition-colors"
                  >
                    <RotateCcw className="w-3 h-3" /> {t('Retry')}
                  </button>
                )}
              </div>
              {step.error_msg && (
                <p className="text-xs text-red-600 mt-1 break-all">
                  {t('Error:')} {step.error_msg}
                </p>
              )}
              {step.status === 'running' && step.progress_text && (
                <p className="text-xs text-blue-600 mt-1 break-all">
                  {t('Progress:')} {step.progress_text}
                </p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────
type UploadState = 'idle' | 'uploading' | 'done' | 'error';

const PAGE_SIZE = 10;

export default function TaskQueueStats() {
  const { locale, t } = useI18n();
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabType>('all');
  const [refreshing, setRefreshing] = useState(false);
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());
  const [currentPage, setCurrentPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [tabCounts, setTabCounts] = useState({ all: 0, processing: 0, completed: 0, failed: 0 });
  const [playerVideo, setPlayerVideo] = useState<{ title: string; videoUrl: string; subtitleUrl?: string; audioBaseUrl?: string } | null>(null);
  const [uploadStates, setUploadStates] = useState<Record<number, UploadState>>({});
  const [uploadErrors, setUploadErrors] = useState<Record<number, string>>({});
  const [resumeStates, setResumeStates] = useState<Record<number, 'idle' | 'resuming'>>({});
  const [resumeErrors, setResumeErrors] = useState<Record<number, string>>({});
  const [stopStates, setStopStates] = useState<Record<number, 'idle' | 'stopping'>>({});
  const [deleteStates, setDeleteStates] = useState<Record<number, 'idle' | 'deleting'>>({});
  const [actionErrors, setActionErrors] = useState<Record<number, string>>({});
  const [rerunDraft, setRerunDraft] = useState<RerunDraft | null>(null);
  const [uploadDraft, setUploadDraft] = useState<UploadDraft | null>(null);
  const [biliAccounts, setBiliAccounts] = useState<BiliAccountOption[]>([]);
  const [biliAccountsLoading, setBiliAccountsLoading] = useState(false);
  const [biliAccountsError, setBiliAccountsError] = useState<string | null>(null);
  const [translationModels, setTranslationModels] = useState<ModelCatalogItem[]>([]);
  const [translationModelsLoading, setTranslationModelsLoading] = useState(false);
  const [defaultSubmissionCopyright, setDefaultSubmissionCopyright] = useState('2');
  const uploadDraftVideoId = uploadDraft?.video.id;
  const hasUploadDraft = Boolean(uploadDraft);

const fmtAccountLastUsed = (value?: string) => {
  if (!value) return t('Unused');
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return t('Unused');
  return date.toLocaleString(locale, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
};

  const selectableTranslationModels = translationModels.filter((item) => (
    typeof item.id === 'string'
    && item.id.trim().length > 0
    && !item.locked
  ));

  useEffect(() => {
    let cancelled = false;
    setTranslationModelsLoading(true);

    void (async () => {
      try {
        const result = await agentApi.getInfo();
        const fetched = (result.available_models ?? []).map((item) => ({
          id: item.id,
          label: item.label,
          description: item.description,
          minTier: item.min_tier,
        }));
        if (!cancelled && fetched.length > 0) {
          setTranslationModels(fetched);
        }
      } catch {
        if (!cancelled) {
          setTranslationModels([]);
        }
      } finally {
        if (!cancelled) {
          setTranslationModelsLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [t]);

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        const settings = await userSettingsApi.getSettings();
        if (cancelled) return;
        const nextValue = String(settings[USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT] ?? '').trim();
        if (isBilibiliSubmissionCopyrightValue(nextValue)) {
          setDefaultSubmissionCopyright(nextValue);
        }
      } catch {
        if (!cancelled) {
          setDefaultSubmissionCopyright('2');
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  // Fetch tab badge counts (lightweight)
  const fetchCounts = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/videos/counts?source_type=manual');
      const data = await res.json();
      if ((data.code === 0 || data.code === 200) && data.data) {
        setTabCounts(data.data);
      }
    } catch { /* ignore */ }
  }, []);

  // Server-side paginated fetch
  const fetchVideos = useCallback(async () => {
    try {
      setRefreshing(true);
      const tabParam = activeTab === 'all' ? '' : `&tab=${activeTab}`;
      const res = await fetch(`/api/v1/videos?source_type=manual&page=${currentPage}&limit=${PAGE_SIZE}${tabParam}`);
      const data = await res.json();
      if ((data.code === 0 || data.code === 200) && data.data) {
        setVideos(data.data.videos ?? []);
        setTotal(data.data.total ?? 0);
        setTotalPages(data.data.total_pages ?? 1);
      } else {
        setVideos([]); setTotal(0); setTotalPages(1);
      }
    } catch {
      setVideos([]);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [activeTab, currentPage]);

  useEffect(() => {
    fetchVideos();
    fetchCounts();
  }, [fetchVideos, fetchCounts]);

  const fetchBiliAccounts = useCallback(async () => {
    setBiliAccountsLoading(true);
    setBiliAccountsError(null);
    try {
      const res = await authFetch('/api/v1/bili-accounts');
      const payload = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error((payload as { message?: string }).message || t('Failed to load Bilibili accounts'));
      }

      const accounts = Array.isArray((payload as { data?: BiliAccountOption[] }).data)
        ? ((payload as { data: BiliAccountOption[] }).data)
        : [];
      setBiliAccounts(accounts);
      return accounts;
    } catch (error: unknown) {
      const message = getErrorMessage(error, t('Failed to load Bilibili accounts'));
      setBiliAccounts([]);
      setBiliAccountsError(message);
      return [] as BiliAccountOption[];
    } finally {
      setBiliAccountsLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (!hasUploadDraft) {
      setBiliAccountsError(null);
      return;
    }

    let cancelled = false;
    void (async () => {
      const accounts = await fetchBiliAccounts();
      if (cancelled) return;
      setUploadDraft((current) => {
        if (!current) return current;
        if (current.accountId > 0) return current;
        const preferred = accounts.find((account) => account.is_primary) ?? accounts[0];
        return {
          ...current,
          accountId: preferred?.id ?? 0,
        };
      });
    })();

    return () => {
      cancelled = true;
    };
  }, [fetchBiliAccounts, hasUploadDraft, uploadDraftVideoId]);

  // Dynamic polling: fast (5s) when processing, slow (30s) otherwise
  useEffect(() => {
    const hasActive = videos.some(v => v.status === '002' || v.status === 'processing');
    const interval = setInterval(() => { fetchVideos(); fetchCounts(); }, hasActive ? 5000 : 30000);
    return () => clearInterval(interval);
  }, [videos, fetchVideos, fetchCounts]);

  const handleRetryStep = async (videoId: number, stepName: string) => {
    try {
      await authFetch(`/api/v1/videos/${videoId}/steps/${encodeURIComponent(stepName)}/retry`, { method: 'POST' });
      fetchVideos();
    } catch { /* ignore */ }
  };

  const handleResume = async (video: Video, payload?: Record<string, unknown>) => {
    setResumeStates(prev => ({ ...prev, [video.id]: 'resuming' }));
    setResumeErrors(prev => { const n = { ...prev }; delete n[video.id]; return n; });
    setActionErrors(prev => { const n = { ...prev }; delete n[video.id]; return n; });
    try {
      const res = await authFetch(`/api/v1/videos/${video.id}/resume`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload ? JSON.stringify(payload) : undefined,
      });
      const data = await res.json();
      if (res.ok && (data.code === 0 || data.code === 200)) {
        setResumeStates(prev => ({ ...prev, [video.id]: 'idle' }));
        setRerunDraft(null);
        fetchVideos();
      } else {
        setResumeStates(prev => ({ ...prev, [video.id]: 'idle' }));
        setResumeErrors(prev => ({ ...prev, [video.id]: data.message || t('Rerun failed') }));
      }
    } catch (error: unknown) {
      setResumeStates(prev => ({ ...prev, [video.id]: 'idle' }));
      setResumeErrors(prev => ({ ...prev, [video.id]: getErrorMessage(error, t('Network error')) }));
    }
  };

  const handleUploadToBilibili = async (video: Video, payload?: Record<string, unknown>) => {
    setUploadStates(prev => ({ ...prev, [video.id]: 'uploading' }));
    setUploadErrors(prev => { const n = { ...prev }; delete n[video.id]; return n; });
    try {
      const res = await authFetch(`/api/v1/videos/${video.id}/upload-bilibili`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload ? JSON.stringify(payload) : undefined,
      });
      const data = await res.json();
      if (res.ok && (data.code === 0 || data.code === 200)) {
        setUploadStates(prev => ({ ...prev, [video.id]: 'done' }));
        setUploadDraft(null);
        fetchVideos();
      } else {
        setUploadStates(prev => ({ ...prev, [video.id]: 'error' }));
        setUploadErrors(prev => ({ ...prev, [video.id]: data.message || t('Upload failed') }));
      }
    } catch (error: unknown) {
      setUploadStates(prev => ({ ...prev, [video.id]: 'error' }));
      setUploadErrors(prev => ({ ...prev, [video.id]: getErrorMessage(error, t('Network error')) }));
    }
  };

  const handleStop = async (video: Video) => {
    setStopStates(prev => ({ ...prev, [video.id]: 'stopping' }));
    setActionErrors(prev => { const n = { ...prev }; delete n[video.id]; return n; });
    try {
      const res = await authFetch(`/api/v1/videos/${video.id}/stop`, { method: 'POST' });
      const data = await res.json();
      setStopStates(prev => ({ ...prev, [video.id]: 'idle' }));
      if (res.ok && (data.code === 0 || data.code === 200)) {
        fetchVideos();
        return;
      }
      setActionErrors(prev => ({ ...prev, [video.id]: data.message || t('Stop failed') }));
    } catch (error: unknown) {
      setStopStates(prev => ({ ...prev, [video.id]: 'idle' }));
      setActionErrors(prev => ({ ...prev, [video.id]: getErrorMessage(error, t('Network error')) }));
    }
  };

  const handleDelete = async (video: Video) => {
    if (!window.confirm(t('Are you sure you want to delete task {title}?', { title: video.title || video.video_id }))) {
      return;
    }
    setDeleteStates(prev => ({ ...prev, [video.id]: 'deleting' }));
    setActionErrors(prev => { const n = { ...prev }; delete n[video.id]; return n; });
    try {
      const res = await authFetch(`/api/v1/videos/${video.id}`, { method: 'DELETE' });
      const data = await res.json().catch(() => ({ code: 0 }));
      setDeleteStates(prev => ({ ...prev, [video.id]: 'idle' }));
      if (res.ok && (data.code === 0 || data.code === 200 || data.code === undefined)) {
        fetchVideos();
        fetchCounts();
        return;
      }
      setActionErrors(prev => ({ ...prev, [video.id]: data.message || t('Delete failed') }));
    } catch (error: unknown) {
      setDeleteStates(prev => ({ ...prev, [video.id]: 'idle' }));
      setActionErrors(prev => ({ ...prev, [video.id]: getErrorMessage(error, t('Network error')) }));
    }
  };

  const openRerunModal = (video: Video, mode: RerunMode = 'voice') => {
    setRerunDraft({
      video,
      mode,
      translationModel: 'default',
      voiceName: video.speech_voice_name || DEFAULT_SPEECH_SYNTHESIS_CONFIG.voice_name,
    });
  };

  const submitRerunDraft = async () => {
    if (!rerunDraft) return;
    const payload: Record<string, unknown> = {
      restart_from_step: rerunDraft.mode === 'voice' ? 'SynthesizeSubtitleAudio' : 'LLMTranslate',
      task_chain_settings: buildRerunTaskChainSettings(rerunDraft.mode),
    };
    if (rerunDraft.mode !== 'voice') {
      const modelName = rerunDraft.translationModel.trim();
      if (modelName && modelName !== 'default') {
        payload.translation_config = { model_name: modelName };
      }
    }
    if (rerunDraft.mode !== 'translation-only') {
      const voiceCatalog = await loadTTSVoiceCatalog().catch(() => null);
      const selectedVoice = findTTSVoiceByShortName(voiceCatalog, rerunDraft.voiceName);

      payload.speech_synthesis_config = {
        ...DEFAULT_SPEECH_SYNTHESIS_CONFIG,
        language: selectedVoice?.locale || DEFAULT_SPEECH_SYNTHESIS_CONFIG.language,
        voice_name: rerunDraft.voiceName,
      };
    }
    await handleResume(rerunDraft.video, payload);
  };

  const openUploadModal = (video: Video) => {
    setUploadDraft({
      video,
      accountId: 0,
      copyright: defaultSubmissionCopyright,
      title: video.generated_title || video.title || video.video_id,
      description: video.generated_desc || video.description || '',
      tags: buildPreferredUploadTags(video),
      cover: resolvePreferredUploadCover(video.video_path, video.thumbnail),
    });
  };

  const submitUploadDraft = async () => {
    if (!uploadDraft) return;
    if (!uploadDraft.accountId) {
      setUploadErrors((prev) => ({ ...prev, [uploadDraft.video.id]: t('Please choose the Bilibili account for this submission') }));
      return;
    }
    await handleUploadToBilibili(uploadDraft.video, {
      account_id: uploadDraft.accountId,
      copyright: Number(uploadDraft.copyright),
      title: uploadDraft.title.trim(),
      description: uploadDraft.description.trim(),
      tags: uploadDraft.tags.trim(),
      cover: uploadDraft.cover.trim(),
    });
  };

  const handleTabChange = (tab: TabType) => {
    if (tab === activeTab) return;
    setActiveTab(tab);
    setCurrentPage(1);
    setExpandedIds(new Set());
  };

  const handlePageChange = (p: number) => {
    if (p === currentPage || p < 1 || p > totalPages) return;
    setCurrentPage(p);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  // Smart page number list with ellipsis
  const pageNumbers = (): (number | '...')[] => {
    if (totalPages <= 7) return Array.from({ length: totalPages }, (_, i) => i + 1);
    const pages: (number | '...')[] = [1];
    if (currentPage > 3) pages.push('...');
    for (let i = Math.max(2, currentPage - 1); i <= Math.min(totalPages - 1, currentPage + 1); i++) pages.push(i);
    if (currentPage < totalPages - 2) pages.push('...');
    pages.push(totalPages);
    return pages;
  };

  const tabs = [
    { key: 'all',        label: 'All',   count: tabCounts.all },
    { key: 'processing', label: 'Processing', count: tabCounts.processing },
    { key: 'completed',  label: 'Completed', count: tabCounts.completed },
    { key: 'failed',     label: 'Failed',   count: tabCounts.failed },
  ] as const;

  const getStepDesc = (v: Video) => {
    const steps = v.task_steps ?? [];
    const running = steps.find(s => s.status === 'running');
    if (running) return running.progress_text || t('Running: {step}', { step: t(stepLabelKey(running.step_name)) });
    const failed = steps.find(s => s.status === 'failed');
    if (failed) return t('Preparation failed. Check the task steps.');
    if (v.status === 'paused') return t('The task was stopped. You can continue or rerun it with a different config.');
    if (steps.length > 0 && steps.every(s => s.status === 'completed' || s.status === 'skipped'))
      return t('All steps completed');
    return t('Waiting to start');
  };

  if (loading) return (
    <div className="flex items-center justify-center h-64">
      <div className="text-center">
        <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4" />
        <p className="text-gray-500 text-sm">{t('Loading task data...')}</p>
      </div>
    </div>
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-gray-900">{t('Task management')}</h2>
        <button
          onClick={fetchVideos}
          disabled={refreshing}
          className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
          {t('Refresh')}
        </button>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <div className="flex space-x-6">
          {tabs.map(tab => (
            <button
              key={tab.key}
              onClick={() => { handleTabChange(tab.key as TabType); }}
              className={`pb-3 px-1 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.key
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {t(tab.label)}
              {tab.count > 0 && (
                <span className={`ml-2 text-xs px-1.5 py-0.5 rounded ${
                  activeTab === tab.key ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-600'
                }`}>
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Video list */}
      <div>
        <h3 className="text-base font-semibold text-gray-800 mb-3">{t('Video processing pipeline')}</h3>
        <div className="space-y-3">
          {videos.length === 0 ? (
            <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-400">{t('No tasks yet')}</div>
          ) : videos.map(video => {
            const st = getVideoStatus(video.status);
            const steps = video.task_steps ?? [];
            const completedCount = steps.filter(s => s.status === 'completed' || s.status === 'skipped').length;
            const progress = calculateStepProgress(steps);
            const isCollapsed = !expandedIds.has(video.id);
            const hasSteps = steps.length > 0;
            const toggleCollapse = () =>
              setExpandedIds(prev => {
                const next = new Set(prev);
                if (next.has(video.id)) {
                  next.delete(video.id);
                } else {
                  next.add(video.id);
                }
                return next;
              });

            return (
              <div key={video.id} className="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
                {/* card header */}
                <div className="p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex-1 min-w-0">
                      {/* title row */}
                      <div className="flex items-center gap-2 mb-1.5 flex-wrap">
                        <span className="font-semibold text-gray-900 truncate max-w-xs text-sm">
                          {video.title || video.video_id}
                        </span>
                        <span className={`flex items-center gap-1.5 text-xs font-medium px-2.5 py-0.5 rounded-full ${st.color}`}>
                          <span className={`w-1.5 h-1.5 rounded-full ${st.dot}`} />
                          {t(st.label)}
                        </span>
                        {video.preferred_resolution && (
                          <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-slate-100 text-slate-700 border border-slate-200">
                            {video.preferred_resolution}
                          </span>
                        )}
                        {video.speech_voice_name && (
                          <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-amber-50 text-amber-700 border border-amber-200">
                            {t('Voice:')} {video.speech_voice_name}
                          </span>
                        )}
                        {video.bili_bvid && (
                          <a
                            href={`https://www.bilibili.com/video/${video.bili_bvid}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-blue-600 hover:underline"
                          >
                            {video.bili_bvid}
                          </a>
                        )}
                      </div>

                      {/* step desc */}
                      <p className="text-sm text-gray-500 mb-2">{getStepDesc(video)}</p>

                      {/* progress bar */}
                      {hasSteps && (
                        <div className="flex items-center gap-2 mb-2">
                          <div className="flex-1 max-w-xs bg-gray-100 rounded-full h-1.5">
                            <div
                              className={`h-1.5 rounded-full transition-all ${
                                progress === 100 ? 'bg-emerald-500' : 'bg-blue-500'
                              }`}
                              style={{ width: `${progress}%` }}
                            />
                          </div>
                          <span className="text-xs text-gray-400">{completedCount}/{steps.length}</span>
                          <span className="text-xs text-gray-400">{progress}%</span>
                        </div>
                      )}

                      {/* meta row */}
                      <div className="flex items-center gap-4 text-xs text-gray-400">
                        <span>{t('Video ID:')} {video.video_id}</span>
                        {video.video_size_bytes ? <span>{t('Size:')} {formatFileSize(video.video_size_bytes)}</span> : null}
                        <span>{t('Created:')} {new Date(video.created_at).toLocaleString(locale, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                        <span>{t('Updated:')} {new Date(video.updated_at).toLocaleString(locale, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                      </div>
                    </div>

                    {/* collapse toggle */}
                    {hasSteps && (
                      <button
                        onClick={toggleCollapse}
                        className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 px-2 py-1 rounded hover:bg-gray-100 transition-colors shrink-0 mt-0.5"
                      >
                        {isCollapsed
                          ? <><ChevronRight className="w-3.5 h-3.5" />{t('Expand')}</>
                          : <><ChevronDown className="w-3.5 h-3.5" />{t('Collapse')}</>}
                      </button>
                    )}

                    {/* 重新处理按钮：任务失败时显示 */}
                    {(video.status === '004' || video.status === 'failed' || video.status === 'paused') && (
                      <div className="flex flex-col items-end gap-1 shrink-0">
                        <button
                          disabled={resumeStates[video.id] === 'resuming'}
                          onClick={() => openRerunModal(video, 'voice')}
                          className="flex items-center gap-1.5 text-xs text-white bg-orange-500 hover:bg-orange-600 disabled:opacity-60 disabled:cursor-not-allowed px-3 py-1.5 rounded-lg transition-colors"
                          title={t('Continue processing or rerun with a different config')}
                        >
                          <RotateCcw className={`w-3.5 h-3.5 ${resumeStates[video.id] === 'resuming' ? 'animate-spin' : ''}`} />
                          {resumeStates[video.id] === 'resuming' ? t('Processing...') : t('Continue / rerun')}
                        </button>
                        {resumeErrors[video.id] && (
                          <span className="text-xs text-red-500">{resumeErrors[video.id]}</span>
                        )}
                      </div>
                    )}

                    {st.tab === 'completed' && (
                      <button
                        onClick={() => openRerunModal(video, 'voice')}
                        className="flex items-center gap-1.5 text-xs text-white bg-indigo-500 hover:bg-indigo-600 px-3 py-1.5 rounded-lg transition-colors shrink-0"
                        title={t('Rerun with a different model or voice without downloading again')}
                      >
                        <SlidersHorizontal className="w-3.5 h-3.5" />
                        {t('Rerun with new config')}
                      </button>
                    )}

                    {/* 上传到B站按钮：始终显示，由后端决定是否允许执行 */}
                    <div className="flex flex-col items-end gap-1 shrink-0">
                      <button
                        disabled={uploadStates[video.id] === 'uploading'}
                        onClick={() => openUploadModal(video)}
                        className="flex items-center gap-1.5 text-xs text-white bg-pink-500 hover:bg-pink-600 disabled:opacity-60 disabled:cursor-not-allowed px-3 py-1.5 rounded-lg transition-colors"
                        title={t('Edit the title, description, and cover before uploading')}
                      >
                        <Upload className={`w-3.5 h-3.5 ${uploadStates[video.id] === 'uploading' ? 'animate-spin' : ''}`} />
                        {uploadStates[video.id] === 'uploading' ? t('Uploading...') : t('Edit and upload')}
                      </button>
                      {video.bili_bvid && (
                        <span className="text-xs text-teal-600 bg-teal-50 border border-teal-200 px-2 py-1 rounded-lg">{t('Uploaded to Bilibili')}</span>
                      )}
                      {uploadStates[video.id] === 'error' && (
                        <span className="text-xs text-red-500">{uploadErrors[video.id]}</span>
                      )}
                    </div>

                    {(video.status === '002' || video.status === 'processing') && (
                      <button
                        disabled={stopStates[video.id] === 'stopping'}
                        onClick={() => handleStop(video)}
                        className="flex items-center gap-1.5 text-xs text-white bg-slate-600 hover:bg-slate-700 disabled:opacity-60 disabled:cursor-not-allowed px-3 py-1.5 rounded-lg transition-colors shrink-0"
                        title={t('Stop the current task pipeline')}
                      >
                        <Square className="w-3.5 h-3.5" />
                        {stopStates[video.id] === 'stopping' ? t('Stopping...') : t('Stop')}
                      </button>
                    )}

                    <button
                      disabled={deleteStates[video.id] === 'deleting'}
                      onClick={() => handleDelete(video)}
                      className="flex items-center gap-1.5 text-xs text-red-600 bg-red-50 hover:bg-red-100 disabled:opacity-60 disabled:cursor-not-allowed px-3 py-1.5 rounded-lg transition-colors shrink-0"
                      title={t('Delete task pipeline')}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                      {deleteStates[video.id] === 'deleting' ? t('Deleting...') : t('Delete')}
                    </button>

                    {/* 视频预览按钮 */}
                    {video.video_path && (
                      <button
                        onClick={() =>
                          setPlayerVideo({
                            title: video.generated_title || video.title || video.video_id,
                            videoUrl: videoPathToUrl(video.video_path!),
                            subtitleUrl: guessSubtitleUrl(video),
                            audioBaseUrl: guessSubtitleAudioBaseUrl(video),
                          })
                        }
                        className="flex items-center gap-1.5 text-xs text-white bg-blue-600 hover:bg-blue-500 px-3 py-1.5 rounded-lg transition-colors shrink-0"
                        title={t('Preview local video')}
                      >
                        <PlayCircle className="w-3.5 h-3.5" />
                        {t('Preview')}
                      </button>
                    )}
                  </div>
                  {actionErrors[video.id] && (
                    <p className="mt-2 text-xs text-red-500">{actionErrors[video.id]}</p>
                  )}
                </div>

                {/* step list — always visible unless collapsed */}
                {hasSteps && !isCollapsed && (
                  <div className="border-t border-gray-100 bg-gray-50 px-5 py-4">
                    <TaskStepList
                      steps={steps}
                      onRetry={name => handleRetryStep(video.id, name)}
                    />
                  </div>
                )}

                {/* no steps placeholder (only when not collapsed) */}
                {!hasSteps && (
                  <div className="border-t border-gray-100 bg-gray-50 px-5 py-3">
                    <p className="text-xs text-gray-400 text-center">{t('No task-step records yet. They will appear automatically after processing starts.')}</p>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-2">
          <span className="text-sm text-gray-500">
            {t('Total {total} items · Page {current} / {pages}', { total, current: currentPage, pages: totalPages })}
          </span>
          <div className="flex items-center gap-1">
            {/* First */}
            <button
              onClick={() => handlePageChange(1)}
              disabled={currentPage === 1}
              className="w-8 h-8 flex items-center justify-center rounded border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-30 text-xs"
              title={t('First page')}
            >«</button>
            {/* Prev */}
            <button
              onClick={() => handlePageChange(currentPage - 1)}
              disabled={currentPage === 1}
              className="w-8 h-8 flex items-center justify-center rounded border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-30 text-sm"
              title={t('Previous page')}
            >‹</button>
            {/* Page numbers */}
            {pageNumbers().map((p, i) =>
              p === '...'
                ? <span key={`e${i}`} className="w-8 h-8 flex items-center justify-center text-gray-400 text-sm">…</span>
                : <button
                    key={p}
                    onClick={() => handlePageChange(p as number)}
                    className={`w-8 h-8 flex items-center justify-center rounded border text-sm transition-colors ${
                      currentPage === p
                        ? 'border-blue-500 bg-blue-500 text-white font-medium'
                        : 'border-gray-200 text-gray-600 hover:bg-gray-50'
                    }`}
                  >{p}</button>
            )}
            {/* Next */}
            <button
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              className="w-8 h-8 flex items-center justify-center rounded border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-30 text-sm"
              title={t('Next page')}
            >›</button>
            {/* Last */}
            <button
              onClick={() => handlePageChange(totalPages)}
              disabled={currentPage === totalPages}
              className="w-8 h-8 flex items-center justify-center rounded border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-30 text-xs"
              title={t('Last page')}
            >»</button>
          </div>
        </div>
      )}

      {/* Scheduling info */}
      <div>
        <h3 className="text-base font-semibold text-gray-800 mb-3">{t('Automation schedule')}</h3>
        <div className="bg-white rounded-xl border border-gray-200 divide-y divide-gray-100">
          <div className="flex items-start gap-3 p-4">
            <div className="w-8 h-8 bg-blue-100 rounded-lg flex items-center justify-center shrink-0 mt-0.5">
              <Play className="w-4 h-4 text-blue-600" />
            </div>
            <div>
              <h4 className="text-sm font-medium text-gray-900 mb-0.5">{t('Video processing pipeline')}</h4>
              <p className="text-xs text-gray-500">
                {t('The base order is Initialize → Download video. After that, the submitted task config determines whether download thumbnail, extract audio, transcribe subtitles, translate subtitles, synthesize subtitle audio, and save results continue to run.')}
              </p>
              <p className="text-xs text-gray-400 mt-1">{t('Currently processing {processing} | Pending 0', { processing: tabCounts.processing })}</p>
            </div>
          </div>
          <div className="flex items-start gap-3 p-4">
            <div className="w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center shrink-0 mt-0.5">
              <Upload className="w-4 h-4 text-purple-600" />
            </div>
            <div>
              <h4 className="text-sm font-medium text-gray-900 mb-0.5">{t('Bilibili auto-upload schedule')}</h4>
              <p className="text-xs text-gray-500">
                {t('Every')} <span className="font-medium text-purple-700">30 {t('minutes')}</span>{t(' the system checks completed videos and uploads them to Bilibili without duplicating uploads. You can also click ')}<span className="font-medium text-pink-600">{t('Upload to Bilibili')}</span>{t(' on a task card to trigger it manually immediately.')}
              </p>
            </div>
          </div>
          {tabCounts.failed > 0 && (
            <div className="flex items-start gap-3 p-4 bg-red-50">
              <div className="w-8 h-8 bg-red-100 rounded-lg flex items-center justify-center shrink-0 mt-0.5">
                <AlertCircle className="w-4 h-4 text-red-600" />
              </div>
              <div>
                <h4 className="text-sm font-medium text-red-900 mb-0.5">{t('Failed task reminder')}</h4>
                <p className="text-xs text-red-700">
                  {t('There are currently {count} failed tasks. Expand them to view details and retry the failed steps manually.', { count: tabCounts.failed })}
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* 视频播放弹窗 */}
      {playerVideo && (
        <VideoPlayerModal
          isOpen
          onClose={() => setPlayerVideo(null)}
          videoUrl={playerVideo.videoUrl}
          title={playerVideo.title}
          subtitleUrl={playerVideo.subtitleUrl}
          audioBaseUrl={playerVideo.audioBaseUrl}
        />
      )}

      <Modal
        isOpen={Boolean(rerunDraft)}
        onClose={() => setRerunDraft(null)}
        title={t('Retry after changing config')}
        description={t('The video will not be downloaded again. Execution resumes from the selected step.')}
        className="max-w-2xl"
      >
        {rerunDraft && (
          <div className="space-y-4">
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
              {t('Current task: {title}', { title: rerunDraft.video.title || rerunDraft.video.video_id })}
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <label className="space-y-2 text-sm text-slate-700">
                <span className="font-medium">{t('Rerun scope')}</span>
                <select
                  value={rerunDraft.mode}
                  onChange={(event) => setRerunDraft(current => current ? { ...current, mode: event.target.value as RerunMode } : current)}
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-blue-400"
                >
                  <option value="voice">{t('Voice only')}</option>
                  <option value="translation-only">{t('Translation only')}</option>
                  <option value="translation">{t('Translate and voice')}</option>
                </select>
              </label>
              {rerunDraft.mode !== 'translation-only' ? (
                <label className="space-y-2 text-sm text-slate-700">
                  <span className="font-medium">{t('Subtitle voice')}</span>
                  <VoicePicker
                    value={rerunDraft.voiceName}
                    onChange={(nextValue) => setRerunDraft(current => current ? { ...current, voiceName: nextValue } : current)}
                    provider="auto"
                  />
                </label>
              ) : (
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-500">
                  {t('Translation-only mode will not run subtitle voice synthesis.')}
                </div>
              )}
            </div>
            {rerunDraft.mode !== 'voice' && (
              <label className="space-y-2 text-sm text-slate-700 block">
                <span className="font-medium">{t('Translation model')}</span>
                <select
                  value={rerunDraft.translationModel}
                  onChange={(event) => setRerunDraft(current => current ? { ...current, translationModel: event.target.value } : current)}
                  disabled={translationModelsLoading && selectableTranslationModels.length === 0}
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-blue-400 disabled:opacity-60"
                >
                  <option value="default">{t('Follow system default')}</option>
                  {selectableTranslationModels.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.label}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {resumeErrors[rerunDraft.video.id] && (
              <p className="text-sm text-red-500">{resumeErrors[rerunDraft.video.id]}</p>
            )}
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setRerunDraft(null)}
                className="rounded-lg border border-slate-200 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50"
              >
                {t('Cancel')}
              </button>
              <button
                onClick={submitRerunDraft}
                disabled={resumeStates[rerunDraft.video.id] === 'resuming'}
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {resumeStates[rerunDraft.video.id] === 'resuming' ? t('Submitting...') : t('Start rerun')}
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        isOpen={Boolean(uploadDraft)}
        onClose={() => setUploadDraft(null)}
        title={t('Edit before uploading to Bilibili')}
        description={t('Before submitting, you can override the title, description, tags, and cover path.')}
        className="max-w-2xl"
      >
        {uploadDraft && (
          <div className="space-y-4">
            <label className="space-y-2 text-sm text-slate-700 block">
              <span className="font-medium">{t('Submission account')}</span>
              <select
                value={uploadDraft.accountId || ''}
                onChange={(event) => {
                  const nextId = Number(event.target.value);
                  setUploadDraft((current) => current ? { ...current, accountId: nextId } : current);
                  setUploadErrors((prev) => {
                    const next = { ...prev };
                    delete next[uploadDraft.video.id];
                    return next;
                  });
                }}
                disabled={biliAccountsLoading || biliAccounts.length === 0}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-pink-400 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-400"
              >
                <option value="">{biliAccountsLoading ? t('Loading accounts...') : biliAccounts.length === 0 ? t('No Bilibili accounts available') : t('Choose a submission account')}</option>
                {biliAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.bili_name} ({account.bili_mid}){account.is_primary ? ` · ${t('Primary account')}` : ''}
                  </option>
                ))}
              </select>
              <div className="rounded-lg border border-pink-100 bg-pink-50/60 px-3 py-2 text-xs text-slate-600">
                {uploadDraft.accountId > 0 ? (
                  (() => {
                    const selectedAccount = biliAccounts.find((account) => account.id === uploadDraft.accountId);
                    if (!selectedAccount) {
                      return t('This submission will use the Bilibili account you selected.');
                    }
                    return t('Using {name}{primary} for this submission. Last used: {date}', {
                      name: selectedAccount.bili_name,
                      primary: selectedAccount.is_primary ? ` (${t('Primary account')})` : '',
                      date: fmtAccountLastUsed(selectedAccount.last_used_at),
                    });
                  })()
                ) : t('Choose the Bilibili account to use for this submission.')}
              </div>
              {biliAccountsError && (
                <p className="text-sm text-red-500">{biliAccountsError}</p>
              )}
              {!biliAccountsLoading && biliAccounts.length === 0 && !biliAccountsError && (
                <p className="text-sm text-amber-600">{t('There are no available Bilibili bindings for the current account. Go to account management and bind one first.')}</p>
              )}
            </label>
            <fieldset className="space-y-2 text-sm text-slate-700">
              <legend className="font-medium">{t('Submission type')}</legend>
              <div className="flex items-center gap-6">
                {BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS.map((option) => (
                  <label key={option.value} className="flex items-center gap-2 text-sm text-slate-700">
                    <input
                      type="radio"
                      name="bilibili-copyright"
                      value={option.value}
                      checked={uploadDraft.copyright === option.value}
                      onChange={(event) => setUploadDraft((current) => current ? { ...current, copyright: event.target.value } : current)}
                      className="h-4 w-4 border-slate-300 text-pink-600 focus:ring-pink-400"
                    />
                    <span>{option.label}</span>
                  </label>
                ))}
              </div>
              <p className="text-xs text-slate-500">{t('Default values come from the global upload settings page. They only override this single upload.')}</p>
            </fieldset>
            <label className="space-y-2 text-sm text-slate-700 block">
              <span className="font-medium">{t('Title')}</span>
              <input
                value={uploadDraft.title}
                onChange={(event) => setUploadDraft(current => current ? { ...current, title: event.target.value } : current)}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-pink-400"
              />
            </label>
            <label className="space-y-2 text-sm text-slate-700 block">
              <span className="font-medium">{t('Description')}</span>
              <textarea
                value={uploadDraft.description}
                onChange={(event) => setUploadDraft(current => current ? { ...current, description: event.target.value } : current)}
                rows={6}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-pink-400"
              />
            </label>
            <label className="space-y-2 text-sm text-slate-700 block">
              <span className="font-medium">{t('Tags')}</span>
              <input
                value={uploadDraft.tags}
                onChange={(event) => setUploadDraft(current => current ? { ...current, tags: event.target.value } : current)}
                placeholder={t('Separate multiple tags with commas')}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-pink-400"
              />
              <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-500">
                {normalizeTagList(uploadDraft.video.recommended_tags).length > 0 ? (
                  <>
                    {t('Recommended tags:')} {normalizeTagList(uploadDraft.video.recommended_tags).join('、')}
                  </>
                ) : normalizeTagList(uploadDraft.video.generated_tags).length > 0 ? (
                  <>
                    {t('The current default tags come from AI generation:')} {normalizeTagList(uploadDraft.video.generated_tags).join('、')}
                  </>
                ) : t('There are no tags available for this task yet. Leaving it empty falls back to the backend default tags.')}
              </div>
            </label>
            <label className="space-y-2 text-sm text-slate-700 block">
              <span className="font-medium">{t('Cover path')}</span>
              <input
                value={uploadDraft.cover}
                onChange={(event) => setUploadDraft(current => current ? { ...current, cover: event.target.value } : current)}
                placeholder={t('Use the cover path saved by the current task by default')}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 outline-none focus:border-pink-400"
              />
            </label>
            {uploadErrors[uploadDraft.video.id] && (
              <p className="text-sm text-red-500">{uploadErrors[uploadDraft.video.id]}</p>
            )}
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setUploadDraft(null)}
                className="rounded-lg border border-slate-200 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50"
              >
                {t('Cancel')}
              </button>
              <button
                onClick={submitUploadDraft}
                disabled={uploadStates[uploadDraft.video.id] === 'uploading' || biliAccountsLoading || !uploadDraft.accountId}
                className="rounded-lg bg-pink-600 px-4 py-2 text-sm font-medium text-white hover:bg-pink-700 disabled:opacity-60"
              >
                {uploadStates[uploadDraft.video.id] === 'uploading' ? t('Uploading...') : t('Confirm upload')}
              </button>
            </div>
          </div>
        )}
      </Modal>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Total tasks', value: tabCounts.all,        color: 'text-gray-900' },
          { label: 'Processing',   value: tabCounts.processing, color: 'text-blue-600' },
          { label: 'Completed',   value: tabCounts.completed,  color: 'text-emerald-600' },
          { label: 'Failed',     value: tabCounts.failed,     color: 'text-red-600' },
        ].map(s => (
          <div key={s.label} className="bg-white rounded-xl border border-gray-200 p-4">
            <div className="text-xs text-gray-500 mb-1">{t(s.label)}</div>
            <div className={`text-2xl font-bold ${s.color}`}>{s.value}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

