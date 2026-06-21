'use client';

import { Suspense, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import { getBackendBaseUrl } from '@/lib/backend-url';

/**
 * YouTube OAuth 回调中转页
 * 后端回调后重定向到此页面，此页面通过 postMessage 通知父窗口后自动关闭。
 */
function YouTubeCallbackInner() {
  const searchParams = useSearchParams();

  useEffect(() => {
    const success = searchParams.get('success');
    const error = searchParams.get('error');
    const username = searchParams.get('username');
    const platform = searchParams.get('platform');
    const transferToken = searchParams.get('transfer_token');

    const postToOpener = (payload: Record<string, string>) => {
      if (window.opener && !window.opener.closed) {
        window.opener.postMessage(payload, window.location.origin);
        window.close();
        return true;
      }
      return false;
    };

    const redirectToAccounts = (query: string) => {
      window.location.replace(`/dashboard/accounts${query}`);
    };

    if (transferToken) {
      let cancelled = false;

      const completeAuthorization = async () => {
        try {
          const response = await fetch(`${getBackendBaseUrl()}/api/v1/bindings/youtube/complete`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ transfer_token: transferToken }),
          });
          const result = await response.json();

          if (cancelled) {
            return;
          }

          if (response.ok && result?.code === 200) {
            const resolvedUsername = result?.data?.username || 'YouTube';
            const resolvedPlatform = result?.data?.platform || 'youtube';
            const message = `${resolvedUsername} 账号绑定成功！`;

            if (
              postToOpener({
                type: 'YOUTUBE_AUTH_SUCCESS',
                message,
                username: resolvedUsername,
                platform: resolvedPlatform,
              })
            ) {
              return;
            }

            redirectToAccounts(
              `?success=true&platform=${encodeURIComponent(resolvedPlatform)}&message=${encodeURIComponent(message)}`
            );
            return;
          }

          const errMessage = result?.message || 'bridge_complete_failed';
          if (postToOpener({ type: 'YOUTUBE_AUTH_ERROR', error: errMessage })) {
            return;
          }
          redirectToAccounts(`?error=${encodeURIComponent(errMessage)}`);
        } catch (requestError) {
          if (cancelled) {
            return;
          }

          const errMessage =
            requestError instanceof Error ? requestError.message : 'bridge_complete_failed';

          if (postToOpener({ type: 'YOUTUBE_AUTH_ERROR', error: errMessage })) {
            return;
          }
          redirectToAccounts(`?error=${encodeURIComponent(errMessage)}`);
        }
      };

      void completeAuthorization();

      return () => {
        cancelled = true;
      };
    }

    if (window.opener && !window.opener.closed) {
      if (success === 'true') {
        window.opener.postMessage(
          {
            type: 'YOUTUBE_AUTH_SUCCESS',
            message: `${username ? decodeURIComponent(username) : 'YouTube'} 账号绑定成功！`,
            username: username ? decodeURIComponent(username) : '',
            platform,
          },
          window.location.origin
        );
      } else {
        window.opener.postMessage(
          {
            type: 'YOUTUBE_AUTH_ERROR',
            error: error || 'unknown_error',
          },
          window.location.origin
        );
      }
      window.close();
    } else {
      // 非弹窗场景（用户直接访问或弹窗已关闭），跳转回账号页
      const qs = window.location.search;
      window.location.replace(`/dashboard/accounts${qs}`);
    }
  }, [searchParams]);

  return (
    <div className="flex items-center justify-center min-h-screen bg-white">
      <div className="text-center space-y-3">
        <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-blue-500 border-r-transparent" />
        <p className="text-gray-500 text-sm">正在完成 YouTube 授权，请稍候...</p>
      </div>
    </div>
  );
}

export default function YouTubeCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center min-h-screen bg-white">
          <div className="text-center space-y-3">
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-blue-500 border-r-transparent" />
            <p className="text-gray-500 text-sm">正在完成 YouTube 授权，请稍候...</p>
          </div>
        </div>
      }
    >
      <YouTubeCallbackInner />
    </Suspense>
  );
}
