import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useI18n } from '@/contexts/I18nContext';
import { agentApi } from '@/lib/api/agent';
import {
  DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
  inspectVideoSubmissionUrl,
  submitVideoToQueue,
  type PlaylistSubmissionConfig,
} from '@/lib/video-submission';
import type { Message } from '../AssistantBubble';

const WELCOME_MESSAGE_ID = 'welcome';

function createWelcomeMessage(t: (key: string, params?: Record<string, string | number>) => string): Message {
  return {
    id: WELCOME_MESSAGE_ID,
    role: 'assistant',
    content: t('Hello! I am your AI assistant. I can help you with the following tasks:\n\n**Video processing**\n• 🚀 Submit YouTube videos, YouTube playlists, or Douyin videos to the full processing pipeline in one step (download → transcribe → translate → generate metadata)\n• ⬇️ Download videos, extract audio, or download covers separately\n\n**Video library management**\n• 🔍 Query the video library and filter by status or platform\n• ✏️ Regenerate Bilibili titles, descriptions, and tags with AI\n• 📝 Summarize video content or translate subtitles into other languages\n\n**Subscription management**\n• 📺 List, add, or remove channel subscriptions\n\nTell me what you want to do, or send a YouTube video link, playlist link, or Douyin share link!'),
    timestamp: new Date(),
  };
}

const MAX_HISTORY_MESSAGES = 100;

function normalizeCandidateURL(raw: string): string {
  return raw.trim().replace(/[),.!?\]】>》"'，。；;！？]+$/u, '');
}

function extractSupportedVideoURL(text: string, playlistSubmissionConfig: PlaylistSubmissionConfig): string | null {
  const matches = text.match(/https?:\/\/[^\s]+/ig) ?? [];
  for (const match of matches) {
    const normalized = normalizeCandidateURL(match);
    if (inspectVideoSubmissionUrl(normalized, playlistSubmissionConfig).isSupported) {
      return normalized;
    }
  }
  return null;
}

function normalizeMessages(messages: Message[]): Message[] {
  const trimmed = messages
    .filter((message) => message.id !== WELCOME_MESSAGE_ID)
    .slice(-MAX_HISTORY_MESSAGES);

  return trimmed;
}

