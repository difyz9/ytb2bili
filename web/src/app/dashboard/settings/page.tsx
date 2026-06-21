"use client";

import * as React from 'react';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useAiModelCatalog } from '@/hooks/useAiModelCatalog';
import { useMembership } from '@/hooks/useMembership';
import { useUserSettings } from '@/hooks/useUserSettings';
import { apiKeysApi, type UserApiKeyRecord } from '@/lib/api/api-keys';
import { systemSettingsApi } from '@/lib/api/system-settings';
import { userSettingsApi } from '@/lib/api/user-settings';
import { normalizeTier } from '@/lib/agent-models';
import { type TTSVoiceRecord } from '@/lib/tts-voice-catalog';
import {
  BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS,
  type BilibiliVideoZone,
  DEFAULT_SPEECH_SYNTHESIS_CONFIG,
  getSpeechSynthesisVoiceLabel,
  getStoredSubtitleAudioTTSConfigValue,
  normalizeSpeechSynthesisConfig,
  parseSpeechSynthesisConfig,
  serializeSpeechSynthesisConfig,
  SPEECH_SYNTHESIS_FORMAT_OPTIONS,
  SPEECH_SYNTHESIS_PROVIDER_OPTIONS,
  type SpeechSynthesisConfig,
  USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT,
  USER_SETTING_KEY_SUBTITLE_AUDIO_TTS_CONFIG,
  USER_SETTING_KEY_BILIBILI_SUBMISSION_TID,
  USER_SETTING_KEY_WATERMARK_PROMO_ENABLED,
} from '@/lib/video-submission';
import { Input } from '@/components/ui/Input';
import { Button } from '@/components/ui/Button';
import { VoicePicker } from '@/components/tts/VoicePicker';
import CookiesManager from '@/components/CookiesManager';
import UpdateManager from '@/components/UpdateManager';
import {
  LogOut,
  KeyRound,
  ChevronDown,
  Check,
  Crown,
  User,
  Languages,
  AudioLines,
  SlidersHorizontal,
  UploadCloud,
  LayoutTemplate,
} from 'lucide-react';
import toast from 'react-hot-toast';
import * as SelectPrimitive from '@radix-ui/react-select';
import { resolveClientLocale, translateClientText } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { useI18n } from '@/contexts/I18nContext';

type SettingsSectionId = 'profile' | 'translation' | 'tts' | 'apiKeys' | 'system' | 'publishing' | 'template';

const SettingsCard = ({ children, className }: { children: React.ReactNode; className?: string }) => (
  <div className={cn('overflow-hidden rounded-[24px] border border-slate-200 bg-white shadow-[0_8px_24px_rgba(15,23,42,0.045)]', className)}>{children}</div>
);

const SettingsCardContent = ({ children }: { children: React.ReactNode }) => (
  <div className="space-y-2.5 px-4 py-3">{children}</div>
);

const SettingsRow = ({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) => (
  <div className="flex flex-col gap-2 rounded-2xl border border-slate-200 bg-white px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between">
    <div className="min-w-0 flex-1 pr-0 sm:pr-8">
      <h3 className="text-[15px] font-semibold text-slate-900">{title}</h3>
      <p className="mt-0.5 text-[13px] leading-5 text-slate-500">{description}</p>
    </div>
    <div className="w-full sm:w-64">{children}</div>
  </div>
);

const AccordionItem = ({
  icon: Icon,
  label,
  active,
  onClick,
}: {
  icon: React.ElementType;
  label: string;
  active: boolean;
  onClick: () => void;
}) => (
  <button
    type="button"
    onClick={onClick}
    className={cn(
      'flex w-full items-center justify-between rounded-xl px-4 py-2.5 text-left transition',
      active
        ? 'bg-blue-50 text-blue-600 shadow-[inset_0_0_0_1px_rgba(59,130,246,0.08)]'
        : 'text-slate-700 hover:bg-slate-50 hover:text-slate-900'
    )}
  >
    <span className="flex items-center gap-3 text-[16px] font-semibold leading-6">
      <Icon className="h-[18px] w-[18px]" />
      {label}
    </span>
    <ChevronDown className={cn('h-4 w-4 transition', active ? 'rotate-180 opacity-100' : 'opacity-60')} />
  </button>
);

const SectionHint = ({ text }: { text: string }) => (
  <div className="px-1 pb-1 text-[13px] leading-5 text-slate-500">{text}</div>
);

const SurfaceCard = ({
  title,
  description,
  children,
  tone = 'default',
}: {
  title: string;
  description: string;
  children: React.ReactNode;
  tone?: 'default' | 'accent';
}) => (
  <div
    className={cn(
      'rounded-3xl border px-4 py-4 sm:px-5',
      tone === 'accent'
        ? 'border-blue-200 bg-[linear-gradient(180deg,rgba(239,246,255,0.95),rgba(255,255,255,1))]'
        : 'border-slate-200 bg-[linear-gradient(180deg,rgba(255,255,255,1),rgba(248,250,252,0.92))]'
    )}
  >
    <div className="mb-4 flex items-start justify-between gap-3">
      <div>
        <h3 className="text-[15px] font-semibold text-slate-950">{title}</h3>
        <p className="mt-1 text-[13px] leading-5 text-slate-500">{description}</p>
      </div>
    </div>
    {children}
  </div>
);

const sliderGradient = (value: number, min: number, max: number) => {
  const safeRange = max - min;
  const progress = safeRange <= 0 ? 0 : ((value - min) / safeRange) * 100;
  return `linear-gradient(90deg, rgb(37 99 235) 0%, rgb(37 99 235) ${progress}%, rgb(226 232 240) ${progress}%, rgb(226 232 240) 100%)`;
};

const SpeechSlider = ({
  title,
  description,
  value,
  min,
  max,
  step,
  defaultValue,
  onChange,
  formatValue,
}: {
  title: string;
  description: string;
  value: number;
  min: number;
  max: number;
  step: number;
  defaultValue: number;
  onChange: (value: number) => void;
  formatValue: (value: number) => string;
}) => {
  const { t } = useI18n();
  const displayValue = formatValue(value);
  const defaultLabel = formatValue(defaultValue);

  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm shadow-slate-100/70 transition hover:border-blue-200 hover:shadow-blue-100/70">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-[15px] font-semibold text-slate-950">{title}</h4>
          <p className="mt-1 text-[13px] leading-5 text-slate-500">{description}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span className="inline-flex min-w-[72px] justify-center rounded-full border border-blue-100 bg-blue-50 px-3 py-1 text-sm font-semibold text-blue-700">
            {displayValue}
          </span>
          <button
            type="button"
            onClick={() => onChange(defaultValue)}
            className="rounded-full border border-slate-200 px-3 py-1 text-xs font-medium text-slate-600 transition hover:border-slate-300 hover:bg-slate-50"
          >
            {t('默认 {label}', { label: defaultLabel })}
          </button>
        </div>
      </div>

      <div className="mt-4">
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={(event) => onChange(Number(event.target.value))}
          className="h-2.5 w-full cursor-pointer appearance-none rounded-full"
          style={{ background: sliderGradient(value, min, max) }}
          aria-label={title}
        />
        <div className="mt-2 flex items-center justify-between text-[11px] font-medium text-slate-400">
          <span>{formatValue(min)}</span>
          <span>{t('默认 {label}', { label: defaultLabel })}</span>
          <span>{formatValue(max)}</span>
        </div>
      </div>
    </div>
  );
};

const ZoneChoiceRow = ({
  title,
  subtitle,
  active,
  onClick,
}: {
  title: string;
  subtitle?: string;
  active: boolean;
  onClick: () => void;
}) => {
  const { t } = useI18n();

  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex w-full items-center justify-between gap-3 rounded-xl border-b border-slate-200 px-3 py-3 text-left transition last:border-b-0',
        active ? 'bg-blue-50 text-blue-700' : 'bg-white text-slate-900 hover:bg-slate-50'
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold">{title}</div>
        {subtitle ? <div className="mt-1 text-xs text-slate-500">{subtitle}</div> : null}
      </div>
      <div className="flex items-center gap-3">
        {active ? <span className="text-[11px] font-semibold text-blue-700">{t('已选')}</span> : null}
        <span
          className={cn(
            'h-2.5 w-2.5 rounded-full border',
            active ? 'border-blue-500 bg-blue-500' : 'border-slate-300 bg-white'
          )}
        />
      </div>
    </button>
  );
};

const Switch = ({ checked, onChange, disabled }: { checked: boolean; onChange: (checked: boolean) => void; disabled?: boolean }) => (
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    onClick={() => !disabled && onChange(!checked)}
    className={cn(
      'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
      checked ? 'bg-slate-950' : 'bg-slate-200',
      disabled && 'cursor-not-allowed opacity-50'
    )}
  >
    <span
      aria-hidden="true"
      className={cn(
        'pointer-events-none mt-[1px] inline-block h-5 w-5 transform rounded-full bg-white shadow-[0_1px_2px_rgba(15,23,42,0.2)] ring-0 transition duration-200 ease-in-out',
        checked ? 'translate-x-5' : 'translate-x-0'
      )}
    />
  </button>
);

const Select = SelectPrimitive.Root;
const SelectTrigger = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger>
>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Trigger
    ref={ref}
    className={cn(
      'flex h-9 w-full items-center justify-between rounded-2xl border border-slate-200 bg-slate-50 px-3.5 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50',
      className
    )}
    {...props}
  >
    {children}
    <SelectPrimitive.Icon asChild>
      <ChevronDown className="h-4 w-4 opacity-50" />
    </SelectPrimitive.Icon>
  </SelectPrimitive.Trigger>
));
SelectTrigger.displayName = SelectPrimitive.Trigger.displayName;

