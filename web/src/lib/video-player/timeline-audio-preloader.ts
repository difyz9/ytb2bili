export interface CachedAudioMeta {
  url: string;
  duration?: number;
}

interface CacheEntry {
  meta: CachedAudioMeta | null;
  promise: Promise<CachedAudioMeta | null> | null;
  touchedAt: number;
}

interface TimelineAudioPreloaderConfig {
  audioBaseUrl: string;
  preloadAhead?: number;
  maxCacheSize?: number;
}

export class TimelineAudioPreloader {
  private readonly audioBaseUrl: string;
  private readonly preloadAhead: number;
  private readonly maxCacheSize: number;
  private readonly cache = new Map<number, CacheEntry>();

  constructor(config: TimelineAudioPreloaderConfig) {
    this.audioBaseUrl = config.audioBaseUrl.replace(/\/$/, '');
    this.preloadAhead = config.preloadAhead ?? 3;
    this.maxCacheSize = config.maxCacheSize ?? 24;
  }

  public buildAudioUrl(index: number): string {
    return `${this.audioBaseUrl}/index_${String(index).padStart(4, '0')}.mp3`;
  }

  public async prime(index: number): Promise<CachedAudioMeta | null> {
    const cachedEntry = this.cache.get(index);
    if (cachedEntry?.meta !== undefined) {
      cachedEntry.touchedAt = Date.now();
      return cachedEntry.meta;
    }
    if (cachedEntry?.promise) {
      cachedEntry.touchedAt = Date.now();
      return cachedEntry.promise;
    }

    const promise = new Promise<CachedAudioMeta | null>((resolve) => {
      const url = this.buildAudioUrl(index);
      const audio = new Audio();
      audio.preload = 'metadata';
      audio.src = url;

      const cleanup = () => {
        audio.removeEventListener('loadedmetadata', handleLoaded);
        audio.removeEventListener('error', handleError);
      };

      const handleLoaded = () => {
        cleanup();
        resolve({
          url,
          duration: Number.isFinite(audio.duration) ? audio.duration : undefined,
        });
      };

      const handleError = () => {
        cleanup();
        resolve(null);
      };

      audio.addEventListener('loadedmetadata', handleLoaded, { once: true });
      audio.addEventListener('error', handleError, { once: true });
      audio.load();
    }).then((meta) => {
      this.cache.set(index, {
        meta,
        promise: null,
        touchedAt: Date.now(),
      });
      this.trimCache();
      return meta;
    });

    this.cache.set(index, {
      meta: undefined as never,
      promise,
      touchedAt: Date.now(),
    });

    return promise;
  }

  public warmAround(index: number, total: number): void {
    const end = Math.min(total - 1, index + this.preloadAhead);
    for (let cursor = index; cursor <= end; cursor += 1) {
      void this.prime(cursor);
    }
  }

  public clear(): void {
    this.cache.clear();
  }

  private trimCache(): void {
    if (this.cache.size <= this.maxCacheSize) return;

    const entries = Array.from(this.cache.entries()).sort((left, right) => left[1].touchedAt - right[1].touchedAt);
    const removeCount = entries.length - this.maxCacheSize;
    for (let index = 0; index < removeCount; index += 1) {
      this.cache.delete(entries[index][0]);
    }
  }
}