export function useChatMessages(
  userId: string,
  playlistSubmissionConfig: PlaylistSubmissionConfig = DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
) {
  const { t } = useI18n();
  const welcomeMessage = useMemo(() => createWelcomeMessage(t), [t]);
  const [messages, setMessages] = useState<Message[]>([welcomeMessage]);
  const [isLoading, setIsLoading] = useState(false);
  const messagesRef = useRef<Message[]>([welcomeMessage]);

  const syncMessages = useCallback((nextMessages: Message[]) => {
    messagesRef.current = nextMessages;
    setMessages(nextMessages);
  }, []);

  const applyMessages = useCallback((updater: (prev: Message[]) => Message[]) => {
    const normalizedMessages = normalizeMessages(updater(messagesRef.current));
    const nextMessages = normalizedMessages.length > 0 ? normalizedMessages : [welcomeMessage];
    syncMessages(nextMessages);
    return nextMessages;
  }, [syncMessages, welcomeMessage]);

  const updateTrackingStatus = useCallback((messageId: string, payload: { status: 'tracking' | 'completed' | 'failed'; biliBvid?: string }) => {
    applyMessages((prev) => prev.map((message) => (
      message.id === messageId
        ? { ...message, trackingStatus: payload.status, biliBvid: payload.biliBvid ?? message.biliBvid }
        : message
    )));
  }, [applyMessages]);

  const deleteMessage = useCallback((messageId: string) => {
    applyMessages((prev) => prev.filter((message) => message.id !== messageId));
  }, [applyMessages]);

  useEffect(() => {
    syncMessages([welcomeMessage]);
  }, [syncMessages, userId, welcomeMessage]);

  const sendMessage = async (query: string, model?: string) => {
    if (!query.trim() || isLoading) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: query.trim(),
      timestamp: new Date(),
    };
    applyMessages((prev) => [...prev, userMessage]);

    // Supported video URL async path
    const videoUrl = extractSupportedVideoURL(query, playlistSubmissionConfig);
    if (videoUrl) {
      const pendingId = (Date.now() + 1).toString();
      applyMessages((prev) => [...prev, {
        id: pendingId,
        role: 'assistant' as const,
        content: t('Okay! Adding the video to the processing queue...\n\n🔗 {url}', { url: videoUrl }),
        timestamp: new Date(),
        trackingStatus: 'pending' as const,
      }]);
      try {
        const result = await submitVideoToQueue(videoUrl, userId, undefined, undefined, undefined, playlistSubmissionConfig);
        const { videoId, platform } = result;
        const localizedPlatformLabel = platform === 'douyin' ? t('Douyin') : 'YouTube';

        if (result.submissionMode === 'playlist') {
          applyMessages((prev) => prev.map((m) => m.id === pendingId ? {
            ...m,
            content: t('✅ The YouTube playlist has been added to the processing queue with {count} items.\nFirst task ID: {id}\n\n🔗 {url}', {
              count: result.submittedCount,
              id: videoId,
              url: videoUrl,
            }),
            videoId,
            trackingStatus: undefined,
          } : m));
          toast.success(t('Playlist queued successfully ({count} items).', { count: result.submittedCount }), { duration: 5000 });
        } else {
          applyMessages((prev) => prev.map((m) => m.id === pendingId ? {
            ...m,
            content: t('✅ The {platform} video has been added to the processing queue. Downloading and processing continue in the background, and you can keep working.\n\n🔗 {url}', {
              platform: localizedPlatformLabel,
              url: videoUrl,
            }),
            videoId,
            trackingStatus: 'tracking' as const,
          } : m));
          toast.success(t('{platform} task queued successfully. ID: {id}', { platform: localizedPlatformLabel, id: videoId }), { duration: 5000 });
        }
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : t('Submission failed');
        applyMessages((prev) => prev.map((m) => m.id === pendingId ? {
          ...m,
          content: t('⚠️ Submission failed: {message}', { message: msg }),
          isError: true,
          trackingStatus: undefined,
        } : m));
        toast.error(t('Submission failed: {message}', { message: msg }));
      }
      return;
    }

    // Normal agent path
    setIsLoading(true);
    try {
      const result = await agentApi.run(query, model);
      applyMessages((prev) => [...prev, {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: result.result || t('Task completed.'),
        timestamp: new Date(),
        steps: result.steps,
        execution_ms: result.execution_ms,
        success: result.success,
      }]);
    } catch (err: unknown) {
      let detail = t('Request failed. Please try again later.');
      let isCreditsError = false;

      if (err && typeof err === 'object') {
        // Axios-style error with response object
        const axiosErr = err as { response?: { status?: number; data?: { message?: string; error?: string } } };
        if (axiosErr.response?.status === 402) {
          isCreditsError = true;
          detail = t('Insufficient credits prevented the AI request. Visit the [unified pricing center](/membership?source=assistant-upsell&recommended=vip_month) to purchase a plan.');
        } else if ('response' in err) {
          detail = axiosErr.response?.data?.message ?? axiosErr.response?.data?.error ?? detail;
        }
      }

      applyMessages((prev) => [...prev, {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: isCreditsError
          ? t('💳 **Insufficient credits**\n\nAI requests consume credits, but your account balance is currently too low.\n\nVisit the [unified pricing center](/membership?source=assistant-upsell&recommended=vip_month) to purchase a plan and get more credits.')
          : `⚠️ ${detail}`,
        timestamp: new Date(),
        isError: true,
      }]);

      if (isCreditsError) {
        toast.error(t('Insufficient credits. Please top up and try again.'));
      }
    } finally {
      setIsLoading(false);
    }
  };

  return {
    messages,
    isLoading,
    sendMessage,
    updateTrackingStatus,
    deleteMessage,
  };
}
