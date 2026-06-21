'use client';

import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import NextLink from 'next/link';
import {
  Plus,
  Sparkles,
  Video as VideoIcon,
  Rss,
  Settings,
  ChevronDown,
  ArrowRight,
  CheckCircle2,
  Loader2,
  Globe,
  TrendingUp,
  HardDrive,
  MemoryStick,
  RotateCcw,
  Check,
  ChevronsUpDown,
} from 'lucide-react';
import { FaYoutube } from 'react-icons/fa';
import { FaBilibili, FaXTwitter, FaTiktok } from 'react-icons/fa6';
import toast from 'react-hot-toast';
import { api, SystemUsageResponse, Video, VideoTabCounts } from '@/lib/api';
import { agentApi, AgentInfo } from '@/lib/api/agent';
import { Skeleton } from '@/components/ui/Skeleton';
import InlineTaskTracker from '@/app/dashboard/assistant/_components/InlineTaskTracker';
import { useAuth } from '@/contexts/AuthContext';
import { useI18n } from '@/contexts/I18nContext';
import { useUserSettings } from '@/hooks/useUserSettings';
import { useMembership } from '@/hooks/useMembership';
import UpgradePromptCard from '@/components/membership/UpgradePromptCard';
import { translateClientText } from '@/lib/i18n';
import {
  DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
  DEFAULT_SPEECH_SYNTHESIS_CONFIG,
  DEFAULT_TASK_CHAIN_SETTINGS,
  estimateCredits,
  getVideoSubmissionErrorMessage,
  getStoredSubtitleAudioTTSConfigValue,
  inspectVideoSubmissionUrl,
  normalizeSpeechSynthesisConfig,
  parseSpeechSynthesisConfig,
  PlaylistSubmissionConfig,
  PreferredResolution,
  TaskChainSettings,
  submitVideoToQueue,
  VideoQueueSubmissionResult,
} from '@/lib/video-submission';

const DEFAULT_FULL_PIPELINE_CREDITS_PER_VIDEO = 50;

const RESOLUTION_OPTIONS: Array<{ value: PreferredResolution; labelKey: string; label: string }> = [
  { value: 'best', labelKey: 'Auto best', label: 'Auto best' },
  { value: '720p', label: '720p' },
  { value: '1080p', label: '1080p' },
  { value: '1440p', label: '1440p' },
  { value: '2160p', label: '2160p / 4K' },
];

const DASHBOARD_SUBMIT_SETTINGS_STORAGE_KEY = 'ytb2bili:dashboard-submit-settings';
const DASHBOARD_SUBMIT_RESOLUTION_STORAGE_KEY = `${DASHBOARD_SUBMIT_SETTINGS_STORAGE_KEY}:resolution`;
const DASHBOARD_SUBMIT_TASK_CHAIN_STORAGE_KEY = `${DASHBOARD_SUBMIT_SETTINGS_STORAGE_KEY}:task-chain`;
const DASHBOARD_SUBMIT_PLAYLIST_STORAGE_KEY = `${DASHBOARD_SUBMIT_SETTINGS_STORAGE_KEY}:playlist`;
const USER_SETTING_KEY_PREFERRED_RESOLUTION = 'preferred_resolution';
const USER_SETTING_KEY_TASK_CHAIN_SETTINGS = 'task_chain_settings';
const USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG = 'playlist_submission_config';

const TASK_CHAIN_ITEMS: Array<{
  key: keyof TaskChainSettings;
  labelKey: string;
  descriptionKey: string;
}> = [
  { key: 'download_thumbnail', labelKey: 'Download thumbnail', descriptionKey: 'Save the video cover for later display and upload.' },
  { key: 'transcribe', labelKey: 'Transcribe subtitles', descriptionKey: 'Extract audio and generate timestamped subtitles. Turning this off also skips translation and subtitle voiceover.' },
  { key: 'translate_subtitles', labelKey: 'AI subtitle translation', descriptionKey: 'Translate subtitles into Chinese output and generate a translated SRT.' },
  { key: 'synthesize_subtitle_audio', labelKey: 'Synthesize subtitle voice', descriptionKey: 'Generate TTS audio for subtitles. Fresh runs depend on transcription, while reruns can reuse existing subtitles.' },
];

function normalizeTaskChainSettings(settings?: Partial<TaskChainSettings> | null): TaskChainSettings {
  const normalized: TaskChainSettings = {
    ...DEFAULT_TASK_CHAIN_SETTINGS,
    ...settings,
  };

  if (!normalized.transcribe) {
    normalized.translate_subtitles = false;
    normalized.synthesize_subtitle_audio = false;
  }

  return normalized;
}

function enabledTaskChainLabels(settings: TaskChainSettings): string[] {
  return TASK_CHAIN_ITEMS
    .filter((item) => settings[item.key])
    .map((item) => translateClientText(item.labelKey));
}

function taskChainSummary(settings: TaskChainSettings): string {
  const labels = enabledTaskChainLabels(settings);
  return labels.length > 0 ? labels.join(' / ') : translateClientText('Base pipeline');
}

function taskChainExecutionSummary(settings: TaskChainSettings): string {
  const steps = [translateClientText('Download video')];

  if (settings.download_thumbnail) {
    steps.push(translateClientText('Download thumbnail'));
  }
  if (settings.transcribe) {
    steps.push(translateClientText('Extract audio'), translateClientText('Transcribe subtitles'));
  }
  if (settings.translate_subtitles) {
    steps.push(translateClientText('AI translation'));
  }
  if (settings.synthesize_subtitle_audio) {
    steps.push(translateClientText('Subtitle voiceover'));
  }

  steps.push(translateClientText('Save results'));
  return steps.join(' → ');
}

function enabledTaskChainCount(settings: TaskChainSettings): number {
  return TASK_CHAIN_ITEMS.filter((item) => settings[item.key]).length;
}

function taskChainSettingsEqual(left: TaskChainSettings, right: TaskChainSettings): boolean {
  return (
    left.download_thumbnail === right.download_thumbnail &&
    left.transcribe === right.transcribe &&
    left.translate_subtitles === right.translate_subtitles &&
    left.synthesize_subtitle_audio === right.synthesize_subtitle_audio
  );
}

