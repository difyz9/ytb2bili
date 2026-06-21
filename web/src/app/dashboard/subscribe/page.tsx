'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { 
  Play, 
  Rss,
  Users,
  ExternalLink,
  RefreshCw,
  Search,
  Loader2,
  PauseCircle,
  CheckCircle2,
  Radio
} from 'lucide-react';
import { cn } from '@/lib/utils';
import api, { SubscriptionVideo, TbSubscription } from '@/lib/api';
import { useAuth } from '@/contexts/AuthContext';
import { useI18n } from '@/contexts/I18nContext';

// 使用 api.ts 中的 TbSubscription 类型作为 Channel
type Channel = TbSubscription;

type TabType = 'videos' | 'channels';
type ChannelStatusFilter = 'all' | 'active' | 'inactive';
type VideoChannelStatusFilter = 'all' | 'active' | 'inactive';

function formatDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);
  
  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  }
  return `${minutes}:${secs.toString().padStart(2, '0')}`;
}

function getYouTubeThumbnail(videoId: string): string {
  return `https://i.ytimg.com/vi/${videoId}/mqdefault.jpg`;
}

function getYouTubeWatchUrl(videoId: string): string {
  return `https://www.youtube.com/watch?v=${videoId}`;
}

export default function VideoListPage() {
  const { currentUser } = useAuth();
  const { locale, t } = useI18n();
  const [activeTab, setActiveTab] = useState<TabType>('videos');
  
  // Videos state
  const [videos, setVideos] = useState<SubscriptionVideo[]>([]);
  const [videosLoading, setVideosLoading] = useState(true);
  const [videosLoaded, setVideosLoaded] = useState(false);
  const [syncingFeed, setSyncingFeed] = useState(false);
  const [videosPage, setVideosPage] = useState(1);
  const [videosTotal, setVideosTotal] = useState(0);
  const [videosHasMore, setVideosHasMore] = useState(true);
  const [videosLoadingMore, setVideosLoadingMore] = useState(false);
  const [videoSearchTerm, setVideoSearchTerm] = useState('');
  const [appliedVideoSearchTerm, setAppliedVideoSearchTerm] = useState('');
  const [videoChannelStatusFilter, setVideoChannelStatusFilter] = useState<VideoChannelStatusFilter>('all');
  
  // Channels state
  const [channels, setChannels] = useState<Channel[]>([]);
  const [channelsLoading, setChannelsLoading] = useState(true);
  const [channelsLoaded, setChannelsLoaded] = useState(false);
  const [channelsPage, setChannelsPage] = useState(1);
  const [channelsTotal, setChannelsTotal] = useState(0);
  const [channelsPageSize] = useState(20);
  const [channelsHasMore, setChannelsHasMore] = useState(true);
  const [channelsLoadingMore, setChannelsLoadingMore] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [appliedSearchTerm, setAppliedSearchTerm] = useState('');
  const [channelStatusFilter, setChannelStatusFilter] = useState<ChannelStatusFilter>('all');
  const [updatingChannelId, setUpdatingChannelId] = useState<number | null>(null);
  
  const observerTarget = useRef<HTMLDivElement>(null);
  const channelsObserverTarget = useRef<HTMLDivElement>(null);
  const videosLoadingMoreRef = useRef(false);
  const channelsLoadingMoreRef = useRef(false);

  useEffect(() => {
    videosLoadingMoreRef.current = videosLoadingMore;
  }, [videosLoadingMore]);

  useEffect(() => {
    channelsLoadingMoreRef.current = channelsLoadingMore;
  }, [channelsLoadingMore]);

  const fetchVideos = useCallback(async (pageNum: number, append: boolean = false) => {
    if (videosLoadingMoreRef.current && append) return;
    
    try {
      if (append) {
        setVideosLoadingMore(true);
      } else {
        setVideosLoading(true);
      }
      
      const params: {
        user_id?: string;
        page: number;
        pageSize: number;
        search?: string;
        channel_status?: 'active' | 'inactive';
      } = {
        page: pageNum,
        pageSize: 20,
        user_id: currentUser?.id,
      };
      if (appliedVideoSearchTerm.trim()) {
        params.search = appliedVideoSearchTerm.trim();
      }
      if (videoChannelStatusFilter !== 'all') {
        params.channel_status = videoChannelStatusFilter;
      }

      const response = await api.getYouTubeFeedVideos(params);
      const newVideos = response.list || [];
      
      if (append) {
        setVideos(prev => [...prev, ...newVideos]);
      } else {
        setVideos(newVideos);
      }
      
      setVideosTotal(response.total || 0);
      const calculatedPages = response.size ? Math.ceil(response.total / response.size) : 1;
      setVideosHasMore(pageNum < calculatedPages);
      setVideosPage(pageNum);
    } catch (error) {
      console.error('Failed to fetch videos:', error);
      setVideos([]);
      setVideosTotal(0);
    } finally {
      setVideosLoading(false);
      setVideosLoadingMore(false);
      setVideosLoaded(true);
    }
  }, [currentUser?.id, appliedVideoSearchTerm, videoChannelStatusFilter]);

  const fetchChannels = useCallback(async (pageNum: number, append: boolean = false) => {
    if (!currentUser?.id) return;
    if (channelsLoadingMoreRef.current && append) return;
    
    try {
      if (append) {
        setChannelsLoadingMore(true);
      } else {
        setChannelsLoading(true);
      }
      
      const params: {
        user_id: string;
        page: number;
        page_size: number;
        search?: string;
        status?: 'active' | 'inactive';
      } = {
        user_id: currentUser.id,
        page: pageNum,
        page_size: channelsPageSize,
      };
      if (appliedSearchTerm.trim()) {
        params.search = appliedSearchTerm.trim();
      }
      if (channelStatusFilter !== 'all') {
        params.status = channelStatusFilter;
      }

      const response = await api.getYouTubeTbSubscriptions(params);
      
      const newChannels = response.list || [];
      
      if (append) {
        setChannels(prev => [...prev, ...newChannels]);
      } else {
        setChannels(newChannels);
      }
        
      setChannelsTotal(response.total || 0);
      const calculatedPages = Math.ceil((response.total || 0) / channelsPageSize);
      setChannelsHasMore(pageNum < calculatedPages);
      setChannelsPage(pageNum);
    } catch (error) {
      console.error('Failed to fetch channels:', error);
      if (!append) {
        setChannels([]);
      }
    } finally {
      setChannelsLoading(false);
      setChannelsLoadingMore(false);
      setChannelsLoaded(true);
    }
  }, [currentUser?.id, channelsPageSize, channelStatusFilter, appliedSearchTerm]);

  const handleRefreshFeed = async () => {
    try {
      setSyncingFeed(true);
      await api.refreshYouTubeFeed();
      setTimeout(() => {
        setVideosPage(1);
        setVideosHasMore(true);
        fetchVideos(1, false);
        setSyncingFeed(false);
      }, 3000);
    } catch (error) {
      console.error('Failed to refresh feed:', error);
      setSyncingFeed(false);
    }
  };

  const handleVideoSearch = () => {
    setAppliedVideoSearchTerm(videoSearchTerm.trim());
    setVideosPage(1);
    setVideosHasMore(true);
  };

  const handleSearch = () => {
    setAppliedSearchTerm(searchTerm.trim());
    setChannelsPage(1);
    setChannelsHasMore(true);
  };

  useEffect(() => {
    if (!currentUser || activeTab !== 'videos') return;
    setVideosPage(1);
    setVideosHasMore(true);
    fetchVideos(1, false);
  }, [activeTab, currentUser, appliedVideoSearchTerm, videoChannelStatusFilter, fetchVideos]);

  const handleToggleChannelSync = async (channel: Channel) => {
    if (!currentUser?.id || updatingChannelId === channel.id) return;

    const nextEnabled = channel.status !== 'active';
    setUpdatingChannelId(channel.id);
    try {
      const response = await api.updateYouTubeTbSubscriptionStatus(channel.id, {
        user_id: currentUser.id,
        sync_enabled: nextEnabled,
      });
      const nextStatus = response.subscription.status;
      setChannels((prev) => prev
        .map((item) => (item.id === channel.id ? { ...item, status: nextStatus } : item))
        .filter((item) => channelStatusFilter === 'all' ? true : item.status === channelStatusFilter));
      if (channelStatusFilter !== 'all' && nextStatus !== channelStatusFilter) {
        setChannelsTotal((prev) => Math.max(0, prev - 1));
      }
    } catch (error) {
      console.error('Failed to update channel sync status:', error);
    } finally {
      setUpdatingChannelId(null);
    }
  };

  const handleToggleVideoChannelSync = async (video: SubscriptionVideo) => {
    if (!currentUser?.id || !video.subscription_id || updatingChannelId === video.subscription_id) return;

    const nextEnabled = video.channel_status !== 'active';
    setUpdatingChannelId(video.subscription_id);
    try {
      const response = await api.updateYouTubeTbSubscriptionStatus(video.subscription_id, {
        user_id: currentUser.id,
        sync_enabled: nextEnabled,
      });
      const nextStatus = response.subscription.status;
      setVideos((prev) => prev.map((item) => (
        item.channel_id === video.channel_id
          ? { ...item, channel_status: nextStatus, subscription_id: response.subscription.id }
          : item
      )));
      setChannels((prev) => prev.map((item) => (
        item.id === response.subscription.id ? { ...item, status: nextStatus } : item
      )));
    } catch (error) {
      console.error('Failed to update video channel sync status:', error);
    } finally {
      setUpdatingChannelId(null);
    }
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return '-';
    return new Date(dateString).toLocaleString(locale, {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  // Infinite scroll for videos
  useEffect(() => {
    if (activeTab !== 'videos') return;

    const target = observerTarget.current;
    
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && videosHasMore && !videosLoadingMore && !videosLoading) {
          fetchVideos(videosPage + 1, true);
        }
      },
      { threshold: 0.1 }
    );

    if (target) {
      observer.observe(target);
    }

    return () => {
      if (target) {
        observer.unobserve(target);
      }
    };
  }, [videosHasMore, videosLoadingMore, videosLoading, videosPage, fetchVideos, activeTab]);

  // Infinite scroll for channels
  useEffect(() => {
    if (activeTab !== 'channels') return;

    const target = channelsObserverTarget.current;
    
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && channelsHasMore && !channelsLoadingMore && !channelsLoading) {
          fetchChannels(channelsPage + 1, true);
        }
      },
      { threshold: 0.1 }
    );

    if (target) {
      observer.observe(target);
    }

    return () => {
      if (target) {
        observer.unobserve(target);
      }
    };
  }, [channelsHasMore, channelsLoadingMore, channelsLoading, channelsPage, fetchChannels, activeTab]);

  useEffect(() => {
    if (activeTab !== 'channels' || !currentUser) return;
    setChannelsPage(1);
    setChannelsHasMore(true);
    fetchChannels(1, false);
  }, [activeTab, currentUser, channelStatusFilter, appliedSearchTerm, fetchChannels]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">{t('YouTube subscriptions')}</h2>
          <p className="text-muted-foreground mt-2">
            {activeTab === 'videos' 
              ? t('Latest videos from your subscribed channels · {count} videos', { count: new Intl.NumberFormat(locale).format(videosTotal) })
              : t('Subscribed channels · {count} channels', { count: new Intl.NumberFormat(locale).format(channelsTotal) })
            }
          </p>
        </div>
        {activeTab === 'videos' && (
          <button
            onClick={handleRefreshFeed}
            disabled={syncingFeed}
            className="flex items-center space-x-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50"
          >
            <Rss className={cn('h-4 w-4', syncingFeed && 'animate-spin')} />
            <span>{syncingFeed ? t('Syncing...') : t('Sync subscriptions')}</span>
          </button>
        )}
        {activeTab === 'channels' && (
          <button
            onClick={() => fetchChannels(1, false)}
            className="flex items-center space-x-2 px-4 py-2 border border-border rounded-lg hover:bg-accent transition-colors"
          >
            <RefreshCw className="h-4 w-4" />
            <span>{t('Refresh')}</span>
          </button>
        )}
      </div>

      {/* Tabs */}
      <div className="border-b border-border">
        <div className="flex space-x-8">
          <button
            onClick={() => setActiveTab('videos')}
            className={cn(
              'pb-4 px-1 border-b-2 font-medium transition-colors',
              activeTab === 'videos'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            <div className="flex items-center space-x-2">
              <Play className="h-4 w-4" />
              <span>{t('Subscription videos')}</span>
            </div>
          </button>
          <button
            onClick={() => setActiveTab('channels')}
            className={cn(
              'pb-4 px-1 border-b-2 font-medium transition-colors',
              activeTab === 'channels'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            <div className="flex items-center space-x-2">
              <Users className="h-4 w-4" />
              <span>{t('Subscriptions')}</span>
            </div>
          </button>
        </div>
      </div>

      {/* Videos Tab Content */}
      {activeTab === 'videos' && (
        <>
          <div className="flex items-center space-x-4">
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <input
                type="text"
                placeholder={t('Search video titles or channels...')}
                value={videoSearchTerm}
                onChange={(e) => setVideoSearchTerm(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleVideoSearch()}
                className="w-full pl-10 pr-4 py-2 border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            <button
              onClick={handleVideoSearch}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
            >
              {t('Search')}
            </button>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {[
              { value: 'all', label: t('All videos') },
              { value: 'active', label: t('Syncing channels') },
              { value: 'inactive', label: t('Paused channels') },
            ].map((option) => (
              <button
                key={option.value}
                onClick={() => setVideoChannelStatusFilter(option.value as VideoChannelStatusFilter)}
                className={cn(
                  'rounded-full px-4 py-2 text-sm font-medium transition-colors',
                  videoChannelStatusFilter === option.value
                    ? 'bg-primary text-primary-foreground'
                    : 'border border-border bg-background text-muted-foreground hover:bg-accent hover:text-foreground'
                )}
              >
                {option.label}
              </button>
            ))}
          </div>

          {videosLoading && !videosLoaded ? (
            <div className="flex items-center justify-center min-h-[400px]">
              <div className="text-center">
                <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
                <p className="mt-4 text-muted-foreground">{t('Loading...')}</p>
              </div>
            </div>
          ) : videos.length === 0 ? (
            <div className="rounded-lg border border-border bg-card p-12 text-center">
              <Play className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold mb-2">{t('No videos')}</h3>
              <p className="text-muted-foreground">
                {t('Click "Sync subscriptions" to fetch the latest videos from your YouTube subscriptions.')}
              </p>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {videos.map((video) => (
                  <div 
                    key={video.id} 
                    className="group cursor-pointer"
                    onClick={() => {
                      if (video.video_id) {
                        window.open(getYouTubeWatchUrl(video.video_id), '_blank', 'noopener,noreferrer');
                      }
                    }}
                  >
                    <div className="relative aspect-video bg-muted rounded-xl overflow-hidden mb-3">
                      {video.video_id ? (
                        <img
                          src={video.img_url || getYouTubeThumbnail(video.video_id)}
                          alt={video.title}
                          className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200">
                          <Play className="h-16 w-16 text-gray-400" />
                        </div>
                      )}
                      {video.duration > 0 && (
                        <div className="absolute bottom-2 right-2 bg-black/90 text-white text-xs font-semibold px-1.5 py-0.5 rounded">
                          {formatDuration(video.duration)}
                        </div>
                      )}
                      <div className="absolute inset-0 flex items-center justify-center bg-black/0 group-hover:bg-black/30 transition-colors">
                        <div className="opacity-0 group-hover:opacity-100 transition-opacity">
                          <div className="bg-white/95 rounded-full p-3">
                            <Play className="h-6 w-6 text-black fill-black" />
                          </div>
                        </div>
                      </div>
                    </div>
                    <div className="flex gap-3">
                      <div className="flex-shrink-0 w-9 h-9 rounded-full bg-gradient-to-br from-blue-400 to-purple-500 flex items-center justify-center text-white text-sm font-semibold">
                        {video.channel_title ? video.channel_title.substring(0, 1).toUpperCase() : video.channel_id ? video.channel_id.substring(0, 1).toUpperCase() : 'Y'}
                      </div>
                      <div className="flex-1 min-w-0">
                        <h3 className="font-medium text-sm line-clamp-2 mb-1 group-hover:text-blue-600 transition-colors">
                          {video.title}
                        </h3>
                        <div className="text-xs text-muted-foreground space-y-0.5">
                          <div className="flex items-center justify-between gap-2">
                            <p className="truncate">{t('Channel')}: {video.channel_title || t('Unknown channel')}</p>
                            <span className={cn(
                              'inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-[11px] font-medium',
                              video.channel_status === 'active'
                                ? 'bg-emerald-50 text-emerald-700'
                                : 'bg-slate-100 text-slate-600'
                            )}>
                              {video.channel_status === 'active' ? t('Syncing') : t('Paused')}
                            </span>
                          </div>
                          <p className="truncate">{t('Channel ID')}: {video.channel_id ? video.channel_id.substring(0, 16) + '...' : t('Unknown')}</p>
                          <p>{new Date(video.created_at).toLocaleDateString(locale, { 
                            year: 'numeric', 
                            month: 'short', 
                            day: 'numeric' 
                          })}</p>
                        </div>
                        {video.subscription_id ? (
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              void handleToggleVideoChannelSync(video);
                            }}
                            disabled={updatingChannelId === video.subscription_id}
                            className={cn(
                              'mt-3 inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50',
                              video.channel_status === 'active'
                                ? 'bg-amber-50 text-amber-700 hover:bg-amber-100'
                                : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
                            )}
                          >
                            {updatingChannelId === video.subscription_id ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : video.channel_status === 'active' ? (
                              <PauseCircle className="h-3.5 w-3.5" />
                            ) : (
                              <Radio className="h-3.5 w-3.5" />
                            )}
                            <span>{video.channel_status === 'active' ? t('Pause channel sync') : t('Resume channel sync')}</span>
                          </button>
                        ) : null}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
              <div className="py-8">
                <div className="flex min-h-[72px] items-center justify-center">
                  {videosLoadingMore ? (
                    <div className="flex flex-col items-center space-y-2">
                      <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
                      <p className="text-sm text-muted-foreground">{t('Loading...')}</p>
                    </div>
                  ) : !videosHasMore && videos.length > 0 ? (
                    <p className="text-sm text-muted-foreground">
                      {t('All {count} videos loaded', { count: new Intl.NumberFormat(locale).format(videosTotal) })}
                    </p>
                  ) : null}
                </div>
                <div ref={observerTarget} className="h-px w-full" aria-hidden="true" />
              </div>
            </>
          )}
        </>
      )}

      {/* Channels Tab Content */}
      {activeTab === 'channels' && (
        <>
          {/* Search */}
          <div className="flex items-center space-x-4">
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <input
                type="text"
                placeholder={t('Search channels...')}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                className="w-full pl-10 pr-4 py-2 border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            <button
              onClick={handleSearch}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
            >
              {t('Search')}
            </button>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {[
              { value: 'all', label: t('All channels') },
              { value: 'active', label: t('Syncing') },
              { value: 'inactive', label: t('Paused') },
            ].map((option) => (
              <button
                key={option.value}
                onClick={() => setChannelStatusFilter(option.value as ChannelStatusFilter)}
                className={cn(
                  'rounded-full px-4 py-2 text-sm font-medium transition-colors',
                  channelStatusFilter === option.value
                    ? 'bg-primary text-primary-foreground'
                    : 'border border-border bg-background text-muted-foreground hover:bg-accent hover:text-foreground'
                )}
              >
                {option.label}
              </button>
            ))}
          </div>

          {channelsLoading && !channelsLoaded ? (
            <div className="flex items-center justify-center min-h-[320px]">
              <div className="text-center">
                <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
                <p className="mt-4 text-muted-foreground">{t('Loading channels...')}</p>
              </div>
            </div>
          ) : channels.length === 0 ? (
            <div className="rounded-lg border border-border bg-card p-12 text-center">
              <Users className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold mb-2">{t('No subscribed channels')}</h3>
              <p className="text-muted-foreground">{t('You have not subscribed to any YouTube channels yet.')}</p>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {channels.map((channel) => (
                  <div key={channel.id} className="rounded-lg border border-border bg-card p-6 hover:shadow-md transition-shadow">
                    <div className="flex items-start space-x-4">
                      <img
                        src={channel.channel_thumbnail_url}
                        alt={channel.channel_title}
                        className="w-16 h-16 rounded-full flex-shrink-0"
                        onError={(e) => {
                          e.currentTarget.src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="64" height="64"%3E%3Crect width="64" height="64" fill="%23ddd"/%3E%3C/svg%3E';
                        }}
                      />
                      <div className="flex-1 min-w-0">
                        <div className="mb-2 flex items-start justify-between gap-3">
                          <h3 className="font-semibold text-lg truncate">{channel.channel_title}</h3>
                          <span className={cn(
                            'inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium',
                            channel.status === 'active'
                              ? 'bg-emerald-50 text-emerald-700'
                              : 'bg-slate-100 text-slate-600'
                          )}>
                            {channel.status === 'active' ? t('Syncing') : t('Paused')}
                          </span>
                        </div>
                        {channel.channel_description && (
                          <p className="text-sm text-muted-foreground line-clamp-2 mb-2">
                            {channel.channel_description}
                          </p>
                        )}
                        <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                          <span>{t('Subscribed on {date}', { date: formatDate(channel.subscribed_at) })}</span>
                          <span>{t('Last synced {date}', { date: formatDate(channel.synced_at) })}</span>
                        </div>
                        <div className="mt-4 flex flex-wrap items-center gap-3">
                          <button
                            onClick={() => handleToggleChannelSync(channel)}
                            disabled={updatingChannelId === channel.id}
                            className={cn(
                              'inline-flex items-center space-x-1 rounded-lg px-3 py-2 text-sm font-medium transition-colors disabled:opacity-50',
                              channel.status === 'active'
                                ? 'bg-amber-50 text-amber-700 hover:bg-amber-100'
                                : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
                            )}
                          >
                            {updatingChannelId === channel.id ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : channel.status === 'active' ? (
                              <PauseCircle className="h-4 w-4" />
                            ) : (
                              <CheckCircle2 className="h-4 w-4" />
                            )}
                            <span>{channel.status === 'active' ? t('Pause sync') : t('Resume sync')}</span>
                          </button>
                          <a
                            href={`https://www.youtube.com/channel/${channel.channel_id}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center space-x-1 text-sm text-primary hover:underline"
                          >
                            <span>{t('Visit channel')}</span>
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Infinite scroll trigger and loading indicator */}
              <div className="py-8 text-center">
                <div className="flex min-h-[56px] items-center justify-center">
                  {channelsLoadingMore ? (
                    <div className="flex items-center justify-center space-x-2 text-muted-foreground">
                      <Loader2 className="h-5 w-5 animate-spin" />
                      <span>{t('Loading more channels...')}</span>
                    </div>
                  ) : !channelsHasMore && channels.length > 0 ? (
                    <div className="text-sm text-muted-foreground">
                      {t('All channels loaded')}
                    </div>
                  ) : null}
                </div>
                <div ref={channelsObserverTarget} className="h-px w-full" aria-hidden="true" />
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
