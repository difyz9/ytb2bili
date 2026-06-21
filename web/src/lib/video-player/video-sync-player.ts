import { TimelineAudioPreloader } from '@/lib/video-player/timeline-audio-preloader';
import type { TimelineSubtitle } from '@/lib/video-player/subtitle-timeline';

export interface VideoSyncPlayerConfig {
  videoElement: HTMLVideoElement;
  subtitles: TimelineSubtitle[];
  audioBaseUrl: string;
  volume?: number;
  speedDeviation?: number;
  smoothSpeedTransition?: boolean;
  smoothTransitionDurationMs?: number;
  onSubtitleChange?: (newIndex: number, oldIndex: number) => void;
}

export class VideoSyncPlayer {
  private readonly MIN_SPEED_EPSILON = 0.005;
  private readonly video: HTMLVideoElement;
  private readonly subtitles: TimelineSubtitle[];
  private readonly audioPlayer: HTMLAudioElement;
  private readonly preloader: TimelineAudioPreloader;
  private readonly onSubtitleChange?: (newIndex: number, oldIndex: number) => void;
  private readonly speedDeviation: number;
  private readonly smoothSpeedTransition: boolean;
  private readonly smoothTransitionDurationMs: number;

  private isEnabled = false;
  private currentSubtitleIndex = -1;
  private lastPlayedIndex = -1;
  private originalVolume = 1;
  private originalMuted = false;
  private originalPlaybackRate = 1;
  private isSpeedAdjusted = false;
  private isProgrammaticRateChange = false;
  private audioPlaybackLock = false;
  private audioCompletionTimeout: NodeJS.Timeout | null = null;
  private speedAnimationFrame: number | null = null;
  private speedAnimationStartTs: number | null = null;
  private playRequestId = 0;

  constructor(config: VideoSyncPlayerConfig) {
    this.video = config.videoElement;
    this.subtitles = config.subtitles;
    this.preloader = new TimelineAudioPreloader({ audioBaseUrl: config.audioBaseUrl });
    this.onSubtitleChange = config.onSubtitleChange;
    this.speedDeviation = config.speedDeviation ?? 0.1;
    this.smoothSpeedTransition = config.smoothSpeedTransition ?? true;
    this.smoothTransitionDurationMs = config.smoothTransitionDurationMs ?? 180;

    this.originalVolume = this.video.volume;
    this.originalMuted = this.video.muted;
    this.originalPlaybackRate = this.video.playbackRate;

    this.audioPlayer = new Audio();
    this.audioPlayer.preload = 'auto';
    this.audioPlayer.crossOrigin = 'anonymous';
    this.audioPlayer.volume = config.volume ?? 1;
    this.audioPlayer.playbackRate = this.originalPlaybackRate;

    this.attachEventListeners();
  }

  public start(): void {
    if (this.isEnabled) return;

    this.isEnabled = true;
    this.muteOriginalVideo();
    this.preloader.warmAround(Math.max(0, this.findSubtitleIndex(this.video.currentTime)), this.subtitles.length);

    if (!this.video.paused) {
      this.video.addEventListener('timeupdate', this.handleTimeUpdate);
      void this.syncToCurrentTime(true);
    }
  }

  public stop(): void {
    if (!this.isEnabled) return;

    this.isEnabled = false;
    this.video.removeEventListener('timeupdate', this.handleTimeUpdate);
    this.audioPlayer.pause();
    this.audioPlayer.currentTime = 0;
    this.audioPlaybackLock = false;
    this.restorePlaybackRate();
    this.restoreOriginalVideo();
    this.currentSubtitleIndex = -1;
    this.lastPlayedIndex = -1;
    this.playRequestId += 1;
  }

  public destroy(): void {
    this.stop();
    this.detachEventListeners();
    this.preloader.clear();
    this.cancelSpeedAnimation();
    this.audioPlayer.src = '';
  }

  public setVolume(volume: number): void {
    this.audioPlayer.volume = volume;
  }

