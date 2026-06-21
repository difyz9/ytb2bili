'use client';

import { useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { translateClientText } from '@/lib/i18n';

function YouTubeCallbackContent() {
  const searchParams = useSearchParams();

  useEffect(() => {
    const channelId = searchParams.get('channel_id');
    const channelTitle = searchParams.get('channel_title');
    const TbSubscriptions = searchParams.get('TbSubscriptions');
    const error = searchParams.get('error');

    // 检查是否在OAuth弹窗中
    const isPopup = window.opener && !window.opener.closed;

    if (isPopup) {
      // 如果是OAuth弹窗，通知主窗口并关闭
      if (error) {
        // 授权失败
        window.opener.postMessage(
          {
            type: 'YOUTUBE_AUTH_ERROR',
            error: error,
          },
          window.location.origin
        );
      } else if (channelId && channelTitle) {
        // 授权成功
        const message = translateClientText('YouTube channel "{title}" authorized successfully!', {
          title: decodeURIComponent(channelTitle),
        });
        window.opener.postMessage(
          {
            type: 'YOUTUBE_AUTH_SUCCESS',
            channelId,
            channelTitle: decodeURIComponent(channelTitle),
            TbSubscriptions,
            message,
          },
          window.location.origin
        );
      }

      // 短暂延迟后关闭窗口，确保消息已发送
      setTimeout(() => {
        window.close();
      }, 500);
    } else {
      // 如果不是弹窗（用户直接访问），重定向到账号管理页面
      if (error) {
        window.location.href = `/dashboard/accounts?error=${error}`;
      } else if (channelId && channelTitle) {
        const message = translateClientText('YouTube channel "{title}" authorized successfully!', {
          title: decodeURIComponent(channelTitle),
        });
        window.location.href = `/dashboard/accounts?success=true&platform=youtube&message=${encodeURIComponent(message)}`;
      } else {
        window.location.href = '/dashboard/accounts';
      }
    }
  }, [searchParams]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4">
        <div className="inline-block h-12 w-12 animate-spin rounded-full border-4 border-solid border-current border-r-transparent mb-4" />
        <p className="text-lg text-gray-600">{translateClientText('Processing authorization callback...')}</p>
        <p className="text-sm text-gray-500 mt-2">{translateClientText('This window will close automatically')}</p>
      </div>
    </div>
  );
}

export default function YouTubeCallbackPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center space-y-4">
          <div className="inline-block h-12 w-12 animate-spin rounded-full border-4 border-solid border-current border-r-transparent mb-4" />
          <p className="text-lg text-gray-600">{translateClientText('Loading...')}</p>
        </div>
      </div>
    }>
      <YouTubeCallbackContent />
    </Suspense>
  );
}
