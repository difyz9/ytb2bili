'use client';

import { useState, useEffect, useCallback } from 'react';
import { Video, ExternalLink, RefreshCw, PlayCircle } from 'lucide-react';
import VideoPlayerModal from '@/components/VideoPlayerModal';
import { useI18n } from '@/contexts/I18nContext';
import { guessSubtitleAudioBaseUrl, guessSubtitleUrl, videoPathToUrl } from '@/lib/video-paths';

interface VideoItem {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: string;
  video_id: string;
  platform: string;
  url: string;
  title: string;
  description: string;
  thumbnail: string;
  duration: number;
  status: string;
  retry_count: number;
  generated_title: string;
  generated_desc: string;
  generated_tags: string;
  bili_bvid: string;
  bili_aid: number;
  video_path: string;
  subtitle_path: string;
}

// 状态码映射
const STATUS_MAP: Record<string, { label: string; color: string }> = {
  '001': { label: 'Pending', color: 'bg-yellow-500' },
  '002': { label: 'Processing', color: 'bg-blue-500' },
  '003': { label: 'Completed', color: 'bg-green-500' },
  '004': { label: 'Failed', color: 'bg-red-500' },
  pending:    { label: 'Pending', color: 'bg-yellow-500' },
  processing: { label: 'Processing', color: 'bg-blue-500' },
  completed:  { label: 'Completed', color: 'bg-green-500' },
  ready:      { label: 'Ready', color: 'bg-green-500' },
  failed:     { label: 'Failed', color: 'bg-red-500' },
  uploaded:   { label: 'Uploaded', color: 'bg-purple-500' },
};

const PAGE_SIZE = 20;