  public setMuted(muted: boolean): void {
    this.audioPlayer.muted = muted;
  }

  private attachEventListeners(): void {
    this.video.addEventListener('play', this.handlePlay);
    this.video.addEventListener('pause', this.handlePause);
    this.video.addEventListener('seeked', this.handleSeeked);
    this.video.addEventListener('ended', this.handleEnded);
    this.video.addEventListener('ratechange', this.handleRateChange);
    this.audioPlayer.addEventListener('ended', this.handleAudioEnded);
  }

  private detachEventListeners(): void {
    this.video.removeEventListener('play', this.handlePlay);
    this.video.removeEventListener('pause', this.handlePause);
    this.video.removeEventListener('seeked', this.handleSeeked);
    this.video.removeEventListener('ended', this.handleEnded);
    this.video.removeEventListener('ratechange', this.handleRateChange);
    this.video.removeEventListener('timeupdate', this.handleTimeUpdate);
    this.audioPlayer.removeEventListener('ended', this.handleAudioEnded);
  }

  private handlePlay = () => {
    if (!this.isEnabled) return;
    this.video.addEventListener('timeupdate', this.handleTimeUpdate);
    void this.syncToCurrentTime(true);
  };

  private handlePause = () => {
    this.video.removeEventListener('timeupdate', this.handleTimeUpdate);
    this.audioPlayer.pause();
    this.audioPlaybackLock = false;
    this.restorePlaybackRate();
  };

  private handleSeeked = () => {
    if (!this.isEnabled) return;

    this.currentSubtitleIndex = -1;
    this.lastPlayedIndex = -1;
    this.audioPlayer.pause();
    this.audioPlayer.currentTime = 0;
    this.audioPlaybackLock = false;
    this.restorePlaybackRate();

    if (!this.video.paused) {
      void this.syncToCurrentTime(true);
    }
  };

  private handleEnded = () => {
    this.audioPlayer.pause();
    this.audioPlayer.currentTime = 0;
    this.audioPlaybackLock = false;
    this.restorePlaybackRate();
    this.restoreOriginalVideo();
  };

  private handleRateChange = () => {
    if (this.isProgrammaticRateChange) {
      this.audioPlayer.playbackRate = this.video.playbackRate;
      this.isProgrammaticRateChange = false;
      return;
    }

    if (this.isSpeedAdjusted) {
      this.cancelSpeedAnimation();
      if (this.audioCompletionTimeout) {
        clearTimeout(this.audioCompletionTimeout);
        this.audioCompletionTimeout = null;
      }
      this.isSpeedAdjusted = false;
    }

    this.originalPlaybackRate = this.video.playbackRate;
    this.audioPlayer.playbackRate = this.video.playbackRate;
  };

  private handleAudioEnded = () => {
    this.audioPlaybackLock = false;
    this.restorePlaybackRate();
  };

  private handleTimeUpdate = () => {
    if (this.audioPlaybackLock) return;
    void this.syncToCurrentTime(false);
  };

  private async syncToCurrentTime(forceReplay: boolean): Promise<void> {
    if (!this.isEnabled || this.video.paused) return;

    const subtitleIndex = this.findSubtitleIndex(this.video.currentTime);
    if (subtitleIndex === -1) {
      this.currentSubtitleIndex = -1;
      if (!this.audioPlayer.paused) {
        this.audioPlayer.pause();
        this.audioPlayer.currentTime = 0;
      }
      this.audioPlaybackLock = false;
      this.restorePlaybackRate();
      return;
    }

    if (!forceReplay && subtitleIndex === this.currentSubtitleIndex) return;

    const previousIndex = this.currentSubtitleIndex;
    this.currentSubtitleIndex = subtitleIndex;
    this.onSubtitleChange?.(subtitleIndex, previousIndex);

    if (!forceReplay && subtitleIndex === this.lastPlayedIndex) return;

    this.lastPlayedIndex = subtitleIndex;
    this.preloader.warmAround(subtitleIndex, this.subtitles.length);
    await this.playSubtitleAudio(subtitleIndex);
  }

