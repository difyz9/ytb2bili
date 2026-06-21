export type TTSVoiceProvider = 'azure' | 'tencent';

export interface TTSVoiceRecord {
  provider: TTSVoiceProvider;
  shortName: string;
  displayName: string;
  localName: string;
  locale: string;
  localeName: string;
  gender?: string;
  voiceType?: string;
  sampleRateHertz?: string;
  status?: string;
  styles?: string[];
  recommendedScene?: string;
  secondaryLocales?: string[];
  supportedLanguages?: string[];
}

type VoiceSnapshotPayload = {
  data?: {
    provider?: string;
    voices?: unknown;
  };
};

type VoiceCatalogApiPayload = {
  data?: {
    voices?: unknown;
  };
};

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string');
}

function normalizeProvider(value: unknown): TTSVoiceProvider | null {
  const provider = String(value ?? '').trim().toLowerCase();
  if (provider === 'azure') return 'azure';
  if (provider === 'tencent') return 'tencent';
  return null;
}

function coerceVoiceRecord(provider: TTSVoiceProvider, value: unknown): TTSVoiceRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }

  const record = value as Record<string, unknown>;
  const shortName = String(record.shortName ?? record.short_name ?? '').trim();
  if (!shortName) {
    return null;
  }

  const displayName = String(record.displayName ?? record.display_name ?? shortName).trim() || shortName;
  const localName = String(record.localName ?? record.local_name ?? displayName).trim() || displayName;
  const locale = String(record.locale ?? '').trim();
  const localeName = String(record.localeName ?? record.locale_name ?? locale).trim() || locale;

  const gender = String(record.gender ?? '').trim();
  const voiceType = String(record.voiceType ?? record.voice_type ?? '').trim();
  const sampleRateHertz = String(record.sampleRateHertz ?? record.sample_rate_hertz ?? '').trim();
  const status = String(record.status ?? '').trim();
  const styles = isStringArray(record.styles) ? record.styles : undefined;
  const recommendedScene = String(record.recommendedScene ?? '').trim();
  const secondaryLocales = isStringArray(record.secondaryLocales)
    ? record.secondaryLocales
    : isStringArray(record.secondary_locales)
      ? record.secondary_locales
      : undefined;

  const supportedLanguages = isStringArray(record.supportedLanguages)
    ? record.supportedLanguages
    : undefined;

  return {
    provider,
    shortName,
    displayName,
    localName,
    locale,
    localeName,
    gender: gender || undefined,
    voiceType: voiceType || undefined,
    sampleRateHertz: sampleRateHertz || undefined,
    status: status || undefined,
    styles,
    recommendedScene: recommendedScene || undefined,
    secondaryLocales,
    supportedLanguages,
  };
}

async function fetchVoiceSnapshot(path: string): Promise<TTSVoiceRecord[]> {
  const response = await fetch(path, { credentials: 'same-origin' });
  if (!response.ok) {
    throw new Error(`voice snapshot fetch failed: ${response.status}`);
  }

  const payload = (await response.json().catch(() => null)) as VoiceSnapshotPayload | null;
  const provider = normalizeProvider(payload?.data?.provider);
  if (!provider) {
    return [];
  }

  const voices = payload?.data?.voices;
  if (!Array.isArray(voices)) {
    return [];
  }

  return voices
    .map((voice) => coerceVoiceRecord(provider, voice))
    .filter((voice): voice is TTSVoiceRecord => Boolean(voice));
}

async function fetchVoiceCatalogFromAPI(path: string): Promise<TTSVoiceRecord[]> {
  const response = await fetch(path, { credentials: 'same-origin' });
  if (!response.ok) {
    throw new Error(`voice catalog api fetch failed: ${response.status}`);
  }

  const payload = (await response.json().catch(() => null)) as VoiceCatalogApiPayload | null;
  const voices = payload?.data?.voices;
  if (!Array.isArray(voices)) {
    return [];
  }

  return voices
    .map((voice) => {
      const provider = normalizeProvider((voice as Record<string, unknown> | null)?.provider);
      if (!provider) return null;
      return coerceVoiceRecord(provider, voice);
    })
    .filter((voice): voice is TTSVoiceRecord => Boolean(voice));
}

let cachedCatalogPromise: Promise<TTSVoiceRecord[]> | null = null;

