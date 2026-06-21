'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { X, Play, Pause, Volume2, VolumeX, Maximize, Settings, SkipBack, SkipForward, Subtitles, AudioLines } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';
import { parseSubtitleTimeline, subtitleTextToVttUrl } from '@/lib/video-player/subtitle-timeline';
import { VideoSyncPlayer } from '@/lib/video-player/video-sync-player';

interface VideoPlayerModalProps {
  isOpen: boolean;
  onClose: () => void;
  videoUrl: string;
  title: string;
  description?: string;
  /** 可选字幕文件 URL（SRT 或 WebVTT 均支持） */
  subtitleUrl?: string;
  /** 可选字幕配音目录，例如 /static/<video_id>/audio */
  audioBaseUrl?: string;
}

export default function VideoPlayerModal({ isOpen, onClose, videoUrl, title, description, subtitleUrl, audioBaseUrl }: VideoPlayerModalProps) {
  const { t } = useI18n();
  const videoRef = useRef<HTMLVideoElement>(null);
  const syncPlayerRef = useRef<VideoSyncPlayer | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [isMuted, setIsMuted] = useState(false);
  const [volume, setVolume] = useState(1);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [showControls, setShowControls] = useState(true);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [showSettings, setShowSettings] = useState(false);
  const [subtitleEnabled, setSubtitleEnabled] = useState(true);
  const [vttUrl, setVttUrl] = useState<string | null>(null);
  const [subtitleTimeline, setSubtitleTimeline] = useState(() => parseSubtitleTimeline(''));
  const [dubbedAudioEnabled, setDubbedAudioEnabled] = useState(false);
  const controlsTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // 加载并转换字幕
  useEffect(() => {
    const controller = new AbortController();
    let blobUrl: string | null = null;
    if (isOpen && subtitleUrl) {
      fetch(subtitleUrl, { signal: controller.signal })
        .then(r => r.ok ? r.text() : Promise.reject())
        .then(text => {
          const timeline = parseSubtitleTimeline(text);
          blobUrl = subtitleTextToVttUrl(text);
          setVttUrl(blobUrl);
          setSubtitleTimeline(timeline);
        })
        .catch(() => {
          setVttUrl(null);
          setSubtitleTimeline([]);
        });
    } else {
      setVttUrl(null);
      setSubtitleTimeline([]);
    }
    return () => {
      controller.abort();
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
  }, [isOpen, subtitleUrl]);

  useEffect(() => {
    if (isOpen) {
      setDubbedAudioEnabled(Boolean(subtitleUrl && audioBaseUrl));
    }
  }, [audioBaseUrl, isOpen, subtitleUrl]);

  useEffect(() => {
    if (isOpen && videoRef.current) {
      videoRef.current.load();
    }
  }, [isOpen, videoUrl]);

  useEffect(() => {
    if (!isOpen || !videoRef.current || !audioBaseUrl || subtitleTimeline.length === 0) {
      syncPlayerRef.current?.destroy();
      syncPlayerRef.current = null;
      return;
    }

    if (!dubbedAudioEnabled) {
      syncPlayerRef.current?.destroy();
      syncPlayerRef.current = null;
      return;
    }

    const player = new VideoSyncPlayer({
      videoElement: videoRef.current,
      subtitles: subtitleTimeline,
      audioBaseUrl,
    });

    syncPlayerRef.current?.destroy();
    syncPlayerRef.current = player;
    player.start();

    return () => {
      if (syncPlayerRef.current === player) {
        player.destroy();
        syncPlayerRef.current = null;
      }
    };
  }, [audioBaseUrl, dubbedAudioEnabled, isOpen, subtitleTimeline]);

  useEffect(() => {
    const videoElement = videoRef.current;
    const syncPlayer = syncPlayerRef.current;

    if (syncPlayer) {
      syncPlayer.setVolume(volume);
      syncPlayer.setMuted(isMuted);
    }

    if (videoElement) {
      videoElement.volume = volume;
      videoElement.muted = Boolean(syncPlayer) || isMuted;
    }
  }, [dubbedAudioEnabled, isMuted, volume]);

  const togglePlay = useCallback(() => {
    const videoElement = videoRef.current;
    if (!videoElement) return;

    if (videoElement.paused) {
      void videoElement.play();
      return;
    }

    videoElement.pause();
  }, []);

  const toggleMute = useCallback(() => {
    setIsMuted((current) => {
      const next = !current;
      if (videoRef.current) {
        videoRef.current.muted = next;
      }
      return next;
    });
  }, []);

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = parseFloat(e.target.value);
    setVolume(newVolume);
    if (videoRef.current) {
      videoRef.current.volume = newVolume;
      if (newVolume === 0) {
        setIsMuted(true);
      } else if (isMuted) {
        setIsMuted(false);
      }
    }
  };

  const handleTimeUpdate = () => {
    if (videoRef.current) {
      setCurrentTime(videoRef.current.currentTime);
    }
  };

  const handleLoadedMetadata = () => {
    if (videoRef.current) {
      setDuration(videoRef.current.duration);
      videoRef.current.volume = volume;
      videoRef.current.muted = Boolean(syncPlayerRef.current) || isMuted;
    }
  };

  const handleProgressChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = parseFloat(e.target.value);
    setCurrentTime(newTime);
    if (videoRef.current) {
      videoRef.current.currentTime = newTime;
    }
  };

  const skip = useCallback((seconds: number) => {
    if (videoRef.current) {
      videoRef.current.currentTime += seconds;
    }
  }, []);

  const toggleFullscreen = useCallback(() => {
    if (videoRef.current) {
      if (document.fullscreenElement) {
        document.exitFullscreen();
      } else {
        videoRef.current.requestFullscreen();
      }
    }
  }, []);

  useEffect(() => {
    const handleKeyPress = (e: KeyboardEvent) => {
      if (!isOpen) return;

      switch (e.key) {
        case ' ':
          e.preventDefault();
          togglePlay();
          break;
        case 'ArrowLeft':
          skip(-10);
          break;
        case 'ArrowRight':
          skip(10);
          break;
        case 'm':
          toggleMute();
          break;
        case 'f':
          toggleFullscreen();
          break;
        case 'Escape':
          onClose();
          break;
      }
    };

    window.addEventListener('keydown', handleKeyPress);
    return () => window.removeEventListener('keydown', handleKeyPress);
  }, [isOpen, onClose, skip, toggleFullscreen, toggleMute, togglePlay]);

  const handlePlaybackRateChange = (rate: number) => {
    setPlaybackRate(rate);
    if (videoRef.current) {
      videoRef.current.playbackRate = rate;
    }
    setShowSettings(false);
  };

  const formatTime = (seconds: number) => {
    if (isNaN(seconds)) return '0:00';
    const hrs = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    
    if (hrs > 0) {
      return `${hrs}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const handleMouseMove = () => {
    setShowControls(true);
    if (controlsTimeoutRef.current) {
      clearTimeout(controlsTimeoutRef.current);
    }
    controlsTimeoutRef.current = setTimeout(() => {
      if (isPlaying) {
        setShowControls(false);
      }
    }, 3000);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 bg-black/95 flex items-center justify-center">
      {/* Close button */}
      <button
        onClick={onClose}
        className="absolute top-4 right-4 p-2 text-white hover:bg-white/10 rounded-full transition-colors z-50"
        aria-label={t('Close')}
      >
        <X className="h-6 w-6" />
      </button>

      <div className="w-full h-full flex flex-col">
        {/* Video container */}
        <div 
          className="flex-1 relative bg-black flex items-center justify-center"
          onMouseMove={handleMouseMove}
          onMouseLeave={() => isPlaying && setShowControls(false)}
        >
          <video
            ref={videoRef}
            className="max-w-full max-h-full"
            playsInline
            onTimeUpdate={handleTimeUpdate}
            onLoadedMetadata={handleLoadedMetadata}
            onPlay={() => setIsPlaying(true)}
            onPause={() => setIsPlaying(false)}
            onEnded={() => setIsPlaying(false)}
          >
            <source src={videoUrl} type="video/mp4" />
            {vttUrl && (
              <track
                kind="subtitles"
                src={vttUrl}
                srcLang="zh"
                label={t('Chinese subtitles')}
                default={subtitleEnabled}
              />
            )}
            {t('Your browser does not support video playback')}
          </video>

          {/* Controls overlay */}
          <div
            className={`absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/90 via-black/50 to-transparent transition-opacity duration-300 ${
              showControls ? 'opacity-100' : 'opacity-0'
            }`}
          >
            {/* Progress bar */}
            <div className="px-4 pb-2">
              <input
                type="range"
                min="0"
                max={duration || 0}
                value={currentTime}
                onChange={handleProgressChange}
                className="w-full h-1 bg-gray-600 rounded-lg appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-red-600 hover:[&::-webkit-slider-thumb]:bg-red-500"
                style={{
                  background: `linear-gradient(to right, #dc2626 0%, #dc2626 ${(currentTime / duration) * 100}%, #4b5563 ${(currentTime / duration) * 100}%, #4b5563 100%)`
                }}
              />
            </div>

            {/* Control buttons */}
            <div className="flex items-center justify-between px-4 pb-4">
              <div className="flex items-center gap-4">
                {/* Play/Pause */}
                <button
                  onClick={togglePlay}
                  className="p-2 hover:bg-white/10 rounded-full transition-colors"
                  aria-label={isPlaying ? t('Pause') : t('Play')}
                >
                  {isPlaying ? (
                    <Pause className="h-6 w-6 text-white fill-white" />
                  ) : (
                    <Play className="h-6 w-6 text-white fill-white" />
                  )}
                </button>

                {/* Skip buttons */}
                <button
                  onClick={() => skip(-10)}
                  className="p-2 hover:bg-white/10 rounded-full transition-colors"
                  aria-label={t('Back 10 seconds')}
                >
                  <SkipBack className="h-5 w-5 text-white" />
                </button>
                <button
                  onClick={() => skip(10)}
                  className="p-2 hover:bg-white/10 rounded-full transition-colors"
                  aria-label={t('Forward 10 seconds')}
                >
                  <SkipForward className="h-5 w-5 text-white" />
                </button>

                {/* Volume */}
                <div className="flex items-center gap-2 group">
                  <button
                    onClick={toggleMute}
                    className="p-2 hover:bg-white/10 rounded-full transition-colors"
                    aria-label={isMuted ? t('Unmute') : t('Mute')}
                  >
                    {isMuted || volume === 0 ? (
                      <VolumeX className="h-5 w-5 text-white" />
                    ) : (
                      <Volume2 className="h-5 w-5 text-white" />
                    )}
                  </button>
                  <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.1"
                    value={isMuted ? 0 : volume}
                    onChange={handleVolumeChange}
                    className="w-0 group-hover:w-20 transition-all duration-200 h-1 bg-gray-600 rounded-lg appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white"
                  />
                </div>

                {/* Time */}
                <span className="text-white text-sm font-medium">
                  {formatTime(currentTime)} / {formatTime(duration)}
                </span>
              </div>

              <div className="flex items-center gap-2">
                {(audioBaseUrl && subtitleTimeline.length > 0) && (
                  <button
                    onClick={() => setDubbedAudioEnabled((current) => !current)}
                    className={`p-2 hover:bg-white/10 rounded-full transition-colors ${
                      dubbedAudioEnabled ? 'text-sky-400' : 'text-white'
                    }`}
                    aria-label={dubbedAudioEnabled ? t('Turn off subtitle voiceover') : t('Turn on subtitle voiceover')}
                    title={dubbedAudioEnabled ? t('Turn off subtitle voiceover') : t('Turn on subtitle voiceover')}
                  >
                    <AudioLines className="h-5 w-5" />
                  </button>
                )}

                {/* Subtitle toggle */}
                {vttUrl && (
                  <button
                    onClick={() => {
                      setSubtitleEnabled(!subtitleEnabled);
                      if (videoRef.current) {
                        const tracks = videoRef.current.textTracks;
                        if (tracks.length > 0) {
                          tracks[0].mode = subtitleEnabled ? 'hidden' : 'showing';
                        }
                      }
                    }}
                    className={`p-2 hover:bg-white/10 rounded-full transition-colors ${
                      subtitleEnabled ? 'text-yellow-400' : 'text-white'
                    }`}
                    aria-label={subtitleEnabled ? t('Turn off subtitles') : t('Turn on subtitles')}
                    title={subtitleEnabled ? t('Turn off subtitles') : t('Turn on subtitles')}
                  >
                    <Subtitles className="h-5 w-5" />
                  </button>
                )}

                {/* Settings (Playback speed) */}
                <div className="relative">
                  <button
                    onClick={() => setShowSettings(!showSettings)}
                    className="p-2 hover:bg-white/10 rounded-full transition-colors"
                    aria-label={t('Settings')}
                  >
                    <Settings className="h-5 w-5 text-white" />
                  </button>
                  {showSettings && (
                    <div className="absolute bottom-full right-0 mb-2 bg-gray-900 rounded-lg shadow-lg overflow-hidden">
                      <div className="px-3 py-2 text-xs text-gray-400 border-b border-gray-700">{t('Playback speed')}</div>
                      {[0.75, 1, 1.25, 1.5, 1.75, 2].map((rate) => (
                        <button
                          key={rate}
                          onClick={() => handlePlaybackRateChange(rate)}
                          className={`w-full px-4 py-2 text-sm text-left hover:bg-white/10 transition-colors ${
                            playbackRate === rate ? 'text-red-500' : 'text-white'
                          }`}
                        >
                          {rate === 1 ? t('Normal') : `${rate}x`}
                        </button>
                      ))}
                    </div>
                  )}
                </div>

                {/* Fullscreen */}
                <button
                  onClick={toggleFullscreen}
                  className="p-2 hover:bg-white/10 rounded-full transition-colors"
                  aria-label={t('Fullscreen')}
                >
                  <Maximize className="h-5 w-5 text-white" />
                </button>
              </div>
            </div>
          </div>

          {/* Center play button overlay */}
          {!isPlaying && (
            <button
              onClick={togglePlay}
              className="absolute inset-0 flex items-center justify-center"
            >
              <div className="w-20 h-20 bg-red-600 rounded-full flex items-center justify-center hover:bg-red-500 transition-colors">
                <Play className="h-10 w-10 text-white fill-white ml-1" />
              </div>
            </button>
          )}
        </div>

        {/* Video info */}
        <div className="bg-black px-6 py-4 border-t border-gray-800">
          <h2 className="text-white text-xl font-semibold mb-2">{title}</h2>
          {description && (
            <p className="text-gray-400 text-sm line-clamp-2">{description}</p>
          )}
        </div>
      </div>
    </div>
  );
}