  private async playSubtitleAudio(index: number): Promise<void> {
    const subtitle = this.subtitles[index];
    if (!subtitle) return;

    const requestId = ++this.playRequestId;
    const meta = await this.preloader.prime(index);
    if (requestId !== this.playRequestId || !this.isEnabled) return;
    if (!meta) {
      this.audioPlaybackLock = false;
      this.restorePlaybackRate();
      return;
    }

    this.audioPlaybackLock = true;

    const subtitleDuration = subtitle.to - subtitle.from;
    const targetSpeed = this.adjustPlaybackRate(meta.duration, subtitleDuration);
    if (requestId !== this.playRequestId || !this.isEnabled) return;

    this.muteOriginalVideo();
    this.audioPlayer.pause();
    this.audioPlayer.src = meta.url;
    this.audioPlayer.currentTime = this.computeAudioStartOffset(subtitle, meta.duration, targetSpeed.audioTarget);
    this.audioPlayer.playbackRate = this.video.playbackRate;

    try {
      await this.audioPlayer.play();
    } catch {
      this.audioPlaybackLock = false;
      this.restorePlaybackRate();
    }
  }

  private adjustPlaybackRate(audioDuration: number | undefined, subtitleDuration: number): { videoTarget: number; audioTarget: number } {
    if (!audioDuration || subtitleDuration <= 0 || audioDuration <= subtitleDuration) {
      this.restorePlaybackRate();
      return {
        videoTarget: this.originalPlaybackRate,
        audioTarget: this.originalPlaybackRate,
      };
    }

    const baseRate = this.originalPlaybackRate;
    const minAllowed = Math.max(0.1, baseRate - this.speedDeviation);
    const maxAllowed = baseRate + this.speedDeviation;
    const idealUnifiedSpeed = baseRate * (subtitleDuration / audioDuration);

    let targetVideo = baseRate;
    let targetAudio = baseRate;

    if (idealUnifiedSpeed >= minAllowed) {
      targetVideo = idealUnifiedSpeed;
      targetAudio = idealUnifiedSpeed;
    } else {
      targetVideo = minAllowed;
      const requiredAudioSpeed = (audioDuration * targetVideo) / subtitleDuration;

      if (requiredAudioSpeed <= maxAllowed) {
        targetAudio = Math.max(requiredAudioSpeed, minAllowed);
      } else {
        targetAudio = maxAllowed;
      }
    }

    const needsSpeedUpdate =
      Math.abs(targetVideo - this.video.playbackRate) > this.MIN_SPEED_EPSILON ||
      Math.abs(targetAudio - this.audioPlayer.playbackRate) > this.MIN_SPEED_EPSILON;

    if (needsSpeedUpdate) {
      this.isSpeedAdjusted =
        Math.abs(targetVideo - baseRate) > this.MIN_SPEED_EPSILON ||
        Math.abs(targetAudio - baseRate) > this.MIN_SPEED_EPSILON;
      this.applySpeed(targetVideo, targetAudio);
    }

    if (this.audioCompletionTimeout) {
      clearTimeout(this.audioCompletionTimeout);
    }

    this.audioCompletionTimeout = setTimeout(() => {
      this.restorePlaybackRate();
    }, Math.min((audioDuration * 1000) / targetAudio + 250, 60000));

    return {
      videoTarget: targetVideo,
      audioTarget: targetAudio,
    };
  }

  private restorePlaybackRate(): void {
    this.cancelSpeedAnimation();

    if (this.audioCompletionTimeout) {
      clearTimeout(this.audioCompletionTimeout);
      this.audioCompletionTimeout = null;
    }

    if (!this.isSpeedAdjusted) return;

    this.isProgrammaticRateChange = true;
    this.video.playbackRate = this.originalPlaybackRate;
    this.audioPlayer.playbackRate = this.originalPlaybackRate;
    this.isSpeedAdjusted = false;
  }