const SelectContent = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content>
>(({ className, children, position = 'popper', ...props }, ref) => (
  <SelectPrimitive.Portal>
    <SelectPrimitive.Content
      ref={ref}
      className={cn(
        'relative z-50 min-w-[8rem] overflow-hidden rounded-2xl border border-slate-200 bg-white text-slate-900 shadow-xl shadow-slate-900/10 animate-in fade-in-80',
        position === 'popper' && 'translate-y-1',
        className
      )}
      position={position}
      {...props}
    >
      <SelectPrimitive.Viewport
        className={cn(
          'p-1',
          position === 'popper' &&
            'h-[var(--radix-select-trigger-height)] w-full min-w-[var(--radix-select-trigger-width)]'
        )}
      >
        {children}
      </SelectPrimitive.Viewport>
    </SelectPrimitive.Content>
  </SelectPrimitive.Portal>
));
SelectContent.displayName = SelectPrimitive.Content.displayName;

const SelectItem = React.forwardRef<
  React.ElementRef<typeof SelectPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item>
>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Item
    ref={ref}
    className={cn(
      'relative flex w-full cursor-default select-none items-center rounded-xl py-2.5 pl-8 pr-3 text-sm outline-none focus:bg-slate-100 data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
      className
    )}
    {...props}
  >
    <span className="absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
      <SelectPrimitive.ItemIndicator>
        <Check className="h-4 w-4" />
      </SelectPrimitive.ItemIndicator>
    </span>
    <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
  </SelectPrimitive.Item>
));
SelectItem.displayName = SelectPrimitive.Item.displayName;

const SelectValue = SelectPrimitive.Value;

const AUTO_UPLOAD_INTERVAL_OPTIONS = [15, 30, 60, 90] as const;
const YOUTUBE_FEED_SYNC_INTERVAL_OPTIONS = [15, 30, 60, 120, 180, 360, 720, 1440] as const;
const YOUTUBE_FEED_SYNC_LOOKBACK_OPTIONS = [1, 7, 30, 90] as const;
const AUTO_UPLOAD_STORAGE_KEY = 'ytb2bili:settings:auto-upload';
const AUTO_UPLOAD_INTERVAL_STORAGE_KEY = 'ytb2bili:settings:auto-upload-interval';
const TRANSLATION_SOURCE_LANG_STORAGE_KEY = 'ytb2bili:settings:translation-source';
const TRANSLATION_TARGET_LANG_STORAGE_KEY = 'ytb2bili:settings:translation-target';
const TRANSLATION_MODEL_STORAGE_KEY = 'ytb2bili:settings:translation-model';
const METADATA_MODEL_STORAGE_KEY = 'ytb2bili:settings:metadata-model';
const SUBTITLE_AUDIO_VOICE_STORAGE_KEY = 'ytb2bili:settings:subtitle-audio-voice';
const BILIBILI_SUBMISSION_COPYRIGHT_STORAGE_KEY = 'ytb2bili:settings:bilibili-submission-copyright';
const BID_DEFAULT_LANGUAGE_STORAGE_KEY = 'ytb2bili:settings:bid-language';
const BID_DEFAULT_TONE_STORAGE_KEY = 'ytb2bili:settings:bid-tone';
const BID_TEMPLATE_STYLE_STORAGE_KEY = 'ytb2bili:settings:bid-template-style';
const SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_ENABLED = 'youtube_feed_sync_enabled';
const SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_INTERVAL = 'youtube_feed_sync_interval_minutes';
const SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_LOOKBACK = 'youtube_feed_sync_lookback_days';

const LANGUAGE_OPTIONS = [
  { value: 'auto', label: '自动检测' },
  { value: 'en', label: '英语' },
  { value: 'zh-Hans', label: '简体中文' },
  { value: 'zh-Hant', label: '繁体中文' },
  { value: 'ja', label: '日语' },
  { value: 'ko', label: '韩语' },
  { value: 'es', label: '西班牙语' },
  { value: 'fr', label: '法语' },
  { value: 'de', label: '德语' },
  { value: 'ru', label: '俄语' },
];

const BID_TONE_OPTIONS = [
  { value: 'professional', label: '专业稳健' },
  { value: 'formal', label: '正式严谨' },
  { value: 'concise', label: '简洁清晰' },
  { value: 'persuasive', label: '说服导向' },
] as const;

const BID_TEMPLATE_STYLE_OPTIONS = [
  { value: 'standard', label: '标准模板' },
  { value: 'structured', label: '结构化模板' },
  { value: 'executive', label: '管理汇报风格' },
  { value: 'technical', label: '技术方案风格' },
] as const;

function buildSettingsStorageKey(storageKey: string, userId?: string): string {
  return `${storageKey}:${userId || 'anonymous'}`;
}

function readStoredSetting(storageKey: string, userId?: string): string | null {
  if (typeof window === 'undefined') return null;

  try {
    return localStorage.getItem(buildSettingsStorageKey(storageKey, userId));
  } catch {
    return null;
  }
}

function writeStoredSetting(storageKey: string, userId: string | undefined, value: string) {
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(buildSettingsStorageKey(storageKey, userId), value);
  } catch {
    // Keep in-memory state even if local persistence fails.
  }
}

function isLanguageOption(value: string, allowAuto = true): boolean {
  return LANGUAGE_OPTIONS.some((option) => option.value === value && (allowAuto || option.value !== 'auto'));
}

function isIntervalOption(value: string): boolean {
  return AUTO_UPLOAD_INTERVAL_OPTIONS.some((item) => String(item) === value);
}

function isYouTubeFeedSyncIntervalOption(value: string): boolean {
  return YOUTUBE_FEED_SYNC_INTERVAL_OPTIONS.some((item) => String(item) === value);
}

function isYouTubeFeedSyncLookbackOption(value: string): boolean {
  return YOUTUBE_FEED_SYNC_LOOKBACK_OPTIONS.some((item) => String(item) === value);
}

function isBidToneOption(value: string): boolean {
  return BID_TONE_OPTIONS.some((option) => option.value === value);
}

function isBidTemplateStyleOption(value: string): boolean {
  return BID_TEMPLATE_STYLE_OPTIONS.some((option) => option.value === value);
}

function isBilibiliSubmissionCopyrightOption(value: string): boolean {
  return BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS.some((option) => option.value === value);
}

function findZoneSelection(zones: BilibiliVideoZone[], tid: string | undefined): {
  parent: BilibiliVideoZone | null;
  child: BilibiliVideoZone | null;
} {
  if (!tid) {
    return { parent: null, child: null };
  }

  const numericTID = Number.parseInt(tid, 10);
  if (!Number.isFinite(numericTID) || numericTID <= 0) {
    return { parent: null, child: null };
  }

  for (const zone of zones) {
    if (zone.id === numericTID) {
      return { parent: zone, child: null };
    }

    const child = zone.children?.find((entry) => entry.id === numericTID) ?? null;
    if (child) {
      return { parent: zone, child };
    }
  }

  return { parent: null, child: null };
}

function normalizeTimestamp(value?: number | null): number | null {
  if (!Number.isFinite(value)) {
    return null;
  }

  const numeric = Number(value);
  return numeric > 1_000_000_000_000 ? numeric : numeric * 1000;
}

