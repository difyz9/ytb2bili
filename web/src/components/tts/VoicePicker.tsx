'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, Check, Search, Play, Square, Loader2 } from 'lucide-react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Skeleton } from '@/components/ui/Skeleton';
import { useI18n } from '@/contexts/I18nContext';
import { buildBackendUrl } from '@/lib/backend-url';
import { getValidEmailAccessToken } from '@/lib/email-auth';
import { cn } from '@/lib/utils';
import {
  filterTTSVoices,
  findTTSVoiceByShortName,
  formatTTSVoiceLabel,
  loadTTSVoiceCatalog,
  type TTSVoiceRecord,
} from '@/lib/tts-voice-catalog';

export interface VoicePickerProps {
  value: string;
  onChange: (value: string) => void;
  onSelectVoice?: (voice: TTSVoiceRecord) => void;
  disabled?: boolean;
  provider?: string;
  preferredLocale?: string;
  placeholder?: string;
  className?: string;
}

type ProviderChoice = 'auto' | 'azure' | 'tencent';

function normalizeProviderChoice(value: string | undefined): ProviderChoice {
  const provider = String(value ?? '').trim().toLowerCase();
  if (provider === 'azure') return 'azure';
  if (provider === 'tencent') return 'tencent';
  return 'auto';
}

function getBrowserPreferredLocale(): string {
  if (typeof navigator === 'undefined') {
    return '';
  }

  const candidates = Array.isArray(navigator.languages) ? navigator.languages : [navigator.language];
  return candidates.find((candidate) => typeof candidate === 'string' && candidate.trim())?.trim() ?? '';
}