function normalizePlaylistSubmissionConfig(config?: Partial<PlaylistSubmissionConfig> | null): PlaylistSubmissionConfig {
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

function playlistSubmissionConfigEqual(left: PlaylistSubmissionConfig, right: PlaylistSubmissionConfig): boolean {
  return left.enabled === right.enabled && left.start_index === right.start_index && left.max_items === right.max_items;
}

function readDashboardSubmitResolution(): PreferredResolution | null {
  if (typeof window === 'undefined') return null;

  try {
    const raw = localStorage.getItem(DASHBOARD_SUBMIT_RESOLUTION_STORAGE_KEY);
    if (!raw) return null;

    return RESOLUTION_OPTIONS.some((option) => option.value === raw as PreferredResolution)
      ? raw as PreferredResolution
      : null;
  } catch {
    return null;
  }
}

function writeDashboardSubmitResolution(value: PreferredResolution) {
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(DASHBOARD_SUBMIT_RESOLUTION_STORAGE_KEY, value);
  } catch {
    // Keep in-memory state even if local persistence fails.
  }
}

function readDashboardTaskChainSettings(): TaskChainSettings | null {
  if (typeof window === 'undefined') return null;

  try {
    const raw = localStorage.getItem(DASHBOARD_SUBMIT_TASK_CHAIN_STORAGE_KEY);
    if (!raw) return null;

    return normalizeTaskChainSettings(JSON.parse(raw) as Partial<TaskChainSettings>);
  } catch {
    return null;
  }
}

function writeDashboardTaskChainSettings(value: TaskChainSettings) {
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(DASHBOARD_SUBMIT_TASK_CHAIN_STORAGE_KEY, JSON.stringify(value));
  } catch {
    // Keep in-memory state even if local persistence fails.
  }
}

function readDashboardPlaylistSubmissionConfig(): PlaylistSubmissionConfig | null {
  if (typeof window === 'undefined') return null;

  try {
    const raw = localStorage.getItem(DASHBOARD_SUBMIT_PLAYLIST_STORAGE_KEY);
    if (!raw) return null;

    return normalizePlaylistSubmissionConfig(JSON.parse(raw) as Partial<PlaylistSubmissionConfig>);
  } catch {
    return null;
  }
}

function writeDashboardPlaylistSubmissionConfig(value: PlaylistSubmissionConfig) {
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(DASHBOARD_SUBMIT_PLAYLIST_STORAGE_KEY, JSON.stringify(value));
  } catch {
    // Keep in-memory state even if local persistence fails.
  }
}

function readSerializedTaskChainSettings(value?: string | null): TaskChainSettings | null {
  if (!value) {
    return null;
  }

  try {
    return normalizeTaskChainSettings(JSON.parse(value) as Partial<TaskChainSettings>);
  } catch {
    return null;
  }
}

function serializeTaskChainSettings(value: TaskChainSettings): string {
  return JSON.stringify(normalizeTaskChainSettings(value));
}

function readSerializedPlaylistSubmissionConfig(value?: string | null): PlaylistSubmissionConfig | null {
  if (!value) {
    return null;
  }

  try {
    return normalizePlaylistSubmissionConfig(JSON.parse(value) as Partial<PlaylistSubmissionConfig>);
  } catch {
    return null;
  }
}

function serializePlaylistSubmissionConfig(value: PlaylistSubmissionConfig): string {
  return JSON.stringify(normalizePlaylistSubmissionConfig(value));
}

// ── helpers ───────────────────────────────────────────────────────────────────

const STATUS_META: Record<string, { labelKey: string; color: string }> = {
  '001': { labelKey: 'Pending', color: 'text-yellow-600 bg-yellow-50 border-yellow-200' },
  '002': { labelKey: 'Processing', color: 'text-blue-600 bg-blue-50 border-blue-200' },
  '003': { labelKey: 'Completed', color: 'text-green-600 bg-green-50 border-green-200' },
  '004': { labelKey: 'Failed',   color: 'text-red-600 bg-red-50 border-red-200' },
};
function statusMeta(s: string) {
  const meta = STATUS_META[s];
  return meta
    ? { label: translateClientText(meta.labelKey), color: meta.color }
    : { label: s, color: 'text-muted-foreground bg-muted border-border' };
}
function fmtDuration(s: number) {
  if (!s) return '';
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  const precision = size >= 100 || unitIndex === 0 ? 0 : size >= 10 ? 1 : 2;
  return `${size.toFixed(precision)} ${units[unitIndex]}`;
}

// ── sub-components ────────────────────────────────────────────────────────────

function StatCard({ label, value, subtitle, icon, accent, loading }: {
  label: string; value: number | string; subtitle?: string; icon: React.ReactNode;
  accent: string; loading: boolean;
}) {
  const bg = accent.replace('text-', 'bg-').replace(/-\d+$/, '-100');
  return (
    <div className="rounded-xl border border-border bg-card p-5 flex items-center gap-4">
      <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${bg}`}>
        <span className={accent}>{icon}</span>
      </div>
      <div>
        <p className="text-xs text-muted-foreground">{label}</p>
        {loading
          ? <Skeleton className="h-6 w-10 mt-0.5" />
          : (
            <>
              <p className="text-2xl font-bold leading-tight">{value}</p>
              {subtitle ? <p className="text-[11px] text-muted-foreground mt-0.5">{subtitle}</p> : null}
            </>
          )}
      </div>
    </div>
  );
}

function ActionCard({ href, icon, title, desc, badge, badgeColor }: {
  href: string; icon: React.ReactNode; title: string; desc: string;
  badge?: string; badgeColor?: string;
}) {
  return (
    <NextLink href={href}
      className="group flex items-start gap-4 rounded-xl border border-border bg-card p-5 hover:border-primary/50 hover:shadow-sm transition-all"
    >
      <div className="w-10 h-10 rounded-lg bg-muted flex items-center justify-center shrink-0 group-hover:bg-primary/10 transition-colors">
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-0.5">
          <p className="font-semibold text-sm">{title}</p>
          {badge && (
            <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full border ${badgeColor}`}>
              {badge}
            </span>
          )}
        </div>
        <p className="text-xs text-muted-foreground">{desc}</p>
      </div>
      <ArrowRight className="w-4 h-4 text-muted-foreground/40 shrink-0 mt-1 group-hover:text-primary group-hover:translate-x-0.5 transition-all" />
    </NextLink>
  );
}