  private applySpeed(targetVideo: number, targetAudio: number): void {
    if (!this.smoothSpeedTransition) {
      this.cancelSpeedAnimation();
      this.isProgrammaticRateChange = true;
      this.video.playbackRate = targetVideo;
      this.audioPlayer.playbackRate = targetAudio;
      return;
    }

    this.cancelSpeedAnimation();

    const fromVideo = this.video.playbackRate;
    const fromAudio = this.audioPlayer.playbackRate;
    if (
      Math.abs(fromVideo - targetVideo) < this.MIN_SPEED_EPSILON &&
      Math.abs(fromAudio - targetAudio) < this.MIN_SPEED_EPSILON
    ) {
      return;
    }

    const duration = Math.max(16, this.smoothTransitionDurationMs);
    const animate = (timestamp: number) => {
      if (this.speedAnimationStartTs == null) {
        this.speedAnimationStartTs = timestamp;
      }

      const elapsed = timestamp - this.speedAnimationStartTs;
      let progress = Math.min(1, elapsed / duration);
      progress = progress < 0.5
        ? 4 * progress * progress * progress
        : 1 - Math.pow(-2 * progress + 2, 3) / 2;

      const currentVideo = fromVideo + (targetVideo - fromVideo) * progress;
      const currentAudio = fromAudio + (targetAudio - fromAudio) * progress;
      this.isProgrammaticRateChange = true;
      this.video.playbackRate = currentVideo;
      this.audioPlayer.playbackRate = currentAudio;

      if (elapsed < duration) {
        this.speedAnimationFrame = requestAnimationFrame(animate);
        return;
      }

      this.speedAnimationFrame = null;
      this.speedAnimationStartTs = null;
    };

    this.speedAnimationFrame = requestAnimationFrame(animate);
  }

  private cancelSpeedAnimation(): void {
    if (this.speedAnimationFrame != null) {
      cancelAnimationFrame(this.speedAnimationFrame);
      this.speedAnimationFrame = null;
    }
    this.speedAnimationStartTs = null;
  }

  private findSubtitleIndex(currentTime: number): number {
    if (this.subtitles.length < 20) {
      for (let index = 0; index < this.subtitles.length; index += 1) {
        const subtitle = this.subtitles[index];
        if (currentTime >= subtitle.from && currentTime <= subtitle.to) {
          return index;
        }
      }

      return -1;
    }

    let low = 0;
    let high = this.subtitles.length - 1;

    while (low <= high) {
      const middle = Math.floor((low + high) / 2);
      const subtitle = this.subtitles[middle];

      if (currentTime < subtitle.from) {
        high = middle - 1;
      } else if (currentTime > subtitle.to) {
        low = middle + 1;
      } else {
        return middle;
      }
    }

    return -1;
  }

  private computeAudioStartOffset(
    subtitle: TimelineSubtitle,
    audioDuration: number | undefined,
    audioPlaybackRate: number,
  ): number {
    if (!audioDuration || subtitle.to <= subtitle.from) {
      return 0;
    }

    const elapsedInSubtitle = Math.max(0, this.video.currentTime - subtitle.from);
    if (elapsedInSubtitle <= 0.05) {
      return 0;
    }

    const subtitleDuration = subtitle.to - subtitle.from;
    const progress = Math.min(1, elapsedInSubtitle / subtitleDuration);
    const adjustedAudioDuration = audioDuration / Math.max(0.1, audioPlaybackRate);
    const offset = Math.min(audioDuration - 0.05, adjustedAudioDuration * progress * audioPlaybackRate);
    return Number.isFinite(offset) && offset > 0 ? offset : 0;
  }

  private muteOriginalVideo(): void {
    this.video.muted = true;
    this.video.volume = 0;
  }

  private restoreOriginalVideo(): void {
    this.video.volume = this.originalVolume;
    this.video.muted = this.originalMuted;
  }
}