export async function loadTTSVoiceCatalog(): Promise<TTSVoiceRecord[]> {
  if (cachedCatalogPromise) {
    return cachedCatalogPromise;
  }

  cachedCatalogPromise = (async () => {
    const apiVoices = await fetchVoiceCatalogFromAPI('/api/v1/tts/voices').catch(() => null);

    const merged = apiVoices ?? (await (async () => {
      const [azure, tencent] = await Promise.all([
        fetchVoiceSnapshot('/tts/azure_voices.json').catch(() => []),
        fetchVoiceSnapshot('/tts/tencent_voices.json').catch(() => []),
      ]);

      return [...azure, ...tencent];
    })());

    const seen = new Set<string>();
    return merged.filter((voice) => {
      const key = `${voice.provider}:${voice.shortName}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  })();

  return cachedCatalogPromise;
}

function normalizeQuery(value: string): string {
  return value.trim().toLowerCase();
}

function normalizeLocalePrefix(value: string): string {
  return value.trim().toLowerCase();
}

function matchesLocaleOrAlias(value: string | undefined, preferredLocale: string): number {
  const locale = normalizeLocalePrefix(value ?? '');
  if (!locale || !preferredLocale) return 0;

  if (locale === preferredLocale) return 3;
  if (locale.startsWith(preferredLocale)) return 2;

  const base = preferredLocale.split('-')[0];
  if (base && locale.startsWith(base)) return 1;
  return 0;
}

function includesNormalized(haystack: string, needle: string): boolean {
  if (!needle) return true;
  return normalizeQuery(haystack).includes(needle);
}

export function findTTSVoiceByShortName(
  voices: TTSVoiceRecord[] | null | undefined,
  shortName: string,
): TTSVoiceRecord | null {
  if (!voices) return null;
  const needle = shortName.trim();
  if (!needle) return null;
  return voices.find((voice) => voice.shortName === needle) ?? null;
}

export function filterTTSVoices(
  voices: TTSVoiceRecord[],
  options: {
    query?: string;
    provider?: string;
    preferredLocale?: string;
    locale?: string;
    limit?: number;
  } = {},
): TTSVoiceRecord[] {
  const query = normalizeQuery(options.query ?? '');
  const preferredLocale = normalizeLocalePrefix(options.preferredLocale ?? '');
  const provider = String(options.provider ?? '').trim().toLowerCase();
  const localeFilter = String(options.locale ?? '').trim();
  const limit = typeof options.limit === 'number' && options.limit > 0 ? Math.floor(options.limit) : 120;

  const matchesProvider = (voice: TTSVoiceRecord) => {
    if (!provider || provider === 'auto') return true;
    return voice.provider === provider;
  };

  const matchesLocale = (voice: TTSVoiceRecord) => {
    if (!localeFilter || localeFilter === 'auto') return true;
    return voice.locale === localeFilter || (voice.secondaryLocales ?? []).includes(localeFilter);
  };

  const matchesQuery = (voice: TTSVoiceRecord) => {
    if (!query) return true;

    return (
      includesNormalized(voice.shortName, query)
      || includesNormalized(voice.displayName, query)
      || includesNormalized(voice.localName, query)
      || includesNormalized(voice.locale, query)
      || includesNormalized(voice.localeName, query)
      || includesNormalized((voice.secondaryLocales ?? []).join(' '), query)
      || includesNormalized(voice.gender ?? '', query)
      || includesNormalized(voice.voiceType ?? '', query)
      || includesNormalized(voice.sampleRateHertz ?? '', query)
      || includesNormalized(voice.status ?? '', query)
      || includesNormalized((voice.styles ?? []).join(' '), query)
      || includesNormalized(voice.recommendedScene ?? '', query)
      || includesNormalized((voice.supportedLanguages ?? []).join(' '), query)
    );
  };

  const filtered = voices.filter((voice) => matchesProvider(voice) && matchesLocale(voice) && matchesQuery(voice));

  const scoreLocale = (voice: TTSVoiceRecord) => {
    if (!preferredLocale) return 0;
    return Math.max(
      matchesLocaleOrAlias(voice.locale, preferredLocale),
      ...(voice.secondaryLocales ?? []).map((locale) => matchesLocaleOrAlias(locale, preferredLocale)),
      ...(voice.supportedLanguages ?? []).map((locale) => matchesLocaleOrAlias(locale, preferredLocale)),
    );
  };

  filtered.sort((a, b) => {
    const localeScoreDiff = scoreLocale(b) - scoreLocale(a);
    if (localeScoreDiff !== 0) return localeScoreDiff;

    if (a.provider !== b.provider) {
      return a.provider.localeCompare(b.provider);
    }

    const localeDiff = a.locale.localeCompare(b.locale);
    if (localeDiff !== 0) return localeDiff;

    const nameA = (a.localName || a.displayName || a.shortName).toLowerCase();
    const nameB = (b.localName || b.displayName || b.shortName).toLowerCase();
    return nameA.localeCompare(nameB);
  });

  return filtered.slice(0, limit);
}

export function formatTTSVoiceLabel(voice: TTSVoiceRecord): string {
  return voice.localName || voice.displayName || voice.shortName;
}