const ResolutionSelector = memo(function ResolutionSelector({
  value,
  onChange,
}: {
  value: PreferredResolution;
  onChange: (value: PreferredResolution) => void;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const selected = useMemo(
    () => RESOLUTION_OPTIONS.find((option) => option.value === value),
    [value],
  );

  useEffect(() => {
    if (!open) return;

    const handlePointerDown = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleEscape);

    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [open]);

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        className="flex h-9 w-full items-center justify-between rounded-lg border border-border bg-background px-3 text-left text-sm text-foreground shadow-sm transition-colors hover:bg-accent"
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate">{selected ? t(selected.labelKey ?? selected.label) : t('Select resolution')}</span>
        <ChevronsUpDown className="ml-3 h-4 w-4 shrink-0 text-muted-foreground" />
      </button>

      {open && (
        <div className="absolute left-0 top-full z-20 mt-1.5 w-full overflow-hidden rounded-xl border border-border bg-popover shadow-[0_18px_40px_rgba(15,23,42,0.18)]">
          <div className="bg-popover p-1.5">
            {RESOLUTION_OPTIONS.map((option) => {
              const active = option.value === value;

              return (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => {
                    onChange(option.value);
                    setOpen(false);
                  }}
                  className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm transition-colors ${active ? 'bg-accent text-foreground' : 'bg-popover text-popover-foreground hover:bg-accent'}`}
                  role="option"
                  aria-selected={active}
                >
                  <span className="flex h-4 w-4 shrink-0 items-center justify-center">
                    {active ? <Check className="h-4 w-4 text-primary" /> : null}
                  </span>
                  <span className="truncate">{t(option.labelKey ?? option.label)}</span>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
});

const SubmitSettingsPanel = memo(function SubmitSettingsPanel({
  preferredResolution,
  playlistSubmissionConfig,
  taskChainSettings,
  enabledTaskCount,
  onResolutionChange,
  onPlaylistConfigChange,
  onTaskChainChange,
  onReset,
}: {
  preferredResolution: PreferredResolution;
  playlistSubmissionConfig: PlaylistSubmissionConfig;
  taskChainSettings: TaskChainSettings;
  enabledTaskCount: number;
  onResolutionChange: (value: PreferredResolution) => void;
  onPlaylistConfigChange: (value: PlaylistSubmissionConfig) => void;
  onTaskChainChange: (value: TaskChainSettings) => void;
  onReset: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="mt-3 overflow-hidden rounded-xl border border-border bg-card">
      <div className="flex h-10 items-center justify-between border-b border-border px-4">
        <span className="text-xs font-medium text-muted-foreground">{t('Submission settings')}</span>
        <button
          type="button"
          onClick={onReset}
          className="flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <RotateCcw className="h-3 w-3" />
          {t('Restore defaults')}
        </button>
      </div>
      <div className="grid gap-4 p-4 md:grid-cols-2">
        <div className="flex flex-col gap-4">
          <div className="rounded-xl border border-border/70 bg-muted/20 p-4">
            <div>
              <p className="text-sm font-medium text-foreground">{t('Download resolution')}</p>
              <p className="mt-0.5 text-xs text-muted-foreground">{t('Prefer filtering video formats by this resolution.')}</p>
            </div>
            <div className="mt-3">
              <ResolutionSelector value={preferredResolution} onChange={onResolutionChange} />
            </div>
            <p className="mt-3 text-[11px] leading-relaxed text-muted-foreground/80">
              {t('Downloads below 720p are still rejected, even if you pick a lower option.')}
            </p>
          </div>

          <div className="rounded-xl border border-border/70 bg-muted/20 p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-sm font-medium text-foreground">{t('Playlist batch submission')}</p>
                <p className="mt-0.5 text-[11px] leading-relaxed text-muted-foreground">
                  {t('When enabled, a YouTube playlist will be split into multiple standalone queue tasks.')}
                </p>
              </div>
              <label className="inline-flex items-center gap-2 text-xs text-foreground">
                <input
                  type="checkbox"
                  checked={playlistSubmissionConfig.enabled}
                  onChange={(event) => onPlaylistConfigChange({
                    ...playlistSubmissionConfig,
                    enabled: event.target.checked,
                  })}
                  className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
                />
                {t('Enable')}
              </label>
            </div>
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <label className="space-y-1 text-xs text-muted-foreground">
                <span className="block">{t('Start index')}</span>
                <input
                  type="number"
                  min={1}
                  value={playlistSubmissionConfig.start_index}
                  disabled={!playlistSubmissionConfig.enabled}
                  onChange={(event) => onPlaylistConfigChange({
                    ...playlistSubmissionConfig,
                    start_index: Number(event.target.value) || 1,
                  })}
                  className="h-9 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground shadow-sm disabled:cursor-not-allowed disabled:opacity-50"
                />
              </label>
              <label className="space-y-1 text-xs text-muted-foreground">
                <span className="block">{t('Max items')}</span>
                <input
                  type="number"
                  min={1}
                  max={50}
                  value={playlistSubmissionConfig.max_items}
                  disabled={!playlistSubmissionConfig.enabled}
                  onChange={(event) => onPlaylistConfigChange({
                    ...playlistSubmissionConfig,
                    max_items: Number(event.target.value) || 1,
                  })}
                  className="h-9 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground shadow-sm disabled:cursor-not-allowed disabled:opacity-50"
                />
              </label>
            </div>
            <p className="mt-3 text-[11px] leading-relaxed text-muted-foreground/80">
              {t('The backend enforces a hard cap of 50 items to avoid creating too many tasks at once.')}
            </p>
          </div>
        </div>

        <div className="flex flex-col gap-4">
          <div className="rounded-xl border border-border/70 bg-muted/20 p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-foreground">{t('Task chain')}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">{t('Disable optional steps when needed and keep the minimal pipeline.')}</p>
              </div>
              <span className="inline-flex min-w-10 justify-center rounded-full bg-muted px-2 py-0.5 text-[11px] tabular-nums text-muted-foreground">
                {enabledTaskCount} / {TASK_CHAIN_ITEMS.length}
              </span>
            </div>
            <div className="mt-3 divide-y divide-border/60">
              {TASK_CHAIN_ITEMS.map((item) => {
                const disabled =
                  (item.key === 'translate_subtitles' && !taskChainSettings.transcribe) ||
                  (item.key === 'synthesize_subtitle_audio' && !taskChainSettings.transcribe);

                return (
                  <label
                    key={item.key}
                    className={`flex items-start gap-3 py-2.5 ${disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}`}
                  >
                    <input
                      type="checkbox"
                      checked={taskChainSettings[item.key]}
                      disabled={disabled}
                      onChange={(event) => {
                        onTaskChainChange(normalizeTaskChainSettings({
                          ...taskChainSettings,
                          [item.key]: event.target.checked,
                        }));
                      }}
                      className="mt-0.5 h-4 w-4 rounded border-border text-primary focus:ring-primary"
                    />
                    <span className="min-w-0">
                      <span className="block text-sm font-medium leading-tight text-foreground">
                        {t(item.labelKey)}
                        <span
                          className={`ml-1.5 inline-block text-[10px] font-normal text-muted-foreground transition-opacity ${disabled ? 'opacity-100' : 'opacity-0'}`}
                          aria-hidden={!disabled}
                        >
                          {t('(dependency not met)')}
                        </span>
                      </span>
                      <span className="mt-0.5 block text-[11px] leading-relaxed text-muted-foreground">{t(item.descriptionKey)}</span>
                    </span>
                  </label>
                );
              })}
            </div>
          </div>

        </div>
      </div>
      <div className="border-t border-border px-4 py-3 text-[11px] leading-relaxed text-muted-foreground/70">
        {t('Voice settings are managed on the settings page. This page only controls resolution, playlist batching, and task-chain toggles.')}
      </div>
    </div>
  );
});

const SubmitVideoCard = memo(function SubmitVideoCard({
  aiAvailable,
  onSubmitted,
}: {
  aiAvailable: boolean;
  onSubmitted: () => Promise<void>;
}) {
  const { t } = useI18n();
  const { user, currentUser } = useAuth();
  const { tier, credits } = useMembership();
  const userId = currentUser?.id ?? user?.uid ?? '';
  const { settings, loaded: settingsLoaded, updateSettings } = useUserSettings(userId);
  const [url, setUrl] = useState('');
  const [showSubmitSettings, setShowSubmitSettings] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [lastSubmissionResult, setLastSubmissionResult] = useState<VideoQueueSubmissionResult | null>(null);
  const [perVideoRequiredCredits, setPerVideoRequiredCredits] = useState(DEFAULT_FULL_PIPELINE_CREDITS_PER_VIDEO);
  const [completionStatus, setCompletionStatus] = useState<'idle' | 'tracking' | 'completed' | 'failed'>('idle');
  const [showCompletionUpsell, setShowCompletionUpsell] = useState(true);
  const [preferredResolution, setPreferredResolution] = useState<PreferredResolution>('best');
  const [playlistSubmissionConfig, setPlaylistSubmissionConfig] = useState<PlaylistSubmissionConfig>(DEFAULT_PLAYLIST_SUBMISSION_CONFIG);
  const [taskChainSettings, setTaskChainSettings] = useState<TaskChainSettings>(DEFAULT_TASK_CHAIN_SETTINGS);
  const syncedUserSettingsRef = useRef(false);
  const speechSynthesisConfig = useMemo(
    () => parseSpeechSynthesisConfig(getStoredSubtitleAudioTTSConfigValue(settings)) ?? DEFAULT_SPEECH_SYNTHESIS_CONFIG,
    [settings],
  );

  const submissionInspection = inspectVideoSubmissionUrl(url, playlistSubmissionConfig);
  const submissionPlatformBadge = submissionInspection.isSupported
    ? (submissionInspection.platform === 'douyin'
      ? t('Douyin pipeline')
      : submissionInspection.targetKind === 'playlist'
        ? t('YouTube playlist')
        : t('YouTube pipeline'))
    : null;

  const shouldEstimatePlaylistMaxItems = submissionInspection.platform === 'youtube'
    && submissionInspection.targetKind === 'playlist'
    && playlistSubmissionConfig.enabled;
  const estimatedUnits = shouldEstimatePlaylistMaxItems
    ? Math.max(1, Math.floor(playlistSubmissionConfig.max_items))
    : 1;
  const estimatedTotalRequiredCredits = perVideoRequiredCredits * estimatedUnits;
  const shortageCredits = Math.max(estimatedTotalRequiredCredits - credits.balance, 0);

  const recommendedPlanId = tier === 'free'
    ? (shortageCredits > 20 ? 'vip_month' : 'vip_7d')
    : 'vip_month';

  useEffect(() => {
    const storedResolution = readDashboardSubmitResolution();
    if (storedResolution) {
      setPreferredResolution(storedResolution);
    }

    const storedTaskChainSettings = readDashboardTaskChainSettings();
    if (storedTaskChainSettings) {
      setTaskChainSettings(storedTaskChainSettings);
    }

    const storedPlaylistConfig = readDashboardPlaylistSubmissionConfig();
    if (storedPlaylistConfig) {
      setPlaylistSubmissionConfig(storedPlaylistConfig);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const estimated = await estimateCredits('full_pipeline', 1);
      if (cancelled) return;
      if (estimated > 0) {
        setPerVideoRequiredCredits(estimated);
        return;
      }
      setPerVideoRequiredCredits(DEFAULT_FULL_PIPELINE_CREDITS_PER_VIDEO);
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    syncedUserSettingsRef.current = false;
  }, [userId]);

  useEffect(() => {
    setCompletionStatus('idle');
    setShowCompletionUpsell(true);
  }, [lastSubmissionResult?.videoId]);

  useEffect(() => {
    if (!userId || !settingsLoaded || syncedUserSettingsRef.current) {
      return;
    }

    syncedUserSettingsRef.current = true;

    const remoteResolutionRaw = settings[USER_SETTING_KEY_PREFERRED_RESOLUTION];
    const remoteResolution = RESOLUTION_OPTIONS.some(
      (option) => option.value === remoteResolutionRaw as PreferredResolution,
    )
      ? remoteResolutionRaw as PreferredResolution
      : null;
    const remoteTaskChain = readSerializedTaskChainSettings(settings[USER_SETTING_KEY_TASK_CHAIN_SETTINGS]);
    const remotePlaylistConfig = readSerializedPlaylistSubmissionConfig(settings[USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG]);

    if (remoteResolution) {
      setPreferredResolution(remoteResolution);
      writeDashboardSubmitResolution(remoteResolution);
    }

    if (remoteTaskChain) {
      setTaskChainSettings(remoteTaskChain);
      writeDashboardTaskChainSettings(remoteTaskChain);
    }

    if (remotePlaylistConfig) {
      setPlaylistSubmissionConfig(remotePlaylistConfig);
      writeDashboardPlaylistSubmissionConfig(remotePlaylistConfig);
    }

    const patch: Record<string, string> = {};
    if (!remoteResolution) {
      patch[USER_SETTING_KEY_PREFERRED_RESOLUTION] = preferredResolution;
    }
    if (!remoteTaskChain) {
      patch[USER_SETTING_KEY_TASK_CHAIN_SETTINGS] = serializeTaskChainSettings(taskChainSettings);
    }
    if (!remotePlaylistConfig) {
      patch[USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG] = serializePlaylistSubmissionConfig(playlistSubmissionConfig);
    }

    if (Object.keys(patch).length > 0) {
      void updateSettings(patch).catch(() => {
        // Keep local settings usable even if remote persistence fails.
      });
    }
  }, [playlistSubmissionConfig, preferredResolution, settings, settingsLoaded, taskChainSettings, updateSettings, userId]);

  const enabledTaskCount = enabledTaskChainCount(taskChainSettings);
  const hasCustomSubmitSettings = preferredResolution !== 'best'
    || enabledTaskCount < TASK_CHAIN_ITEMS.length
    || !playlistSubmissionConfigEqual(playlistSubmissionConfig, DEFAULT_PLAYLIST_SUBMISSION_CONFIG);

  const handleResolutionChange = useCallback((value: PreferredResolution) => {
    setPreferredResolution((currentValue) => {
      if (currentValue === value) {
        return currentValue;
      }

      writeDashboardSubmitResolution(value);
      if (userId) {
        void updateSettings({
          [USER_SETTING_KEY_PREFERRED_RESOLUTION]: value,
        }).catch(() => {
          // Keep local settings usable even if remote persistence fails.
        });
      }
      return value;
    });
  }, [updateSettings, userId]);

  const handleTaskChainChange = useCallback((value: TaskChainSettings) => {
    const normalizedValue = normalizeTaskChainSettings(value);
    setTaskChainSettings((currentValue) => {
      if (taskChainSettingsEqual(currentValue, normalizedValue)) {
        return currentValue;
      }

      writeDashboardTaskChainSettings(normalizedValue);
      if (userId) {
        void updateSettings({
          [USER_SETTING_KEY_TASK_CHAIN_SETTINGS]: serializeTaskChainSettings(normalizedValue),
        }).catch(() => {
          // Keep local settings usable even if remote persistence fails.
        });
      }
      return normalizedValue;
    });
  }, [updateSettings, userId]);

  const handlePlaylistConfigChange = useCallback((value: PlaylistSubmissionConfig) => {
    const normalizedValue = normalizePlaylistSubmissionConfig(value);
    setPlaylistSubmissionConfig((currentValue) => {
      if (playlistSubmissionConfigEqual(currentValue, normalizedValue)) {
        return currentValue;
      }

      writeDashboardPlaylistSubmissionConfig(normalizedValue);
      if (userId) {
        void updateSettings({
          [USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG]: serializePlaylistSubmissionConfig(normalizedValue),
        }).catch(() => {
          // Keep local settings usable even if remote persistence fails.
        });
      }
      return normalizedValue;
    });
  }, [updateSettings, userId]);

  const handleResetSubmitSettings = useCallback(() => {
    setPreferredResolution('best');
    setPlaylistSubmissionConfig(DEFAULT_PLAYLIST_SUBMISSION_CONFIG);
    setTaskChainSettings(DEFAULT_TASK_CHAIN_SETTINGS);
    writeDashboardSubmitResolution('best');
    writeDashboardPlaylistSubmissionConfig(DEFAULT_PLAYLIST_SUBMISSION_CONFIG);
    writeDashboardTaskChainSettings(DEFAULT_TASK_CHAIN_SETTINGS);
    if (userId) {
      void updateSettings({
        [USER_SETTING_KEY_PREFERRED_RESOLUTION]: 'best',
        [USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG]: serializePlaylistSubmissionConfig(DEFAULT_PLAYLIST_SUBMISSION_CONFIG),
        [USER_SETTING_KEY_TASK_CHAIN_SETTINGS]: serializeTaskChainSettings(DEFAULT_TASK_CHAIN_SETTINGS),
      }).catch(() => {
        // Keep local settings usable even if remote persistence fails.
      });
    }
  }, [updateSettings, userId]);

  const handleSubmit = useCallback(async () => {
    const trimmed = url.trim();
    if (!trimmed || submitting || !submissionInspection.isSupported) return;

    setSubmitting(true);
    try {
      const result = await submitVideoToQueue(
        trimmed,
        undefined,
        preferredResolution,
        taskChainSettings,
        normalizeSpeechSynthesisConfig(speechSynthesisConfig),
        playlistSubmissionConfig,
      );
      setLastSubmissionResult(result);
      setUrl('');
      toast.success(
        result.submissionMode === 'playlist'
          ? t('Playlist queued successfully ({count} items).', { count: result.submittedCount })
          : t('{platform} task queued successfully. ID: {id}', {
            platform: result.platform === 'douyin' ? 'Douyin' : 'YouTube',
            id: result.videoId,
          }),
        { duration: 5000 },
      );
      await onSubmitted();
    } catch (error) {
      toast.error(getVideoSubmissionErrorMessage(error));
    } finally {
      setSubmitting(false);
    }
  }, [currentUser, onSubmitted, perVideoRequiredCredits, playlistSubmissionConfig, preferredResolution, speechSynthesisConfig, submissionInspection.isSupported, submitting, t, taskChainSettings, url, user]);

  return (
    <div className="rounded-2xl border border-border bg-card p-6 shadow-sm">
      <h1 className="text-xl font-bold mb-1">{t('Submit new video')}</h1>
      <p className="text-sm text-muted-foreground mb-4">
  		{t('Paste a YouTube video, YouTube playlist, or Douyin share link. It will first enter the queue, then continue with download, audio extraction, subtitle transcription, translation, and subtitle voiceover based on your settings.')}
      </p>
      <div className="flex flex-col gap-2 md:flex-row">
        <input
          type="url"
          value={url}
          onChange={e => setUrl(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleSubmit()}
          placeholder="https://www.youtube.com/watch?v=... / playlist?list=... / https://v.douyin.com/..."
          className="flex-1 h-10 px-4 rounded-lg border border-border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        />
        <button
          type="button"
          onClick={() => setShowSubmitSettings((value) => !value)}
          className={`h-10 px-3.5 rounded-lg border text-sm font-medium flex items-center gap-2 transition-colors shrink-0 ${showSubmitSettings ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background text-foreground hover:bg-accent'}`}
        >
          <Settings className="h-4 w-4 shrink-0" />
          {t('Settings')}
          <span
            className={`h-1.5 w-1.5 rounded-full bg-primary shrink-0 transition-opacity ${hasCustomSubmitSettings ? 'opacity-100' : 'opacity-0'}`}
            aria-hidden={!hasCustomSubmitSettings}
          />
          <ChevronDown className={`h-3.5 w-3.5 opacity-60 transition-transform duration-200 shrink-0 ${showSubmitSettings ? 'rotate-180' : ''}`} />
        </button>
        <button
          onClick={handleSubmit}
          disabled={!url.trim() || submitting || !submissionInspection.isSupported}
          className="h-10 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium flex items-center gap-2 hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shrink-0"
        >
          {submitting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
          {t('Submit')}
        </button>
      </div>
      {!showSubmitSettings && (
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          {submissionPlatformBadge && (
            <span className="inline-flex items-center rounded-md border border-primary/30 bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
              {submissionPlatformBadge}
            </span>
          )}
          <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/50 px-2 py-0.5 text-[11px] text-muted-foreground">
            <Settings className="h-3 w-3 opacity-60" />
            {t(RESOLUTION_OPTIONS.find((option) => option.value === preferredResolution)?.labelKey ?? preferredResolution)}
          </span>
          {playlistSubmissionConfig.enabled && (
            <span className="inline-flex items-center rounded-md border border-border bg-muted/50 px-2 py-0.5 text-[11px] text-muted-foreground">
              {t('Playlist: start from {start} / max {count} items', {
                start: playlistSubmissionConfig.start_index,
                count: playlistSubmissionConfig.max_items,
              })}
            </span>
          )}
          {TASK_CHAIN_ITEMS.map((item) => (
            <span
              key={item.key}
              className={`inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] transition-colors ${
                taskChainSettings[item.key]
                  ? 'border-border bg-muted/50 text-muted-foreground'
                  : 'border-dashed border-border/60 bg-transparent text-muted-foreground/50 line-through'
              }`}
            >
              {t(item.labelKey)}
            </span>
          ))}
        </div>
      )}



      {showSubmitSettings && (
        <SubmitSettingsPanel
          preferredResolution={preferredResolution}
          playlistSubmissionConfig={playlistSubmissionConfig}
          taskChainSettings={taskChainSettings}
          enabledTaskCount={enabledTaskCount}
          onResolutionChange={handleResolutionChange}
          onPlaylistConfigChange={handlePlaylistConfigChange}
          onTaskChainChange={handleTaskChainChange}
          onReset={handleResetSubmitSettings}
        />
      )}
      <p className={`mt-3 text-xs ${url && !submissionInspection.isSupported ? 'text-amber-600' : 'text-muted-foreground'}`}>
        {url
          ? (submissionInspection.isSupported ? submissionInspection.description : submissionInspection.reason)
          : t('This page supports YouTube videos, YouTube playlists, and Douyin share links. After submission the backend continues with {summary}. You can watch progress in the task queue.', {
            summary: taskChainExecutionSummary(taskChainSettings),
          })}
      </p>

      {lastSubmissionResult && (
        <div className="mt-4 rounded-xl border border-green-200 bg-green-50 p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-semibold text-green-800">
                {lastSubmissionResult.submissionMode === 'playlist' ? t('Playlist queued in batch') : t('Queued for processing')}
              </p>
              <p className="mt-1 text-xs text-green-700">
                {t('First task ID: {id}', { id: lastSubmissionResult.videoId })}
              </p>
              {lastSubmissionResult.submissionMode === 'playlist' && (
                <p className="mt-1 text-xs text-green-700">
                  {t('Batch submission count: {count}', { count: lastSubmissionResult.submittedCount })}
                  {lastSubmissionResult.playlistId ? ` · ${t('Playlist ID: {id}', { id: lastSubmissionResult.playlistId })}` : ''}
                </p>
              )}
              <p className="mt-1 text-xs text-green-700">{t('Resolution preference: {value}', { value: preferredResolution })}</p>
              <p className="mt-1 text-xs text-green-700">{t('Task chain: {summary}', { summary: taskChainSummary(taskChainSettings) })}</p>
              <p className="mt-2 text-xs text-green-700/90 leading-5">
				{t('The backend continues with {summary}. You do not need to keep this page open.', {
				  summary: taskChainExecutionSummary(taskChainSettings),
				})}
              </p>
            </div>
            <CheckCircle2 className="w-5 h-5 text-green-600 shrink-0 mt-0.5" />
          </div>
          {lastSubmissionResult.submissionMode === 'single' && (
            <div className="mt-4 rounded-lg border border-green-100 bg-white/80 p-3">
              <p className="mb-2 text-xs font-medium text-green-800">{t('Live processing progress')}</p>

              <InlineTaskTracker
                videoId={lastSubmissionResult.videoId}
                onStatusChange={(payload) => {
                  setCompletionStatus(payload.status);
                }}
              />
            </div>
          )}

          {lastSubmissionResult.submissionMode === 'single' && completionStatus === 'completed' && showCompletionUpsell ? (
            <div className="mt-4">
              <UpgradePromptCard
                badge={t('Next step')}
                title={tier === 'free' ? t('This video completed successfully. Upgrade before the next one.') : t('Top up your credits before the next batch.')}
                description={
                  tier === 'free'
                    ? t('Membership unlocks AI translation, voiceover, metadata, and batch capabilities together, so the next run is less likely to be interrupted by limits.')
                    : t('You currently have {balance} credits. A full pipeline run can use about {perVideo} credits per video, so topping up early is safer.', {
                      balance: credits.balance,
                      perVideo: perVideoRequiredCredits,
                    })
                }
                highlights={tier === 'free'
                  ? [t('More reliable subtitle translation and voiceover'), t('Batch tasks with less friction'), t('Higher credit limits')]
                  : [t('{balance} credits available', { balance: credits.balance }), t('{perVideo} credits per full run', { perVideo: perVideoRequiredCredits }), t('Credits arrive immediately after payment')]}
                primaryHref={`/membership?source=dashboard-success&recommended=${recommendedPlanId}`}
                primaryLabel={tier === 'free' ? t('Upgrade and continue') : t('Top up credits')}
                secondaryHref="/membership"
                secondaryLabel={t('View all plans')}
                dismissible
                onDismiss={() => setShowCompletionUpsell(false)}
              />
            </div>
          ) : null}
          <div className="mt-3 flex flex-wrap items-center gap-4 text-xs">
            <NextLink href="/dashboard/tasks" className="text-green-700 hover:underline font-medium">
              {t('View task queue')} →
            </NextLink>
            <NextLink href="/dashboard/videos" className="text-green-700 hover:underline font-medium">
              {t('View video library')} →
            </NextLink>
          </div>
        </div>
      )}
      {aiAvailable && (
        <p className="mt-3 text-xs text-muted-foreground">
          {t('You can also use the ')}
          <NextLink href="/dashboard/assistant" className="text-primary hover:underline font-medium">
            {t('AI Assistant')}
          </NextLink>
          {t(' to submit and manage tasks in natural language (type ')}<code className="bg-muted px-1 rounded">/</code>{t(' to quickly pick a command)')}
        </p>
      )}

    </div>
  );
});


// ── page ──────────────────────────────────────────────────────────────────────

export default function DashboardPage() {
  const { t } = useI18n();
  const [recent, setRecent]         = useState<Video[]>([]);
  const [counts, setCounts]         = useState<VideoTabCounts | null>(null);
  const [statsLoading, setStatsLoading] = useState(true);
  const [agentInfo, setAgentInfo]   = useState<AgentInfo | null>(null);
  const [systemUsage, setSystemUsage] = useState<SystemUsageResponse | null>(null);

  const refreshDashboardSummary = useCallback(async () => {
    const [cr, vr, ar, sr] = await Promise.allSettled([
      api.getVideoCounts(),
      api.getVideos({ size: 6 }),
      agentApi.getInfo(),
      api.getSystemUsage(),
    ]);

    if (cr.status === 'fulfilled') setCounts(cr.value);
    if (vr.status === 'fulfilled') setRecent(vr.value.videos ?? []);
    if (ar.status === 'fulfilled') setAgentInfo(ar.value);
    if (sr.status === 'fulfilled') setSystemUsage(sr.value);
    setStatsLoading(false);
  }, []);

  // ── load on mount ──────────────────────────────────────────────────────────
  useEffect(() => {
    void refreshDashboardSummary();
  }, [refreshDashboardSummary]);

  // ── derived stats from backend counts ─────────────────────────────────────
  const total      = counts?.all        ?? 0;
  const completed  = counts?.completed  ?? 0;
  const processing = counts?.processing ?? 0;
  const uploaded   = counts?.bili_uploaded ?? 0;
  const diskRemaining = systemUsage ? formatBytes(systemUsage.disk.free_bytes) : '0 B';
  const diskSubtitle = systemUsage
    ? `${systemUsage.disk.used_percent.toFixed(1)}% / ${formatBytes(systemUsage.disk.total_bytes)}`
    : undefined;
  const availableMemory = systemUsage ? formatBytes(systemUsage.memory.free_bytes) : '0 B';
  const memorySubtitle = systemUsage
    ? `${systemUsage.memory.used_percent.toFixed(1)}% / ${formatBytes(systemUsage.memory.total_bytes)}`
    : undefined;
  const cpuValue = systemUsage ? `${systemUsage.cpu_percent.toFixed(1)}%` : '0%';
  const cpuSubtitle = systemUsage ? t('Current host CPU usage') : undefined;

  const aiAvailable = agentInfo?.available === true;

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-5xl mx-auto px-4 py-8 space-y-8">
        <section className="rounded-2xl border border-border bg-card/80 p-5 shadow-sm">
          <div className="flex flex-col gap-1 pb-4 md:flex-row md:items-end md:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted-foreground/70">{t('System Overview')}</p>
              <h2 className="mt-1 text-lg font-semibold text-foreground">{t('System information')}</h2>
              <p className="mt-1 text-sm text-muted-foreground">{t('Only the three most useful resource indicators are shown here: disk remaining, CPU usage, and available memory.')}</p>
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <StatCard loading={statsLoading} accent="text-amber-600"  label={t('Disk remaining')}  value={diskRemaining}   subtitle={diskSubtitle} icon={<HardDrive className="w-5 h-5" />} />
            <StatCard loading={statsLoading} accent="text-emerald-600" label={t('CPU usage')} value={cpuValue}       subtitle={cpuSubtitle} icon={<Loader2 className="w-5 h-5" />} />
            <StatCard loading={statsLoading} accent="text-cyan-600"   label={t('Available memory')}  value={availableMemory} subtitle={memorySubtitle} icon={<MemoryStick className="w-5 h-5" />} />
          </div>
        </section>

        <section className="rounded-2xl border border-border bg-card/60 p-5 shadow-sm">
          <div className="flex flex-col gap-1 pb-4 md:flex-row md:items-end md:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted-foreground/70">{t('Task Overview')}</p>
              <h2 className="mt-1 text-lg font-semibold text-foreground">{t('Task overview')}</h2>
              <p className="mt-1 text-sm text-muted-foreground">{t('Shows the current video processing queue and Bilibili upload progress.')}</p>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <StatCard loading={statsLoading} accent="text-indigo-600" label={t('Total videos')}  value={total}      icon={<VideoIcon    className="w-5 h-5" />} />
            <StatCard loading={statsLoading} accent="text-green-600"  label={t('Completed')}    value={completed}  icon={<CheckCircle2 className="w-5 h-5" />} />
            <StatCard loading={statsLoading} accent="text-blue-500"   label={t('Processing')}    value={processing} icon={<Loader2      className="w-5 h-5" />} />
            <StatCard loading={statsLoading} accent="text-pink-600"   label={t('Uploaded to Bilibili')} value={uploaded}   icon={<TrendingUp   className="w-5 h-5" />} />
          </div>
        </section>

        <SubmitVideoCard aiAvailable={aiAvailable} onSubmitted={refreshDashboardSummary} />

        {/* ── Recent + Quick actions ────────────────────────────────────── */}
        <div className="grid md:grid-cols-3 gap-6 items-start">

          {/* Recent videos (2/3 width) */}
          <div className="md:col-span-2 rounded-xl border border-border bg-card overflow-hidden">
            <div className="flex items-center justify-between px-5 py-3.5 border-b border-border">
              <h2 className="font-semibold text-sm">{t('Recent activity')}</h2>
              <NextLink href="/dashboard/videos" className="text-xs text-primary hover:underline flex items-center gap-1">
                {t('View all')} <ArrowRight className="w-3 h-3" />
              </NextLink>
            </div>
            {statsLoading ? (
              <div className="divide-y divide-border">
                {Array.from({ length: 4 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-3 px-5 py-3">
                    <Skeleton className="w-14 h-9 shrink-0" />
                    <div className="flex-1 space-y-1.5">
                      <Skeleton className="h-3 w-3/4" />
                      <Skeleton className="h-2.5 w-1/3" />
                    </div>
                    <Skeleton className="h-5 w-14 rounded-full" />
                  </div>
                ))}
              </div>
            ) : recent.length === 0 ? (
              <p className="py-12 text-center text-sm text-muted-foreground">
                {t('No videos yet. Submit your first link to start.')} 🎬
              </p>
            ) : (
              <div className="divide-y divide-border">
                {recent.map(v => {
                  const meta = statusMeta(v.status);
                  return (
                    <div key={v.id} className="flex items-center gap-3 px-5 py-3 hover:bg-muted/40 transition-colors">
                      {v.thumbnail ? (
                        <img src={v.thumbnail} alt="" className="w-14 h-9 rounded object-cover shrink-0 bg-muted" />
                      ) : (
                        <div className="w-14 h-9 rounded bg-muted shrink-0 flex items-center justify-center">
                          <VideoIcon className="w-4 h-4 text-muted-foreground/50" />
                        </div>
                      )}
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate leading-tight">
                          {v.generated_title || v.title || v.url}
                        </p>
                        <p className="text-xs text-muted-foreground mt-0.5 flex items-center gap-1.5">
                          {v.platform && <span className="capitalize">{v.platform}</span>}
                          {v.duration > 0 && <><span>·</span><span>{fmtDuration(v.duration)}</span></>}
                          {v.bili_bvid && <><span>·</span><span className="text-pink-500">{t('Uploaded to Bilibili')}</span></>}
                        </p>
                      </div>
                      <span className={`text-[11px] font-medium px-2 py-0.5 rounded-full border shrink-0 ${meta.color}`}>
                        {meta.label}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Quick actions (1/3 width) */}
          <div className="space-y-3">
            <h2 className="font-semibold text-sm px-1">{t('Quick actions')}</h2>
            <ActionCard
              href="/dashboard/assistant"
              icon={<Sparkles className="w-5 h-5 text-primary" />}
              title={t('AI Assistant')}
              desc={t('Manage tasks, subtitle translation, and metadata rewriting conversationally')}
              badge={aiAvailable ? t('Online') : t('Not configured')}
              badgeColor={
                aiAvailable
                  ? 'text-green-600 bg-green-50 border-green-200'
                  : 'text-muted-foreground bg-muted border-border'
              }
            />
            <ActionCard
              href="/dashboard/videos"
              icon={<VideoIcon className="w-5 h-5 text-indigo-500" />}
              title={t('Video library')}
              desc={t('Browse, play, and manage all local videos')}
            />
            <ActionCard
              href="/dashboard/subscriptions"
              icon={<Rss className="w-5 h-5 text-orange-500" />}
              title={t('Subscriptions')}
              desc={t('Manage automatic syncing for YouTube channels')}
            />
            <ActionCard
              href="/dashboard/settings"
              icon={<Settings className="w-5 h-5 text-muted-foreground" />}
              title={t('Settings')}
              desc={t('Account bindings, cookies, and system configuration')}
            />
          </div>
        </div>

        {/* ── Supported platforms ───────────────────────────────────────── */}
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold">{t('Supported platforms')}</h2>
            <span className="text-xs text-muted-foreground">{t('· Powered by yt-dlp, covering 1000+ sites')}</span>
          </div>
          <div className="flex flex-wrap gap-3">
            {[
              { icon: <FaYoutube  className="w-5 h-5 text-red-600" />,      name: 'YouTube'   },
              { icon: <FaTiktok   className="w-5 h-5 text-foreground" />,   name: 'Douyin'    },
              { icon: <FaBilibili className="w-5 h-5 text-sky-500" />,      name: 'Bilibili'  },
              { icon: <FaXTwitter className="w-5 h-5 text-foreground" />,   name: 'Twitter/X' },
              { icon: <FaTiktok   className="w-5 h-5 text-foreground" />,   name: 'TikTok'    },
            ].map(p => (
              <div key={p.name} className="flex items-center gap-2 px-3 py-2 rounded-lg border border-border bg-background">
                {p.icon}
                <span className="text-sm font-medium">{p.name}</span>
              </div>
            ))}
            <div className="flex items-center gap-2 px-3 py-2 rounded-lg border border-dashed border-border text-muted-foreground text-sm">
              {t('+ More platforms')}
            </div>
          </div>
        </div>

      </div>
    </div>
  );
}