async function authFetch(url: string, init: RequestInit = {}): Promise<Response> {
  const token = await getValidEmailAccessToken();
  const headers: Record<string, string> = {
    ...(init.headers as Record<string, string>),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return fetch(url, {
    ...init,
    headers,
    credentials: 'include',
  });
}

export function VoicePicker({
  value,
  onChange,
  onSelectVoice,
  disabled,
  provider,
  preferredLocale,
  placeholder = 'Select voice',
  className,
}: VoicePickerProps) {
  const { t } = useI18n();
  const resolvedPlaceholder = placeholder === 'Select voice' ? t('Select voice') : placeholder;
  const [open, setOpen] = useState(false);
  const [catalog, setCatalog] = useState<TTSVoiceRecord[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [providerChoice, setProviderChoice] = useState<ProviderChoice>(() => normalizeProviderChoice(provider));
  const [localeChoice, setLocaleChoice] = useState<string>('auto');
  const [previewLoadingKey, setPreviewLoadingKey] = useState<string | null>(null);
  const [previewPlayingKey, setPreviewPlayingKey] = useState<string | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const queryRef = useRef<HTMLInputElement | null>(null);
  const loadIdRef = useRef(0);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const previewUrlRef = useRef<string | null>(null);
  const previewAbortRef = useRef<AbortController | null>(null);
  const fallbackPreferredLocale = useMemo(() => getBrowserPreferredLocale(), []);
  const effectivePreferredLocale = preferredLocale?.trim() || fallbackPreferredLocale;

  const selectedVoice = useMemo(() => findTTSVoiceByShortName(catalog, value), [catalog, value]);
  const buttonLabel = selectedVoice ? formatTTSVoiceLabel(selectedVoice) : (value.trim() || resolvedPlaceholder);
  const buttonSubLabel = selectedVoice ? selectedVoice.shortName : (value.trim() ? value.trim() : '');

  const buttonMeta = useMemo(() => {
    if (!selectedVoice) return '';
    const parts = [
      selectedVoice.localeName || selectedVoice.locale,
      selectedVoice.gender,
      selectedVoice.voiceType,
      selectedVoice.sampleRateHertz ? `${selectedVoice.sampleRateHertz} Hz` : undefined,
      selectedVoice.status,
    ].filter(Boolean);
    return parts.join(' · ');
  }, [selectedVoice]);

  const results = useMemo(() => {
    if (!catalog) return [];
    return filterTTSVoices(catalog, {
      query,
      provider: providerChoice,
      preferredLocale: effectivePreferredLocale,
      locale: localeChoice,
      limit: 160,
    });
  }, [catalog, query, providerChoice, effectivePreferredLocale, localeChoice]);

  const localeOptions = useMemo(() => {
    if (!catalog) return [] as Array<{ locale: string; label: string }>;

    const map = new Map<string, string>();
    for (const voice of catalog) {
      if (providerChoice !== 'auto' && voice.provider !== providerChoice) continue;
      if (!voice.locale) continue;
      map.set(voice.locale, voice.localeName || voice.locale);
    }

    const preferred = effectivePreferredLocale.trim();
    const entries = Array.from(map.entries()).map(([locale, label]) => ({ locale, label }));
    entries.sort((a, b) => {
      if (preferred) {
        if (a.locale === preferred) return -1;
        if (b.locale === preferred) return 1;
      }

      const labelDiff = a.label.localeCompare(b.label, effectivePreferredLocale || undefined);
      if (labelDiff !== 0) return labelDiff;
      return a.locale.localeCompare(b.locale);
    });
    return entries;
  }, [catalog, providerChoice, effectivePreferredLocale]);

  useEffect(() => {
    if (!open) return;

    const timer = window.setTimeout(() => queryRef.current?.focus(), 50);
    return () => window.clearTimeout(timer);
  }, [open]);

  useEffect(() => {
    if (!open) {
      loadIdRef.current += 1;
      setLoading(false);
      return;
    }

    if (catalog) return;

    const loadId = loadIdRef.current + 1;
    loadIdRef.current = loadId;

    setLoadError(null);
    setLoading(true);

    void loadTTSVoiceCatalog()
      .then((voices) => {
        if (loadIdRef.current != loadId) return;
        setCatalog(voices);
      })
      .catch((err: unknown) => {
        if (loadIdRef.current != loadId) return;
        setLoadError(err instanceof Error ? err.message : t('Voice list failed to load'));
      })
      .finally(() => {
        if (loadIdRef.current != loadId) return;
        setLoading(false);
      });

    return () => {
      loadIdRef.current += 1;
      setLoading(false);
    };
  }, [open, catalog, t]);

  useEffect(() => {
    if (!open) return;
    setProviderChoice(normalizeProviderChoice(provider));
    setLocaleChoice('auto');
    setQuery('');
  }, [open, provider]);

  useEffect(() => {
    if (!open) return;
    if (localeChoice === 'auto') return;
    if (localeOptions.some((item) => item.locale === localeChoice)) return;
    setLocaleChoice('auto');
  }, [open, localeChoice, localeOptions]);

  const hintText = useMemo(() => {
    const providerText = providerChoice !== 'auto'
      ? t('Only show {provider} voices', { provider: providerChoice.toUpperCase() })
      : t('Includes Azure / Tencent voices');
    const localeText = localeChoice !== 'auto' ? t('Filter language: {locale}', { locale: localeChoice }) : '';
    const preferredText = effectivePreferredLocale ? t('Priority locale: {locale}', { locale: effectivePreferredLocale }) : '';
    return [providerText, localeText, preferredText].filter(Boolean).join(' · ');
  }, [providerChoice, localeChoice, effectivePreferredLocale, t]);

  const stopPreview = useCallback(() => {
    if (previewAbortRef.current) {
      previewAbortRef.current.abort();
      previewAbortRef.current = null;
    }

    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.currentTime = 0;
      audioRef.current.onended = null;
      audioRef.current.onerror = null;
      audioRef.current.src = '';
    }

    if (previewUrlRef.current) {
      URL.revokeObjectURL(previewUrlRef.current);
      previewUrlRef.current = null;
    }

    setPreviewLoadingKey(null);
    setPreviewPlayingKey(null);
  }, []);

  useEffect(() => {
    if (open) return;
    stopPreview();
    setPreviewError(null);
  }, [open, stopPreview]);

  useEffect(() => () => stopPreview(), [stopPreview]);

  const handlePreview = useCallback(async (voice: TTSVoiceRecord) => {
    const previewKey = `${voice.provider}:${voice.shortName}`;
    if (previewPlayingKey === previewKey) {
      stopPreview();
      return;
    }

    stopPreview();
    setPreviewError(null);
    setPreviewLoadingKey(previewKey);

    const controller = new AbortController();
    previewAbortRef.current = controller;

    try {
      const response = await authFetch(buildBackendUrl('/api/v1/tts/preview'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          provider: voice.provider,
          voice_name: voice.shortName,
          language: voice.locale,
          format: 'mp3',
        }),
        signal: controller.signal,
      });

      if (!response.ok) {
        const payload = await response.json().catch(() => null) as { message?: string; error?: string } | null;
        throw new Error(payload?.message || payload?.error || t('Preview request failed ({status})', { status: response.status }));
      }

      const audioBlob = await response.blob();
      if (controller.signal.aborted) return;

      const objectUrl = URL.createObjectURL(audioBlob);
      previewUrlRef.current = objectUrl;

      const audio = audioRef.current ?? new Audio();
      audioRef.current = audio;
      audio.src = objectUrl;
      audio.currentTime = 0;
      audio.onended = () => {
        if (previewUrlRef.current === objectUrl) {
          URL.revokeObjectURL(objectUrl);
          previewUrlRef.current = null;
        }
        setPreviewLoadingKey(null);
        setPreviewPlayingKey((current) => (current === previewKey ? null : current));
      };
      audio.onerror = () => {
        if (previewUrlRef.current === objectUrl) {
          URL.revokeObjectURL(objectUrl);
          previewUrlRef.current = null;
        }
        setPreviewLoadingKey(null);
        setPreviewPlayingKey(null);
        setPreviewError(t('Preview playback failed'));
      };

      await audio.play();
      if (controller.signal.aborted) return;

      setPreviewLoadingKey(null);
      setPreviewPlayingKey(previewKey);
    } catch (err: unknown) {
      if (controller.signal.aborted) return;
      setPreviewLoadingKey(null);
      setPreviewPlayingKey(null);
      setPreviewError(err instanceof Error ? err.message : t('Preview failed'));
    } finally {
      if (previewAbortRef.current === controller) {
        previewAbortRef.current = null;
      }
    }
  }, [previewPlayingKey, stopPreview, t]);

  return (
    <>
      <Button
        type="button"
        variant="outline"
        disabled={disabled}
        onClick={() => setOpen(true)}
        className={cn('h-auto w-full justify-between gap-3 px-3 py-2 text-left', className)}
      >
        <div className="min-w-0">
          <div className="truncate text-sm font-medium text-foreground">{buttonLabel}</div>
          {buttonSubLabel ? (
            <div className="space-y-0.5">
              <div className="truncate text-xs text-muted-foreground">{buttonSubLabel}</div>
              {buttonMeta ? (
                <div className="truncate text-[11px] text-muted-foreground">{buttonMeta}</div>
              ) : null}
            </div>
          ) : (
            <div className="truncate text-xs text-muted-foreground">{t('Supports search: name / shortName / locale / gender')}</div>
          )}
        </div>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
      </Button>

      <Modal
        isOpen={open}
        onClose={() => setOpen(false)}
        title={t('Choose subtitle voice')}
        description={hintText}
        className="max-w-3xl"
      >
        <div className="space-y-3">
          <div className="grid gap-2 sm:grid-cols-2">
            <label className="space-y-1">
              <div className="text-xs font-medium text-muted-foreground">{t('Provider')}</div>
              <select
                value={providerChoice}
                onChange={(event) => setProviderChoice(event.target.value as ProviderChoice)}
                className={cn(
                  'flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground shadow-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
                )}
              >
                <option value="auto">{t('All (Azure + Tencent)')}</option>
                <option value="azure">Azure</option>
                <option value="tencent">Tencent</option>
              </select>
            </label>

            <label className="space-y-1">
              <div className="text-xs font-medium text-muted-foreground">{t('Language / Region')}</div>
              <select
                value={localeChoice}
                onChange={(event) => setLocaleChoice(event.target.value)}
                disabled={!catalog || loading}
                className={cn(
                  'flex h-10 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground shadow-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
                )}
              >
                <option value="auto">{t('All languages')}</option>
                {localeOptions.map((item) => (
                  <option key={item.locale} value={item.locale}>
                    {item.label} ({item.locale})
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                ref={queryRef}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t('Search voices: name / shortName / locale / gender')}
                className="pl-9"
              />
            </div>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setQuery('')}
              disabled={!query}
            >
              {t('Clear')}
            </Button>
          </div>

          {loadError && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {loadError}
            </div>
          )}

          {previewError && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700">
              {previewError}
            </div>
          )}

          <div className="max-h-[55vh] overflow-y-auto rounded-xl border border-border">
            {loading && (
              <div className="space-y-2 p-3">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            )}

            {!loading && catalog && results.length === 0 && (
              <div className="space-y-2 p-4 text-sm">
                <div className="text-foreground">{t('No matching voices.')}</div>
                <div className="text-muted-foreground">{t('You can still input a shortName directly.')}</div>
                <div className="flex gap-2">
                  <Input
                    value={value}
                    onChange={(event) => onChange(event.target.value)}
                    placeholder="例如 zh-CN-XiaoxiaoNeural"
                  />
                  <Button type="button" onClick={() => setOpen(false)}>
                    {t('Use')}
                  </Button>
                </div>
              </div>
            )}

            {!loading && results.length > 0 && (
              <div className="divide-y divide-border">
                {results.map((voice) => {
                  const isSelected = voice.shortName === value;
                  const previewKey = `${voice.provider}:${voice.shortName}`;
                  const isPreviewLoading = previewLoadingKey === previewKey;
                  const isPreviewPlaying = previewPlayingKey === previewKey;
                  const primary = formatTTSVoiceLabel(voice);
                  const secondaryParts = [
                    voice.localeName || voice.locale,
                    voice.secondaryLocales && voice.secondaryLocales.length > 0
                      ? `alt: ${voice.secondaryLocales.slice(0, 2).join(',')}${voice.secondaryLocales.length > 2 ? '…' : ''}`
                      : undefined,
                    voice.gender,
                    voice.voiceType,
                    voice.sampleRateHertz ? `${voice.sampleRateHertz} Hz` : undefined,
                    voice.status,
                    voice.supportedLanguages && voice.supportedLanguages.length > 0
                      ? `langs: ${voice.supportedLanguages.slice(0, 3).join(',')}${voice.supportedLanguages.length > 3 ? '…' : ''}`
                      : undefined,
                    voice.styles && voice.styles.length > 0 ? `styles: ${voice.styles.slice(0, 2).join(',')}${voice.styles.length > 2 ? '…' : ''}` : undefined,
                    voice.recommendedScene,
                  ].filter(Boolean);

                  return (
                    <div
                      key={previewKey}
                      className={cn(
                        'flex items-start gap-2 px-3 py-3 transition-colors hover:bg-accent',
                        isSelected && 'bg-accent'
                      )}
                    >
                      <button
                        type="button"
                        onClick={() => {
                          onChange(voice.shortName);
                          onSelectVoice?.(voice);
                          setOpen(false);
                        }}
                        className="flex min-w-0 flex-1 items-start gap-3 text-left"
                      >
                        <div className="mt-0.5 flex h-5 w-5 items-center justify-center">
                          {isSelected ? (
                            <Check className="h-4 w-4 text-primary" />
                          ) : (
                            <span className="h-2 w-2 rounded-full bg-muted" />
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                            <div className="truncate text-sm font-medium text-foreground">{primary}</div>
                            <div className="truncate text-xs text-muted-foreground">{voice.shortName}</div>
                            <span className="rounded-md border border-border bg-background px-2 py-0.5 text-[11px] text-muted-foreground">
                              {voice.provider.toUpperCase()}
                            </span>
                          </div>
                          {secondaryParts.length > 0 && (
                            <div className="mt-1 truncate text-xs text-muted-foreground">
                              {secondaryParts.join(' · ')}
                            </div>
                          )}
                        </div>
                      </button>

                      <Button
                        type="button"
                        variant={isPreviewPlaying ? 'secondary' : 'outline'}
                        className="shrink-0"
                        onClick={() => void handlePreview(voice)}
                        disabled={Boolean(previewLoadingKey && !isPreviewLoading)}
                      >
                        {isPreviewLoading ? (
                          <>
                            <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                            {t('Loading')}
                          </>
                        ) : isPreviewPlaying ? (
                          <>
                            <Square className="mr-1 h-4 w-4" />
                            {t('Stop')}
                          </>
                        ) : (
                          <>
                            <Play className="mr-1 h-4 w-4" />
                            {t('Preview')}
                          </>
                        )}
                      </Button>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              {t('Close')}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