export default function VideosPage() {
  const { locale, t } = useI18n();
  const [videos, setVideos] = useState<VideoItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState('');
  const [playerVideo, setPlayerVideo] = useState<{ title: string; videoUrl: string; subtitleUrl?: string; audioBaseUrl?: string } | null>(null);

  const fetchVideos = useCallback(async () => {
    try {
      setRefreshing(true);
      const params = new URLSearchParams({
        source_type: 'manual',   // 只看本地上传 / 下载的视频
        page: String(page),
        limit: String(PAGE_SIZE),
      });
      if (statusFilter) params.set('status', statusFilter);

      const res = await fetch(`/api/v1/videos?${params}`);
      const json = await res.json();
      // 后端返回 { code, data: { videos, total, total_pages } }
      if ((json.code === 0 || json.code === 200) && json.data) {
        setVideos(json.data.videos ?? []);
        setTotal(json.data.total ?? 0);
        setTotalPages(json.data.total_pages ?? 1);
      } else {
        setVideos([]); setTotal(0); setTotalPages(1);
      }
    } catch {
      setVideos([]);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [page, statusFilter]);

  useEffect(() => { fetchVideos(); }, [fetchVideos]);

  const getStatusBadge = (status: string) => {
    const info = STATUS_MAP[status] ?? { label: status, color: 'bg-gray-500' };
    return (
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium text-white ${info.color}`}>
        {t(info.label)}
      </span>
    );
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return '-';
    const diff = Date.now() - new Date(dateString).getTime();
    const days = Math.floor(diff / 86400000);
    const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
    if (days < 7) return formatter.format(-days, 'day');
    if (days < 30) return formatter.format(-Math.floor(days / 7), 'week');
    if (days < 365) return formatter.format(-Math.floor(days / 30), 'month');
    return formatter.format(-Math.floor(days / 365), 'year');
  };

  const formatDuration = (seconds: number) => {
    if (!seconds) return '';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) return `${h}:${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`;
    return `${m}:${String(s).padStart(2,'0')}`;
  };

  const openPlayer = (video: VideoItem) => {
    if (!video.video_path) return;
    setPlayerVideo({
      title: video.generated_title || video.title || video.video_id,
      videoUrl: videoPathToUrl(video.video_path),
      subtitleUrl: guessSubtitleUrl(video),
      audioBaseUrl: guessSubtitleAudioBaseUrl(video),
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-r-transparent" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-3">
          <select
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
            className="px-3 py-1.5 text-sm border border-border rounded-full bg-background hover:bg-accent transition-colors cursor-pointer"
          >
            <option value="">{t('All statuses')}</option>
            <option value="001">{t('Pending')}</option>
            <option value="002">{t('Processing')}</option>
            <option value="003">{t('Completed')}</option>
            <option value="004">{t('Failed')}</option>
          </select>
          <span className="text-sm text-muted-foreground">{t('{count} videos', { count: total })}</span>
        </div>
        <button
          onClick={() => fetchVideos()}
          disabled={refreshing}
          className="flex items-center gap-2 px-3 py-1.5 text-sm hover:bg-accent rounded-full transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
          {t('Refresh')}
        </button>
      </div>

      {/* Video Grid */}
      {videos.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="w-24 h-24 rounded-full bg-muted flex items-center justify-center mb-4">
            <Video className="h-12 w-12 text-muted-foreground" />
          </div>
          <h3 className="text-lg font-semibold mb-1">{t('No local videos yet')}</h3>
          <p className="text-sm text-muted-foreground">{t('Downloaded or uploaded videos will appear here.')}</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {videos.map((video) => (
            <div key={video.id} className="group">
              {/* Thumbnail */}
              <div
                className={`relative aspect-video bg-muted rounded-xl overflow-hidden mb-3 ${video.video_path ? 'cursor-pointer' : 'cursor-default'}`}
                onClick={() => openPlayer(video)}
              >
                <img
                  src={`/static/${video.video_id}/thumbnail_maxresdefault.jpg`}
                  alt={video.title || t('Video thumbnail')}
                  className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
                  onError={(e) => {
                    const image = e.target as HTMLImageElement;
                    const retryCount = parseInt(image.dataset.retryCount || '0');
                    if (retryCount >= 3) {
                      image.onerror = null;
                      image.src = `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect fill="#374151" width="320" height="180"/><text fill="#9CA3AF" font-family="sans-serif" font-size="20" x="50%" y="50%" text-anchor="middle" dominant-baseline="middle">${t('No cover')}</text></svg>`)}`;
                      return;
                    }
                    image.dataset.retryCount = String(retryCount + 1);
                    const src = image.src;
                    if (src.includes('maxresdefault'))      image.src = `/static/${video.video_id}/thumbnail_sddefault.jpg`;
                    else if (src.includes('sddefault'))     image.src = `/static/${video.video_id}/thumbnail_hqdefault.jpg`;
                    else if (src.includes('hqdefault'))     image.src = `/static/${video.video_id}/thumbnail_mqdefault.jpg`;
                    else { image.onerror = null; image.src = video.thumbnail || `https://i.ytimg.com/vi/${video.video_id}/mqdefault.jpg`; }
                  }}
                />

                {/* Duration */}
                {video.duration > 0 && (
                  <div className="absolute bottom-2 right-2 px-1.5 py-0.5 bg-black/80 text-white text-xs font-medium rounded">
                    {formatDuration(video.duration)}
                  </div>
                )}

                {/* Status */}
                <div className="absolute top-2 left-2">{getStatusBadge(video.status)}</div>

                {/* Play overlay — 只在有本地文件时显示 */}
                {video.video_path && (
                  <div className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity bg-black/30">
                    <PlayCircle className="h-14 w-14 text-white drop-shadow-lg" />
                  </div>
                )}
              </div>

              {/* Info */}
              <div className="flex-1 min-w-0 px-0.5">
                <h3 className="font-medium text-sm line-clamp-2 mb-1 group-hover:text-primary transition-colors">
                  {video.title || video.generated_title || t('Untitled')}
                </h3>
                <div className="flex flex-col gap-0.5 text-xs text-muted-foreground">
                  <div className="flex items-center gap-1.5">
                    <span>{video.platform || 'YouTube'}</span>
                    {video.bili_bvid && (
                      <>
                        <span>·</span>
                        <a
                          href={`https://www.bilibili.com/video/${video.bili_bvid}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-primary hover:underline flex items-center gap-1"
                          onClick={(e) => e.stopPropagation()}
                        >
                          {t('Uploaded to Bilibili')} <ExternalLink className="h-3 w-3" />
                        </a>
                      </>
                    )}
                  </div>
                  <span>{formatDate(video.created_at)}</span>
                </div>
                {video.generated_title && video.generated_title !== video.title && (
                  <p className="mt-1 text-xs text-muted-foreground italic line-clamp-1">
                    {t('AI title:')} {video.generated_title}
                  </p>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 pt-4">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="px-4 py-2 text-sm border border-border rounded-full hover:bg-accent disabled:opacity-40 transition-colors"
          >
            {t('Previous page')}
          </button>
          <span className="text-sm text-muted-foreground">{page} / {totalPages}</span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className="px-4 py-2 text-sm border border-border rounded-full hover:bg-accent disabled:opacity-40 transition-colors"
          >
            {t('Next page')}
          </button>
        </div>
      )}

      {/* Video Player Modal */}
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
    </div>
  );
}