function formatDateTime(value?: number | null): string {
  const timestamp = normalizeTimestamp(value);
  if (!timestamp) {
    return translateClientText('未记录');
  }

  return new Intl.DateTimeFormat(resolveClientLocale(), {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(timestamp);
}

export default function SettingsPage() {
  const { t } = useI18n();
  const { logout, currentUser, user } = useAuth();
  const { tier } = useMembership();
  const userId = currentUser?.id || user?.uid || '';
  const { settings, loading: settingsLoading, loaded: settingsLoaded, updateSettings } = useUserSettings(userId);
  const { models: modelCatalog, loading: modelCatalogLoading } = useAiModelCatalog({
    onlyAvailable: true,
    mergeFallback: false,
  });
  const [autoUpload, setAutoUpload] = useState(false);
  const [autoUploadInterval, setAutoUploadInterval] = useState('30');
  const [youtubeFeedSyncEnabled, setYouTubeFeedSyncEnabled] = useState(true);
  const [youtubeFeedSyncInterval, setYouTubeFeedSyncInterval] = useState('60');
  const [youtubeFeedSyncLookback, setYouTubeFeedSyncLookback] = useState('7');
  const [systemSettingsLoading, setSystemSettingsLoading] = useState(false);
  const [systemSettingsLoaded, setSystemSettingsLoaded] = useState(false);
  const [systemSettingsError, setSystemSettingsError] = useState('');
  const [translationSourceLang, setTranslationSourceLang] = useState('en');
  const [translationTargetLang, setTranslationTargetLang] = useState('zh-Hans');
  const [speechSynthesisConfig, setSpeechSynthesisConfig] = useState<SpeechSynthesisConfig>(DEFAULT_SPEECH_SYNTHESIS_CONFIG);
  const [ttsSaving, setTTSSaving] = useState(false);
  const [translationModel, setTranslationModel] = useState('default');
  const [metadataModel, setMetadataModel] = useState('default');
  const [bidDefaultLanguage, setBidDefaultLanguage] = useState('zh-Hans');
  const [bidDefaultTone, setBidDefaultTone] = useState('professional');
  const [bidTemplateStyle, setBidTemplateStyle] = useState('standard');
  const [bilibiliZones, setBilibiliZones] = useState<BilibiliVideoZone[]>([]);
  const [bilibiliZonesLoading, setBilibiliZonesLoading] = useState(false);
  const [bilibiliZonesError, setBilibiliZonesError] = useState('');
  const [watermarkPromoEnabled, setWatermarkPromoEnabled] = useState(true);
  const [submissionCopyright, setSubmissionCopyright] = useState('2');
  const [submissionParentTid, setSubmissionParentTid] = useState('');
  const [submissionChildTid, setSubmissionChildTid] = useState('');
  const [apiKeys, setApiKeys] = useState<UserApiKeyRecord[]>([]);
  const [apiKeysLoaded, setApiKeysLoaded] = useState(false);
  const [apiKeysLoading, setApiKeysLoading] = useState(false);
  const [apiKeysError, setApiKeysError] = useState('');
  const [newApiKeyName, setNewApiKeyName] = useState('ytb2bili-default');
  const [creatingApiKey, setCreatingApiKey] = useState(false);
  const [deletingApiKeyId, setDeletingApiKeyId] = useState('');
  const [latestCreatedApiKey, setLatestCreatedApiKey] = useState<UserApiKeyRecord | null>(null);
  const [activeSection, setActiveSection] = useState<SettingsSectionId | null>('profile');
  const normalizedTier = normalizeTier(tier);
  const isPaidMember = normalizedTier !== 'free';
  const isProMember = normalizedTier === 'pro' || normalizedTier === 'enterprise';
  const selectableModels = React.useMemo(
    () => modelCatalog.filter((item) => typeof item.id === 'string' && item.id.trim().length > 0),
    [modelCatalog]
  );

  const currentTranslationModelLabel = selectableModels.find((item) => item.id === translationModel)?.label || t('跟随系统默认');
  const currentMetadataModelLabel = selectableModels.find((item) => item.id === metadataModel)?.label || t('跟随系统默认');
  const currentSubtitleAudioVoiceLabel = getSpeechSynthesisVoiceLabel(speechSynthesisConfig);
  const storedSpeechSynthesisSetting = React.useMemo(() => getStoredSubtitleAudioTTSConfigValue(settings), [settings]);
  const submissionSettingTid = settings[USER_SETTING_KEY_BILIBILI_SUBMISSION_TID];
  const submissionSettingCopyright = settings[USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT];
  const watermarkPromoSetting = settings[USER_SETTING_KEY_WATERMARK_PROMO_ENABLED];
  const currentSubmissionCopyrightLabel = submissionCopyright === '1' ? t('自制') : t('转载');
  const selectedSubmissionParent = React.useMemo(
    () => bilibiliZones.find((zone) => String(zone.id) === submissionParentTid) ?? null,
    [bilibiliZones, submissionParentTid],
  );
  const selectedSubmissionChild = React.useMemo(
    () => selectedSubmissionParent?.children?.find((zone) => String(zone.id) === submissionChildTid) ?? null,
    [selectedSubmissionParent, submissionChildTid],
  );
  const currentSubmissionLabel = selectedSubmissionChild?.name ?? selectedSubmissionParent?.name ?? t('未设置，系统将使用默认分区');
  const selectedSubmissionParentLabel = selectedSubmissionParent?.name ?? t('未选择一级分区');
  const selectedSubmissionChildLabel = selectedSubmissionChild?.name ?? t('将使用一级分区默认投稿');
  const apiKeysReady = apiKeysLoaded || apiKeysLoading;
  const sectionItems: Array<{ id: SettingsSectionId; label: string; icon: React.ElementType }> = [
    { id: 'profile', label: '支持开发者（微信:tradingba）', icon: User },
    { id: 'translation', label: '翻译设置', icon: Languages },
    { id: 'tts', label: '语音合成', icon: AudioLines },
    { id: 'apiKeys', label: 'API 密钥管理', icon: KeyRound },
    { id: 'system', label: '系统设置', icon: SlidersHorizontal },
    { id: 'publishing', label: '上传设置', icon: UploadCloud },
    { id: 'template', label: '模板偏好', icon: LayoutTemplate },
  ];
  const toggleSection = useCallback((sectionId: SettingsSectionId) => {
    setActiveSection((current) => (current === sectionId ? null : sectionId));
  }, []);

  const loadApiKeys = useCallback(async () => {
    if (!userId) {
      return;
    }

    setApiKeysLoading(true);
    setApiKeysError('');
    try {
      const keys = await apiKeysApi.list();
      React.startTransition(() => {
        setApiKeys(keys);
        setApiKeysLoaded(true);
      });
    } catch (error) {
      React.startTransition(() => {
        setApiKeysError(error instanceof Error ? error.message : translateClientText('读取 API 密钥失败'));
        setApiKeysLoaded(true);
      });
    } finally {
      setApiKeysLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    let cancelled = false;

    if (!userId) {
      setBilibiliZones([]);
      setBilibiliZonesError('');
      setSubmissionParentTid('');
      setSubmissionChildTid('');
      return () => {
        cancelled = true;
      };
    }

    setBilibiliZonesLoading(true);
    setBilibiliZonesError('');

    void userSettingsApi.getBilibiliVideoZones()
      .then((zones) => {
        if (cancelled) return;
        setBilibiliZones(zones);
      })
      .catch((error) => {
        if (cancelled) return;
        setBilibiliZones([]);
        setBilibiliZonesError(error instanceof Error ? error.message : translateClientText('获取投稿分区失败'));
      })
      .finally(() => {
        if (!cancelled) {
          setBilibiliZonesLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [userId]);

  useEffect(() => {
  let cancelled = false;

  if (!userId) {
    setYouTubeFeedSyncEnabled(true);
    setYouTubeFeedSyncInterval('60');
    setYouTubeFeedSyncLookback('7');
    setSystemSettingsError('');
    setSystemSettingsLoaded(false);
    setSystemSettingsLoading(false);
    return () => {
    cancelled = true;
    };
  }

  setSystemSettingsLoading(true);
  setSystemSettingsError('');

  void systemSettingsApi.getSettings()
    .then((systemSettings) => {
    if (cancelled) {
      return;
    }

    const nextEnabled = systemSettings[SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_ENABLED] !== '0';
    const nextInterval = isYouTubeFeedSyncIntervalOption(systemSettings[SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_INTERVAL] ?? '')
      ? String(systemSettings[SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_INTERVAL])
      : '60';
    const nextLookback = isYouTubeFeedSyncLookbackOption(systemSettings[SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_LOOKBACK] ?? '')
      ? String(systemSettings[SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_LOOKBACK])
      : '7';

    setYouTubeFeedSyncEnabled(nextEnabled);
    setYouTubeFeedSyncInterval(nextInterval);
    setYouTubeFeedSyncLookback(nextLookback);
    setSystemSettingsLoaded(true);
    })
    .catch((error) => {
    if (cancelled) {
      return;
    }

    setSystemSettingsError(error instanceof Error ? error.message : translateClientText('读取 YouTube feed 同步设置失败'));
    setSystemSettingsLoaded(true);
    })
    .finally(() => {
    if (!cancelled) {
      setSystemSettingsLoading(false);
    }
    });

  return () => {
    cancelled = true;
  };
  }, [userId]);

  useEffect(() => {
    if (!userId) {
      setApiKeys([]);
      setApiKeysLoaded(false);
      setApiKeysLoading(false);
      setApiKeysError('');
      setLatestCreatedApiKey(null);
    }
  }, [userId]);

  useEffect(() => {
    if (activeSection !== 'apiKeys' || !userId || apiKeysLoaded || apiKeysLoading) {
      return;
    }

    void loadApiKeys();
  }, [activeSection, apiKeysLoaded, apiKeysLoading, loadApiKeys, userId]);

  useEffect(() => {
    if (!userId) {
      setAutoUpload(false);
      setAutoUploadInterval('30');
      setTranslationSourceLang('en');
      setTranslationTargetLang('zh-Hans');
      setSpeechSynthesisConfig(DEFAULT_SPEECH_SYNTHESIS_CONFIG);
      setTranslationModel('default');
      setMetadataModel('default');
      setBidDefaultLanguage('zh-Hans');
      setBidDefaultTone('professional');
      setBidTemplateStyle('standard');
      setWatermarkPromoEnabled(true);
      setSubmissionCopyright('2');
      return;
    }

    const storedAutoUpload = readStoredSetting(AUTO_UPLOAD_STORAGE_KEY, userId);
    const storedAutoUploadInterval = readStoredSetting(AUTO_UPLOAD_INTERVAL_STORAGE_KEY, userId);
    const storedTranslationSource = readStoredSetting(TRANSLATION_SOURCE_LANG_STORAGE_KEY, userId);
    const storedTranslationTarget = readStoredSetting(TRANSLATION_TARGET_LANG_STORAGE_KEY, userId);
    const storedSubtitleAudioVoice = readStoredSetting(SUBTITLE_AUDIO_VOICE_STORAGE_KEY, userId);
    const storedTranslationModel = readStoredSetting(TRANSLATION_MODEL_STORAGE_KEY, userId);
    const storedMetadataModel = readStoredSetting(METADATA_MODEL_STORAGE_KEY, userId);
    const storedSubmissionCopyright = readStoredSetting(BILIBILI_SUBMISSION_COPYRIGHT_STORAGE_KEY, userId);
    const storedBidLanguage = readStoredSetting(BID_DEFAULT_LANGUAGE_STORAGE_KEY, userId);
    const storedBidTone = readStoredSetting(BID_DEFAULT_TONE_STORAGE_KEY, userId);
    const storedBidTemplateStyle = readStoredSetting(BID_TEMPLATE_STYLE_STORAGE_KEY, userId);

    const nextAutoUpload = settings.auto_upload === '1'
      ? true
      : settings.auto_upload === '0'
        ? false
        : storedAutoUpload === '1';

    const nextAutoUploadInterval = isIntervalOption(settings.auto_upload_interval_minutes ?? '')
      ? String(settings.auto_upload_interval_minutes)
      : isIntervalOption(storedAutoUploadInterval ?? '')
        ? String(storedAutoUploadInterval)
        : '30';

    const nextTranslationSourceLang = isLanguageOption(settings.translation_source_lang ?? '', true)
      ? String(settings.translation_source_lang)
      : isLanguageOption(storedTranslationSource ?? '', true)
        ? String(storedTranslationSource)
        : 'en';

    const nextTranslationTargetLang = isLanguageOption(settings.translation_target_lang ?? '', false)
      ? String(settings.translation_target_lang)
      : isLanguageOption(storedTranslationTarget ?? '', false)
        ? String(storedTranslationTarget)
        : 'zh-Hans';

    const nextSpeechSynthesisConfig = parseSpeechSynthesisConfig(storedSpeechSynthesisSetting)
      ?? parseSpeechSynthesisConfig(storedSubtitleAudioVoice)
      ?? DEFAULT_SPEECH_SYNTHESIS_CONFIG;

    const nextTranslationModel = (settings.translation_model ?? storedTranslationModel) || 'default';
    const nextMetadataModel = (settings.metadata_model ?? storedMetadataModel) || 'default';

    const nextBidDefaultLanguage = isLanguageOption(settings.bid_default_language ?? '', false)
      ? String(settings.bid_default_language)
      : isLanguageOption(storedBidLanguage ?? '', false)
        ? String(storedBidLanguage)
        : 'zh-Hans';

    const nextBidDefaultTone = isBidToneOption(settings.bid_default_tone ?? '')
      ? String(settings.bid_default_tone)
      : isBidToneOption(storedBidTone ?? '')
        ? String(storedBidTone)
        : 'professional';

    const nextBidTemplateStyle = isBidTemplateStyleOption(settings.bid_template_style ?? '')
      ? String(settings.bid_template_style)
      : isBidTemplateStyleOption(storedBidTemplateStyle ?? '')
        ? String(storedBidTemplateStyle)
        : 'standard';

    const nextSubmissionCopyright = isBilibiliSubmissionCopyrightOption(submissionSettingCopyright ?? '')
      ? String(submissionSettingCopyright)
      : isBilibiliSubmissionCopyrightOption(storedSubmissionCopyright ?? '')
        ? String(storedSubmissionCopyright)
        : '2';
    const nextWatermarkPromoEnabled = watermarkPromoSetting !== '0';

    setAutoUpload(nextAutoUpload);
    setAutoUploadInterval(nextAutoUploadInterval);
    setTranslationSourceLang(nextTranslationSourceLang);
    setTranslationTargetLang(nextTranslationTargetLang);
    setSpeechSynthesisConfig(nextSpeechSynthesisConfig);
    setTranslationModel(nextTranslationModel);
    setMetadataModel(nextMetadataModel);
    setWatermarkPromoEnabled(nextWatermarkPromoEnabled);
    setSubmissionCopyright(nextSubmissionCopyright);
    setBidDefaultLanguage(nextBidDefaultLanguage);
    setBidDefaultTone(nextBidDefaultTone);
    setBidTemplateStyle(nextBidTemplateStyle);
  }, [
    settings.auto_upload,
    settings.auto_upload_interval_minutes,
    settings.translation_source_lang,
    settings.translation_target_lang,
    storedSpeechSynthesisSetting,
    settings.translation_model,
    settings.metadata_model,
    watermarkPromoSetting,
    submissionSettingCopyright,
    settings.bid_default_language,
    settings.bid_default_tone,
    settings.bid_template_style,
    userId,
  ]);

  useEffect(() => {
    const selection = findZoneSelection(bilibiliZones, submissionSettingTid);
    setSubmissionParentTid(selection.parent ? String(selection.parent.id) : '');
    setSubmissionChildTid(selection.child ? String(selection.child.id) : '');
  }, [bilibiliZones, submissionSettingTid]);

  const handleAutoUploadChange = useCallback((nextValue: boolean) => {
    setAutoUpload(nextValue);
    writeStoredSetting(AUTO_UPLOAD_STORAGE_KEY, userId, nextValue ? '1' : '0');
    void updateSettings({ auto_upload: nextValue ? '1' : '0' }).catch(() => {
      toast.error(translateClientText('保存自动上传设置失败'));
    });
  }, [updateSettings, userId]);

  const handleAutoUploadIntervalChange = useCallback((nextValue: string) => {
    setAutoUploadInterval(nextValue);
    writeStoredSetting(AUTO_UPLOAD_INTERVAL_STORAGE_KEY, userId, nextValue);
    void updateSettings({ auto_upload_interval_minutes: nextValue }).catch(() => {
      toast.error(translateClientText('保存自动上传时间间隔失败'));
    });
  }, [updateSettings, userId]);

  const handleTranslationSettingChange = useCallback((key: 'translation_source_lang' | 'translation_target_lang', nextValue: string) => {
    if (key === 'translation_source_lang') {
      setTranslationSourceLang(nextValue);
      writeStoredSetting(TRANSLATION_SOURCE_LANG_STORAGE_KEY, userId, nextValue);
    } else {
      setTranslationTargetLang(nextValue);
      writeStoredSetting(TRANSLATION_TARGET_LANG_STORAGE_KEY, userId, nextValue);
    }

    void updateSettings({ [key]: nextValue }).catch(() => {
      toast.error(translateClientText('保存翻译设置失败'));
    });
  }, [updateSettings, userId]);

  const handleYouTubeFeedSyncEnabledChange = useCallback((nextValue: boolean) => {
  setYouTubeFeedSyncEnabled(nextValue);
  setSystemSettingsError('');
  void systemSettingsApi.updateSettings({
    [SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_ENABLED]: nextValue ? '1' : '0',
  }).catch((error) => {
    setSystemSettingsError(error instanceof Error ? error.message : translateClientText('保存 YouTube feed 同步开关失败'));
    toast.error(translateClientText('保存 YouTube feed 同步开关失败'));
  });
  }, []);

  const handleYouTubeFeedSyncIntervalChange = useCallback((nextValue: string) => {
  setYouTubeFeedSyncInterval(nextValue);
  setSystemSettingsError('');
  void systemSettingsApi.updateSettings({
    [SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_INTERVAL]: nextValue,
  }).catch((error) => {
    setSystemSettingsError(error instanceof Error ? error.message : translateClientText('保存 YouTube feed 同步间隔失败'));
    toast.error(translateClientText('保存 YouTube feed 同步间隔失败'));
  });
  }, []);

  const handleYouTubeFeedSyncLookbackChange = useCallback((nextValue: string) => {
  setYouTubeFeedSyncLookback(nextValue);
  setSystemSettingsError('');
  void systemSettingsApi.updateSettings({
    [SYSTEM_SETTING_KEY_YOUTUBE_FEED_SYNC_LOOKBACK]: nextValue,
  }).catch((error) => {
    setSystemSettingsError(error instanceof Error ? error.message : translateClientText('保存 YouTube feed 同步时间范围失败'));
    toast.error(translateClientText('保存 YouTube feed 同步时间范围失败'));
  });
  }, []);

  const handleSpeechSynthesisConfigChange = useCallback((patch: Partial<SpeechSynthesisConfig>) => {
    setSpeechSynthesisConfig((prev) => normalizeSpeechSynthesisConfig({ ...prev, ...patch }));
  }, []);

  const handleSpeechSynthesisVoiceSelect = useCallback((voice: TTSVoiceRecord) => {
    setSpeechSynthesisConfig((prev) => normalizeSpeechSynthesisConfig({
      ...prev,
      voice_name: voice.shortName,
      language: voice.locale || prev.language,
      provider: prev.provider === 'auto' ? prev.provider : voice.provider,
    }));
  }, []);

  const handleSaveSpeechSynthesisConfig = useCallback(async () => {
    const normalizedConfig = normalizeSpeechSynthesisConfig(speechSynthesisConfig);
    const serializedConfig = serializeSpeechSynthesisConfig(normalizedConfig);
    setTTSSaving(true);
    setSpeechSynthesisConfig(normalizedConfig);
    writeStoredSetting(SUBTITLE_AUDIO_VOICE_STORAGE_KEY, userId, serializedConfig);
    try {
      await updateSettings({ [USER_SETTING_KEY_SUBTITLE_AUDIO_TTS_CONFIG]: serializedConfig });
      toast.success(translateClientText('字幕配音配置已保存'));
    } catch {
      toast.error(translateClientText('保存字幕配音配置失败'));
    } finally {
      setTTSSaving(false);
    }
  }, [speechSynthesisConfig, updateSettings, userId]);

  const handleResetSpeechSynthesisConfig = useCallback(() => {
    setSpeechSynthesisConfig(DEFAULT_SPEECH_SYNTHESIS_CONFIG);
  }, []);

  const handleTranslationModelChange = useCallback((nextValue: string) => {
    setTranslationModel(nextValue);
    writeStoredSetting(TRANSLATION_MODEL_STORAGE_KEY, userId, nextValue);
    void updateSettings({ translation_model: nextValue === 'default' ? '' : nextValue }).catch(() => {
      toast.error(translateClientText('保存翻译模型失败'));
    });
  }, [updateSettings, userId]);

  const handleMetadataModelChange = useCallback((nextValue: string) => {
    setMetadataModel(nextValue);
    writeStoredSetting(METADATA_MODEL_STORAGE_KEY, userId, nextValue);
    void updateSettings({ metadata_model: nextValue === 'default' ? '' : nextValue }).catch(() => {
      toast.error(translateClientText('保存元数据模型失败'));
    });
  }, [updateSettings, userId]);

  const handleBidDefaultLanguageChange = useCallback((nextValue: string) => {
    setBidDefaultLanguage(nextValue);
    writeStoredSetting(BID_DEFAULT_LANGUAGE_STORAGE_KEY, userId, nextValue);
    void updateSettings({ bid_default_language: nextValue }).catch(() => {
      toast.error(translateClientText('保存默认语言失败'));
    });
  }, [updateSettings, userId]);

  const handleBidDefaultToneChange = useCallback((nextValue: string) => {
    setBidDefaultTone(nextValue);
    writeStoredSetting(BID_DEFAULT_TONE_STORAGE_KEY, userId, nextValue);
    void updateSettings({ bid_default_tone: nextValue }).catch(() => {
      toast.error(translateClientText('保存默认语气失败'));
    });
  }, [updateSettings, userId]);

  const handleBidTemplateStyleChange = useCallback((nextValue: string) => {
    setBidTemplateStyle(nextValue);
    writeStoredSetting(BID_TEMPLATE_STYLE_STORAGE_KEY, userId, nextValue);
    void updateSettings({ bid_template_style: nextValue }).catch(() => {
      toast.error(translateClientText('保存模板风格失败'));
    });
  }, [updateSettings, userId]);

  const handleSubmissionParentChange = useCallback((nextValue: string) => {
    const nextParent = bilibiliZones.find((zone) => String(zone.id) === nextValue);
    if (!nextParent) {
      return;
    }

    const nextChild = nextParent.children?.[0] ?? null;
    const nextTid = nextChild ? String(nextChild.id) : nextValue;

    setSubmissionParentTid(nextValue);
    setSubmissionChildTid(nextChild ? String(nextChild.id) : '');
    void updateSettings({ [USER_SETTING_KEY_BILIBILI_SUBMISSION_TID]: nextTid }).catch(() => {
      toast.error(translateClientText('保存投稿分区失败'));
    });
  }, [bilibiliZones, updateSettings]);

  const handleSubmissionChildChange = useCallback((nextValue: string) => {
    setSubmissionChildTid(nextValue);
    void updateSettings({ [USER_SETTING_KEY_BILIBILI_SUBMISSION_TID]: nextValue }).catch(() => {
      toast.error(translateClientText('保存投稿分区失败'));
    });
  }, [updateSettings]);

  const handleSubmissionCopyrightChange = useCallback((nextValue: string) => {
    setSubmissionCopyright(nextValue);
    writeStoredSetting(BILIBILI_SUBMISSION_COPYRIGHT_STORAGE_KEY, userId, nextValue);
    void updateSettings({ [USER_SETTING_KEY_BILIBILI_SUBMISSION_COPYRIGHT]: nextValue }).catch(() => {
      toast.error(translateClientText('保存稿件类型失败'));
    });
  }, [updateSettings, userId]);

  const handleWatermarkPromoEnabledChange = useCallback((nextValue: boolean) => {
    if (!isProMember && !nextValue) {
      toast.error(translateClientText('仅 Pro 会员可关闭上传宣传文案'));
      return;
    }
    setWatermarkPromoEnabled(nextValue);
    void updateSettings({ [USER_SETTING_KEY_WATERMARK_PROMO_ENABLED]: nextValue ? '1' : '0' }).catch(() => {
      toast.error(translateClientText('保存宣传文案开关失败'));
    });
  }, [isProMember, updateSettings]);

  const handleCreateApiKey = useCallback(async () => {
    const trimmedName = newApiKeyName.trim();
    if (!trimmedName) {
      toast.error(translateClientText('请输入 API 密钥名称'));
      return;
    }

    setCreatingApiKey(true);
    setApiKeysError('');
    try {
      const createdKey = await apiKeysApi.create(trimmedName);
      React.startTransition(() => {
        setApiKeys((current) => [createdKey, ...current.filter((item) => item.id !== createdKey.id)]);
        setLatestCreatedApiKey(createdKey);
        setApiKeysLoaded(true);
      });
      toast.success(translateClientText('创建 API 密钥成功'));
    } catch (error) {
      const message = error instanceof Error ? error.message : translateClientText('创建 API 密钥失败');
      setApiKeysError(message);
      toast.error(message);
    } finally {
      setCreatingApiKey(false);
    }
  }, [newApiKeyName]);

  const handleDeleteApiKey = useCallback(async (key: UserApiKeyRecord) => {
    if (!window.confirm(translateClientText('确认删除 API 密钥“{name}”吗？此操作不可撤销。', { name: key.name }))) {
      return;
    }

    setDeletingApiKeyId(key.id);
    setApiKeysError('');
    try {
      await apiKeysApi.remove(key.id);
      React.startTransition(() => {
        setApiKeys((current) => current.filter((item) => item.id !== key.id));
        setLatestCreatedApiKey((current) => (current?.id === key.id ? null : current));
      });
      toast.success(translateClientText('API 密钥已删除'));
    } catch (error) {
      const message = error instanceof Error ? error.message : translateClientText('删除 API 密钥失败');
      setApiKeysError(message);
      toast.error(message);
    } finally {
      setDeletingApiKeyId('');
    }
  }, []);

  const handleCopyApiKeySecret = useCallback(async (secret: string) => {
    try {
      await navigator.clipboard.writeText(secret);
      toast.success(translateClientText('API 密钥已复制'));
    } catch {
      toast.error(translateClientText('复制失败，请手动复制'));
    }
  }, []);

  const handleCopyApiKey = useCallback(async (key: UserApiKeyRecord) => {
    if (!key.secret) {
      toast.error(translateClientText('出于安全限制，旧密钥列表不会再次返回完整 secret。请在创建时立即复制并保存。'));
      return;
    }

    await handleCopyApiKeySecret(key.secret);
  }, [handleCopyApiKeySecret]);

  return (
    <div className="min-h-screen bg-[#f4f7fb]">
      <div className="mx-auto max-w-5xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="mb-6">
          <h1 className="text-4xl font-semibold tracking-[-0.04em] text-slate-950">{t('设置')}</h1>
          <p className="mt-2 text-base text-slate-600">{t('管理您的账户设置和偏好')}</p>
        </div>

        <div className="space-y-4">
          <SettingsCard>
            <div className="space-y-2 p-2">
              {sectionItems.map((item) => (
                <div key={item.id} className="rounded-xl border border-slate-200 bg-white">
                  <AccordionItem
                    icon={item.icon}
                    label={t(item.label)}
                    active={activeSection === item.id}
                    onClick={() => toggleSection(item.id)}
                  />

                  {activeSection === item.id && item.id === 'profile' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <div className="rounded-[28px] border border-amber-100 bg-[linear-gradient(180deg,rgba(255,251,235,0.95),rgba(255,255,255,1))] p-4 sm:p-5">
                          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                            <div className="max-w-3xl space-y-2">
                              <p className="text-[17px] font-semibold leading-7 text-slate-950 sm:text-[18px]">{t('每一次更新、每一次修复，背后都是看不见的成本。')}</p>
                              <p className="text-sm leading-6 text-slate-500">
                                {t('当它在默默为你节省时间、提高产出时，成为付费用户，就是对开发者最好的鼓励。你支持的不只是一款工具，更是它持续进化、不断变好的未来。')}
                              </p>
                              <div className="flex flex-wrap gap-2 pt-0.5 text-xs font-medium text-slate-600">
                                <span className="rounded-full bg-white px-3 py-1 shadow-sm shadow-slate-200/70 ring-1 ring-slate-200">{t('支持持续迭代')}</span>
                                <span className="rounded-full bg-white px-3 py-1 shadow-sm shadow-slate-200/70 ring-1 ring-slate-200">{t('解锁更完整能力')}</span>
                                <span className="rounded-full bg-white px-3 py-1 shadow-sm shadow-slate-200/70 ring-1 ring-slate-200">{t('帮助开发者长期维护')}</span>
                              </div>
                            </div>
                          <div className="flex shrink-0 flex-col gap-2 lg:min-w-[240px] lg:items-end">
                            <Link
                              href="/membership"
                              className="inline-flex h-11 items-center justify-center gap-2 rounded-2xl border border-amber-200 bg-amber-50 px-5 text-sm font-semibold text-amber-800 transition hover:border-amber-300 hover:bg-amber-100/80"
                            >
                              <Crown className="h-4 w-4 text-amber-500" />
                              <span>{isPaidMember ? t('继续支持开发者') : t('赞助开发者并升级')}</span>
                            </Link>
                          </div>
                        </div>
                        </div>
                        <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:justify-end">
                          <button
                            type="button"
                            className="rounded-2xl border border-slate-200 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
                          >
                            {t('继续使用当前方案')}
                          </button>
                          <Link
                            href="/membership"
                            className="inline-flex items-center justify-center rounded-2xl bg-amber-500 px-5 py-2.5 text-sm font-semibold text-white shadow-lg shadow-amber-500/20 transition hover:bg-amber-600"
                          >
                            {t('立即赞助开发者')}
                          </Link>
                        </div>
                      </SettingsCardContent>
                    </div>
                  )}

                  {activeSection === item.id && item.id === 'translation' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <SectionHint text={t('这里仅管理字幕翻译链路本身，包括输入语言、输出语言和默认翻译模型。')} />
                        <div className="space-y-3">
                          <SurfaceCard
                            title={t('语言偏好')}
                            description={t('定义系统如何理解原始视频语言，以及翻译后的默认输出语言。')}
                            tone="accent"
                          >
                            <div className="space-y-2.5">
                              <SettingsRow
                                title={t('源语言')}
                                description={t('视频内容原始语言，设置为自动检测时由系统智能识别')}
                              >
                                <Select
                                  value={translationSourceLang}
                                  disabled={settingsLoading && !settingsLoaded}
                                  onValueChange={(value: string) => handleTranslationSettingChange('translation_source_lang', value)}
                                >
                                  <SelectTrigger>
                                    <SelectValue placeholder={t('选择源语言')} />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {LANGUAGE_OPTIONS.map((option) => (
                                      <SelectItem key={option.value} value={option.value}>
                                        {t(option.label)}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                              </SettingsRow>

                              <SettingsRow
                                title={t('目标语言')}
                                description={t('字幕翻译后的默认输出语言')}
                              >
                                <Select
                                  value={translationTargetLang}
                                  disabled={settingsLoading && !settingsLoaded}
                                  onValueChange={(value: string) => handleTranslationSettingChange('translation_target_lang', value)}
                                >
                                  <SelectTrigger>
                                    <SelectValue placeholder={t('选择目标语言')} />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {LANGUAGE_OPTIONS.filter((option) => option.value !== 'auto').map((option) => (
                                      <SelectItem key={option.value} value={option.value}>
                                        {t(option.label)}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                              </SettingsRow>
                            </div>
                          </SurfaceCard>

                          <SurfaceCard
                            title={t('翻译模型')}
                            description={t('为字幕翻译单独指定模型，不设置时继续跟随系统默认。')}
                          >
                            <SettingsRow
                              title={t('默认翻译模型')}
                              description={t('当前：{label}', { label: currentTranslationModelLabel })}
                            >
                              <Select
                                value={translationModel}
                                disabled={(settingsLoading && !settingsLoaded) || (modelCatalogLoading && selectableModels.length === 0)}
                                onValueChange={handleTranslationModelChange}
                              >
                                <SelectTrigger>
                                  <SelectValue placeholder={t('跟随系统默认')} />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="default">{t('跟随系统默认')}</SelectItem>
                                  {selectableModels.map((item) => (
                                    <SelectItem key={item.id} value={item.id}>
                                      {item.label}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </SettingsRow>
                          </SurfaceCard>
                        </div>
                      </SettingsCardContent>
                    </div>
                  )}

                  {activeSection === item.id && item.id === 'tts' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <SectionHint text={t('语音合成配置单独维护。这里决定默认音色、供应商以及字幕配音的语速、音量、音高。')} />
                        <div className="space-y-3">
                          <SurfaceCard
                            title={t('音色与服务')}
                            description={t('先确定默认音色，再决定由系统自动路由还是固定供应商执行。')}
                            tone="accent"
                          >
                            <div className="space-y-2.5">

 <SettingsRow
                                title={t('TTS 提供商')}
                                description={t('默认自动选择，可固定到具体供应商。')}
                              >
                                <Select
                                  value={speechSynthesisConfig.provider ?? 'auto'}
                                  disabled={settingsLoading && !settingsLoaded}
                                  onValueChange={(value: string) => handleSpeechSynthesisConfigChange({ provider: value })}
                                >
                                  <SelectTrigger>
                                    <SelectValue placeholder={t('选择 TTS 提供商')} />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {SPEECH_SYNTHESIS_PROVIDER_OPTIONS.map((option) => (
                                      <SelectItem key={option.value} value={option.value}>
                                        {option.label}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                              </SettingsRow>

                              <SettingsRow
                                title={t('字幕配音音色')}
                                description={t('当前：{label}，用于翻译后字幕的默认音频合成', { label: currentSubtitleAudioVoiceLabel })}
                              >
                                <VoicePicker
                                  value={speechSynthesisConfig.voice_name}
                                  disabled={settingsLoading && !settingsLoaded}
                                  provider={speechSynthesisConfig.provider ?? 'auto'}
                                  preferredLocale={speechSynthesisConfig.language}
                                  onChange={(nextValue) => handleSpeechSynthesisConfigChange({ voice_name: nextValue })}
                                  onSelectVoice={handleSpeechSynthesisVoiceSelect}
                                />
                              </SettingsRow>

                             

                              <div className="grid gap-2.5 lg:grid-cols-2">
                                <SettingsRow
                                  title={t('语言 Locale')}
                                  description={t('例如 zh-CN、en-US、ja-JP。')}
                                >
                                  <Input
                                    value={speechSynthesisConfig.language}
                                    onChange={(event) => handleSpeechSynthesisConfigChange({ language: event.target.value })}
                                    placeholder="zh-CN"
                                  />
                                </SettingsRow>
                                <SettingsRow
                                  title={t('输出格式')}
                                  description={t('提交任务时默认带上的音频格式。')}
                                >
                                  <Select
                                    value={speechSynthesisConfig.format}
                                    disabled={settingsLoading && !settingsLoaded}
                                    onValueChange={(value: string) => handleSpeechSynthesisConfigChange({ format: value })}
                                  >
                                    <SelectTrigger>
                                      <SelectValue placeholder={t('选择格式')} />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {SPEECH_SYNTHESIS_FORMAT_OPTIONS.map((option) => (
                                        <SelectItem key={option.value} value={option.value}>
                                          {option.label}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </SettingsRow>
                              </div>

                   
                              <SpeechSlider
                                title={t('语速')}
                                description={t('默认 1.00，数值越大语速越快，适合根据字幕节奏微调。')}
                                value={speechSynthesisConfig.rate ?? 1}
                                min={0.5}
                                max={2}
                                step={0.05}
                                defaultValue={1}
                                onChange={(value) => handleSpeechSynthesisConfigChange({ rate: value })}
                                formatValue={(value) => value.toFixed(2)}
                              />
                              <SpeechSlider
                                title={t('音量')}
                                description={t('默认 100，可下调减轻背景声压迫，也可适度提高人声存在感。')}
                                value={speechSynthesisConfig.volume ?? 100}
                                min={0}
                                max={200}
                                step={1}
                                defaultValue={100}
                                onChange={(value) => handleSpeechSynthesisConfigChange({ volume: value })}
                                formatValue={(value) => `${Math.round(value)}`}
                              />
                              <SpeechSlider
                                title={t('音高')}
                                description={t('默认 0，负值更低沉，正值更明亮，建议小步微调。')}
                                value={speechSynthesisConfig.pitch ?? 0}
                                min={-20}
                                max={20}
                                step={1}
                                defaultValue={0}
                                onChange={(value) => handleSpeechSynthesisConfigChange({ pitch: value })}
                                formatValue={(value) => `${value > 0 ? '+' : ''}${Math.round(value)}`}
                              />
                            </div>
                          </SurfaceCard>

                          <div className="flex items-center justify-end gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3">
                            <Button variant="outline" size="sm" onClick={handleResetSpeechSynthesisConfig} disabled={ttsSaving}>
                              {t('重置默认值')}
                            </Button>
                            <Button size="sm" onClick={handleSaveSpeechSynthesisConfig} disabled={ttsSaving || (settingsLoading && !settingsLoaded)}>
                              {ttsSaving ? t('保存中...') : t('保存字幕配音配置')}
                            </Button>
                          </div>
                        </div>
                      </SettingsCardContent>
                    </div>
                  )}

                  {activeSection === item.id && item.id === 'system' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <SectionHint text={t('配置默认模型、通用行为和浏览器侧偏好')} />
                        <SurfaceCard
                          title={t('YouTube feed 同步')}
                          description={t('绑定 YouTube 账号后，可在此配置订阅同步开关与间隔。')}
                          tone="accent"
                        >
                          <div className="space-y-3">
                            <SettingsRow
                              title={t('启用后台同步')}
                              description={t('关闭后，系统将暂停自动同步订阅频道更新。')}
                            >
                              <Switch
                                checked={youtubeFeedSyncEnabled}
                                disabled={systemSettingsLoading && !systemSettingsLoaded}
                                onChange={handleYouTubeFeedSyncEnabledChange}
                              />
                            </SettingsRow>

                            <SettingsRow
                              title={t('同步间隔')}
                              description={t('后台重新检查 YouTube feed 的频率。')}
                            >
                              <Select
                                value={youtubeFeedSyncInterval}
                                disabled={(systemSettingsLoading && !systemSettingsLoaded) || !youtubeFeedSyncEnabled}
                                onValueChange={handleYouTubeFeedSyncIntervalChange}
                              >
                                <SelectTrigger>
                                  <SelectValue placeholder={t('选择时间间隔')} />
                                </SelectTrigger>
                                <SelectContent>
                                  {YOUTUBE_FEED_SYNC_INTERVAL_OPTIONS.map((minutes) => (
                                    <SelectItem key={minutes} value={String(minutes)}>
                                      {minutes < 60 ? t('{minutes} 分钟', { minutes }) : minutes % 60 === 0 ? t('{hours} 小时', { hours: minutes / 60 }) : t('{minutes} 分钟', { minutes })}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </SettingsRow>

                            <SettingsRow
                              title={t('同步时间范围')}
                              description={t('仅同步最近一段时间内发布的视频。')}
                            >
                              <Select
                                value={youtubeFeedSyncLookback}
                                disabled={(systemSettingsLoading && !systemSettingsLoaded) || !youtubeFeedSyncEnabled}
                                onValueChange={handleYouTubeFeedSyncLookbackChange}
                              >
                                <SelectTrigger>
                                  <SelectValue placeholder={t('选择时间范围')} />
                                </SelectTrigger>
                                <SelectContent>
                                  {YOUTUBE_FEED_SYNC_LOOKBACK_OPTIONS.map((days) => (
                                    <SelectItem key={days} value={String(days)}>
                                      {t('最近 {days} 天', { days })}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </SettingsRow>
                            {systemSettingsError ? (
                              <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] leading-5 text-amber-800">
                                {systemSettingsError}
                              </div>
                            ) : null}
                          </div>
                        </SurfaceCard>

                        <SettingsRow
                          title={t('元数据模型')}
                          description={t('当前：{label}', { label: currentMetadataModelLabel })}
                        >
                          <Select
                            value={metadataModel}
                            disabled={(settingsLoading && !settingsLoaded) || (modelCatalogLoading && selectableModels.length === 0)}
                            onValueChange={handleMetadataModelChange}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('跟随系统默认')} />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="default">{t('跟随系统默认')}</SelectItem>
                              {selectableModels.map((item) => (
                                <SelectItem key={item.id} value={item.id}>
                                  {item.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </SettingsRow>

                        <div className="space-y-3">
                          <UpdateManager />
                          <CookiesManager />
                        </div>
                      </SettingsCardContent>
                    </div>
                  )}

                  {activeSection === item.id && item.id === 'apiKeys' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <SectionHint text={t('直接在当前页面管理 API key。支持创建新 key、复制新生成的明文 secret，以及删除不再使用的 key。')} />

                        <div className="space-y-3">
                          <SurfaceCard
                            title={t('创建新密钥')}
                            description={t('建议按用途命名，例如字幕配音、批量脚本或外部集成，便于后续排查与轮换。')}
                            tone="accent"
                          >
                            <div className="grid gap-3 lg:grid-cols-[1fr_auto] lg:items-end">
                              <label className="block">
                                <span className="mb-2 block text-sm font-medium text-slate-900">{t('密钥名称')}</span>
                                <Input
                                  value={newApiKeyName}
                                  onChange={(event) => setNewApiKeyName(event.target.value)}
                                  placeholder={t('例如：subtitle-tts-runner')}
                                />
                              </label>
                              <Button size="sm" onClick={handleCreateApiKey} disabled={creatingApiKey}>
                                {creatingApiKey ? t('创建中...') : t('创建 API 密钥')}
                              </Button>
                            </div>

                            {latestCreatedApiKey?.secret ? (
                              <div className="mt-4 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-4">
                                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                                  <div>
                                    <p className="text-sm font-semibold text-emerald-900">{t('新密钥已生成')}</p>
                                    <p className="mt-1 text-[13px] leading-5 text-emerald-800/80">
                                      {t('明文 secret 通常只会在创建成功这一刻返回一次。建议现在就复制并保存到你的脚本或环境变量里。')}
                                    </p>
                                  </div>
                                  <button
                                    type="button"
                                    onClick={() => void handleCopyApiKeySecret(latestCreatedApiKey.secret ?? '')}
                                    className="rounded-full border border-emerald-300 bg-white px-3 py-1.5 text-xs font-semibold text-emerald-800 transition hover:bg-emerald-100"
                                  >
                                    {t('复制明文密钥')}
                                  </button>
                                </div>
                                <div className="mt-3 overflow-x-auto rounded-xl border border-emerald-200 bg-white px-3 py-2 font-mono text-sm text-slate-900">
                                  {latestCreatedApiKey.secret}
                                </div>
                              </div>
                            ) : null}
                          </SurfaceCard>

                          <SurfaceCard
                            title={t('当前密钥列表')}
                            description={t('共 {count} 个 key。删除后该 key 将立即失效，适合清理旧脚本或泄露风险。', { count: apiKeys.length })}
                          >
                            <div className="mb-3 flex items-center justify-between gap-3">
                              <div className="text-sm text-slate-500">
                                {apiKeysLoading ? t('正在同步密钥列表...') : t('列表展示当前账号下可管理的 API key。')}
                              </div>
                              <button
                                type="button"
                                onClick={() => void loadApiKeys()}
                                disabled={apiKeysLoading}
                                className="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                              >
                                {apiKeysLoading ? t('刷新中...') : t('刷新列表')}
                              </button>
                            </div>

                            {apiKeysError ? (
                              <div className="mb-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] leading-5 text-amber-800">
                                {apiKeysError}
                              </div>
                            ) : null}

                            {!apiKeysReady ? (
                              <div className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-6 text-center text-sm text-slate-500">
                                {t('正在加载 API 密钥列表...')}
                              </div>
                            ) : null}

                            {apiKeysReady && apiKeys.length === 0 && !apiKeysLoading && !apiKeysError ? (
                              <div className="rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-4 py-6 text-center text-sm text-slate-500">
                                {t('当前还没有 API 密钥。建议先创建一个默认 key，用于字幕配音或其他自动化调用。')}
                              </div>
                            ) : null}

                            <div className="space-y-3">
                              {apiKeys.map((key) => (
                                <div key={key.id} className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm shadow-slate-100/70">
                                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                                    <div className="min-w-0">
                                      <div className="flex flex-wrap items-center gap-2">
                                        <p className="text-[15px] font-semibold text-slate-950">{key.name || t('未命名密钥')}</p>
                                        <span className={cn(
                                          'rounded-full px-2.5 py-1 text-[11px] font-semibold',
                                          key.active ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200' : 'bg-slate-100 text-slate-500 ring-1 ring-slate-200'
                                        )}>
                                          {key.active ? t('启用中') : t('已停用')}
                                        </span>
                                      </div>
                                      <div className="mt-2 flex flex-wrap gap-3 text-[13px] text-slate-500">
                                        <span>{t('前缀：')}<span className="font-mono text-slate-700">{key.keyPrefix}</span></span>
                                        <span>{t('创建于：')}{formatDateTime(key.createdAt)}</span>
                                        <span>{t('最近使用：')}{formatDateTime(key.lastUsedAt)}</span>
                                      </div>
                                    </div>
                                    <div className="flex items-center gap-2">
                                      <button
                                        type="button"
                                        onClick={() => void handleCopyApiKey(key)}
                                        disabled={!key.secret}
                                        className="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                                      >
                                        {key.secret ? t('复制密钥') : t('仅创建时可复制')}
                                      </button>
                                      <button
                                        type="button"
                                        onClick={() => void handleDeleteApiKey(key)}
                                        disabled={deletingApiKeyId === key.id}
                                        className="rounded-full border border-rose-200 px-3 py-1.5 text-xs font-semibold text-rose-700 transition hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60"
                                      >
                                        {deletingApiKeyId === key.id ? t('删除中...') : t('删除密钥')}
                                      </button>
                                    </div>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </SurfaceCard>
                        </div>
                      </SettingsCardContent>
                    </div>
                  )}

                  {activeSection === item.id && item.id === 'publishing' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <SectionHint text={t('把默认投稿分区和自动上传策略集中在同一组管理，避免上传链路配置分散。')} />

                        <SurfaceCard
                          title={t('投稿分区')}
                          description={t('手动上传和自动上传都会复用这里的默认投稿分区与稿件类型，建议优先选择二级分区。')}
                          tone="accent"
                        >
                          <div className="space-y-3">
                            <SettingsRow
                              title={t('携带上传宣传文案')}
                              description={isProMember ? t('关闭后，上传简介将不再自动追加 ytb2bili 宣传文案。') : t('默认会附带 ytb2bili 宣传文案，升级 Pro 后可关闭。')}
                            >
                              <Switch
                                checked={watermarkPromoEnabled}
                                disabled={(settingsLoading && !settingsLoaded) || !isProMember}
                                onChange={handleWatermarkPromoEnabledChange}
                              />
                            </SettingsRow>

                            <SettingsRow
                              title={t('稿件类型')}
                              description={t('控制 B 站投稿按“自制”还是“转载”提交。转载会自动附带来源链接。')}
                            >
                              <fieldset className="space-y-2">
                                <legend className="sr-only">{t('稿件类型')}</legend>
                                <div className="flex items-center gap-6">
                                  {BILIBILI_SUBMISSION_COPYRIGHT_OPTIONS.map((option) => (
                                    <label key={option.value} className="flex items-center gap-2 text-sm text-slate-700">
                                      <input
                                        type="radio"
                                        name="settings-bilibili-copyright"
                                        value={option.value}
                                        checked={submissionCopyright === option.value}
                                        disabled={settingsLoading && !settingsLoaded}
                                        onChange={(event) => handleSubmissionCopyrightChange(event.target.value)}
                                        className="h-4 w-4 border-slate-300 text-blue-600 focus:ring-blue-500 disabled:cursor-not-allowed"
                                      />
                                      <span>{option.value === '1' ? t('自制') : t('转载')}</span>
                                    </label>
                                  ))}
                                </div>
                              </fieldset>
                            </SettingsRow>

                            <div className="grid gap-3 lg:grid-cols-[1.15fr_0.85fr]">
                            <div className="rounded-2xl border border-slate-200 bg-white/90 p-3">
                              <div className="mb-3 flex items-center justify-between gap-3">
                                <div>
                                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">{t('分区选择')}</p>
                                  <p className="mt-1 text-sm text-slate-600">{t('先选一级分区，再确认具体投稿子分区。')}</p>
                                </div>
                                {bilibiliZonesLoading ? (
                                  <span className="rounded-full border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-700">{t('同步中')}</span>
                                ) : null}
                              </div>

                              {bilibiliZonesError ? (
                                <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] leading-5 text-amber-800">
                                  <p>{t('读取投稿分区失败：{error}', { error: bilibiliZonesError })}</p>
                                  <Link href="/dashboard/accounts" className="mt-1 inline-flex font-semibold text-amber-900 hover:underline">
                                    {t('前往账号管理检查 B 站绑定')}
                                  </Link>
                                </div>
                              ) : null}

                              {!bilibiliZonesLoading && !bilibiliZonesError && bilibiliZones.length === 0 ? (
                                <div className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-[13px] leading-5 text-slate-600">
                                  {t('当前账号未返回可用投稿分区，请重新绑定 B 站账号后再试。')}
                                </div>
                              ) : null}

                              {!bilibiliZonesError && bilibiliZones.length > 0 ? (
                                <div className="grid gap-3 xl:grid-cols-[0.92fr_1.08fr]">
                                  <div className="space-y-2">
                                    <div className="flex items-center justify-between px-1">
                                      <span className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">{t('一级分区')}</span>
                                      <span className="text-xs text-slate-500">{t('共 {count} 项', { count: bilibiliZones.length })}</span>
                                    </div>
                                    <div className="max-h-[340px] overflow-y-auto rounded-2xl border border-slate-200 bg-white pr-1">
                                      {bilibiliZones.map((zone) => (
                                        <ZoneChoiceRow
                                          key={zone.id}
                                          title={zone.name}
                                          subtitle={zone.children?.length ? t('{count} 个子分区', { count: zone.children.length }) : t('无二级分区')}
                                          active={String(zone.id) === submissionParentTid}
                                          onClick={() => handleSubmissionParentChange(String(zone.id))}
                                        />
                                      ))}
                                    </div>
                                  </div>

                                  <div className="space-y-2 rounded-2xl border border-slate-200 bg-slate-50/70 p-3">
                                    <div className="flex items-center justify-between px-1">
                                      <span className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">{t('二级分区')}</span>
                                      <span className="text-xs text-slate-500">
                                        {selectedSubmissionParent?.children?.length ? t('{count} 个可选项', { count: selectedSubmissionParent.children.length }) : t('跟随一级分区')}
                                      </span>
                                    </div>

                                    {!selectedSubmissionParent ? (
                                      <div className="flex min-h-[220px] items-center justify-center rounded-2xl border border-dashed border-slate-300 bg-white px-6 text-center text-sm leading-6 text-slate-500">
                                        {t('先在左侧选择一级分区，再精确设置投稿落点。')}
                                      </div>
                                    ) : !selectedSubmissionParent.children?.length ? (
                                      <div className="flex min-h-[220px] items-center justify-center rounded-2xl border border-dashed border-slate-300 bg-white px-6 text-center text-sm leading-6 text-slate-500">
                                        {t('当前一级分区没有可选的二级分区，系统会直接使用“{name}”。', { name: selectedSubmissionParent.name })}
                                      </div>
                                    ) : (
                                      <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
                                        {selectedSubmissionParent.children.map((zone) => (
                                          <ZoneChoiceRow
                                            key={zone.id}
                                            title={zone.name}
                                            subtitle={`TID ${zone.id}`}
                                            active={String(zone.id) === submissionChildTid}
                                            onClick={() => handleSubmissionChildChange(String(zone.id))}
                                          />
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                </div>
                              ) : null}
                            </div>

                            <div className="space-y-3 self-start lg:sticky lg:top-24">
                              <div className="rounded-2xl border border-slate-200 bg-white/95 p-4 shadow-[0_10px_30px_rgba(15,23,42,0.06)] backdrop-blur-sm">
                                <div className="flex items-start justify-between gap-3">
                                  <div>
                                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">{t('当前生效')}</p>
                                    <p className="mt-1 text-sm text-slate-600">{t('滚动浏览分区时，这里会持续显示最终投稿结果。')}</p>
                                  </div>
                                  <span className="rounded-full border border-blue-200 bg-blue-50 px-2.5 py-1 text-[11px] font-semibold text-blue-700">
                                    {t('实时同步')}
                                  </span>
                                </div>
                                <div className="mt-3 space-y-3">
                                    <div className="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3">
                                      <p className="text-xs text-slate-500">{t('稿件类型')}</p>
                                      <p className="mt-1 text-sm font-semibold text-slate-900">{currentSubmissionCopyrightLabel}</p>
                                    </div>

                                  <div className="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3">
                                    <p className="text-xs text-slate-500">{t('一级分区')}</p>
                                    <p className="mt-1 text-sm font-semibold text-slate-900">{selectedSubmissionParentLabel}</p>
                                  </div>

                                  <div className="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3">
                                    <p className="text-xs text-slate-500">{t('二级分区状态')}</p>
                                    <p className="mt-1 text-sm font-semibold text-slate-900">{selectedSubmissionChildLabel}</p>
                                  </div>
                                   <div className="rounded-2xl border border-blue-200 bg-blue-50/70 px-3 py-3">
                                    <p className="text-xs text-blue-700/80">{t('最终投稿分区')}</p>
                                    <p className="mt-1 text-sm font-semibold text-slate-950">{currentSubmissionLabel}</p>
                                  </div>
                                </div>
                              </div>

                              <div className="rounded-2xl border border-slate-200 bg-white/90 p-4 text-[13px] leading-6 text-slate-600">
                                {t('这个配置会直接写入 B 站上传参数。对于有明确内容垂类的账号，建议固定二级分区，并提前统一稿件类型，避免人工上传和自动上传落到不同规则。')}
                              </div>
                            </div>
                          </div>
                          </div>
                        </SurfaceCard>

                        <SurfaceCard
                          title={t('自动上传')}
                          description={t('统一配置自动上传开关和巡检频率，和默认投稿分区一起管理。')}
                        >
                          <div className="space-y-3">
                            <SettingsRow
                              title={t('启用自动上传')}
                              description={t('允许系统按设定时间扫描并自动上传已完成视频')}
                            >
                              <Switch
                                checked={autoUpload}
                                disabled={settingsLoading && !settingsLoaded}
                                onChange={handleAutoUploadChange}
                              />
                            </SettingsRow>

                            <SettingsRow
                              title={t('自动上传间隔')}
                              description={t('系统执行自动上传检查的频率')}
                            >
                              <Select
                                value={autoUploadInterval}
                                disabled={(settingsLoading && !settingsLoaded) || !autoUpload}
                                onValueChange={handleAutoUploadIntervalChange}
                              >
                                <SelectTrigger>
                                  <SelectValue placeholder={t('选择时间间隔')} />
                                </SelectTrigger>
                                <SelectContent>
                                  {AUTO_UPLOAD_INTERVAL_OPTIONS.map((minutes) => (
                                    <SelectItem key={minutes} value={String(minutes)}>
                                      {t('{minutes} 分钟', { minutes })}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </SettingsRow>

                            <div className="grid gap-3 lg:grid-cols-2">
                              <div className="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-[13px] leading-5 text-slate-600">
                                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">{t('执行策略')}</p>
                                <p className="mt-2">{t('开启后，系统会基于当前账户配置自动检查尚未同步到 B 站的已完成视频。')}</p>
                              </div>
                              <div className="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-[13px] leading-5 text-slate-600">
                                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">{t('分区继承')}</p>
                                <p className="mt-2">{t('自动上传任务会继承上方投稿分区和稿件类型，确保自动链路和人工链路使用同一投递规则。')}</p>
                              </div>
                            </div>
                          </div>
                        </SurfaceCard>
                      </SettingsCardContent>
                    </div>
                  )}

                  {activeSection === item.id && item.id === 'template' && (
                    <div className="border-t border-slate-200">
                      <SettingsCardContent>
                        <SectionHint text={t('管理默认输出语言、语气和模板风格')} />
                        <SettingsRow
                          title={t('默认输出语言')}
                          description={t('AI 生成内容时优先采用的语言')}
                        >
                          <Select
                            value={bidDefaultLanguage}
                            disabled={settingsLoading && !settingsLoaded}
                            onValueChange={handleBidDefaultLanguageChange}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('选择语言')} />
                            </SelectTrigger>
                            <SelectContent>
                              {LANGUAGE_OPTIONS.filter((option) => option.value !== 'auto').map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                  {t(option.label)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </SettingsRow>

                        <SettingsRow
                          title={t('默认语气')}
                          description={t('生成文案时采用的表达风格')}
                        >
                          <Select
                            value={bidDefaultTone}
                            disabled={settingsLoading && !settingsLoaded}
                            onValueChange={handleBidDefaultToneChange}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('选择语气')} />
                            </SelectTrigger>
                            <SelectContent>
                              {BID_TONE_OPTIONS.map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                  {t(option.label)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </SettingsRow>

                        <SettingsRow
                          title={t('模板风格')}
                          description={t('结构化输出所采用的默认模板样式')}
                        >
                          <Select
                            value={bidTemplateStyle}
                            disabled={settingsLoading && !settingsLoaded}
                            onValueChange={handleBidTemplateStyleChange}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder={t('选择风格')} />
                            </SelectTrigger>
                            <SelectContent>
                              {BID_TEMPLATE_STYLE_OPTIONS.map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                  {t(option.label)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </SettingsRow>
                      </SettingsCardContent>
                    </div>
                  )}
                </div>
              ))}

              <button
                type="button"
                onClick={logout}
                className="flex w-full items-center gap-3 rounded-2xl bg-red-50 px-4 py-3 text-left text-[14px] font-semibold text-red-500 transition hover:bg-red-100"
              >
                <LogOut className="h-5 w-5" />
                {t('退出登录')}
              </button>
            </div>
          </SettingsCard>
        </div>
      </div>
    </div>
  );
}
