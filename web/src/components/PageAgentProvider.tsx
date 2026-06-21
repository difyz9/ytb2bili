'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useUserSettings } from '@/hooks/useUserSettings';
import {
  DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
  inspectVideoSubmissionUrl,
  parsePlaylistSubmissionConfig,
  submitVideoToQueue,
  USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG,
} from '@/lib/video-submission';
import { agentApi } from '@/lib/api/agent';

// ── 系统级知识 ──────────────────────────────────────────────────────────────────
const SYSTEM_INSTRUCTIONS = `
你是 ytb2bili 平台的智能助手，帮助用户完成 YouTube、YouTube 播放列表或抖音视频下载、AI 翻译和 Bilibili 上传等工作流。

平台导航结构：
- /dashboard          首页概览
- /dashboard/assistant   AI 助手（对话式入口）
- /dashboard/videos      视频列表（已处理视频）
- /dashboard/tasks       任务队列（实时进度）
- /dashboard/subscribe   频道订阅管理
- /dashboard/accounts    账号管理（YouTube / Bilibili 绑定）
- /dashboard/extension   浏览器插件下载
- /dashboard/settings    系统设置

操作原则：
- 提交视频前先确认是有效的 YouTube 视频链接、YouTube 播放列表链接或抖音分享链接；当前快速提交流程不支持直接提交 Bilibili 链接
- 账号绑定操作涉及 OAuth，需引导用户完成授权流程
- 任务处理中的步骤：下载 → 提取音频 → 转录 → AI 翻译 → 合成语音 → 上传 Bilibili
- 遇到错误时读取页面上的错误信息后再处理，不要盲目重试
`.trim();

// ── 消息类型 ───────────────────────────────────────────────────────────────────
interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: number;
}

interface ActivityState {
  type: 'thinking' | 'executing' | 'executed' | 'retrying' | 'error';
  tool?: string;
  message?: string;
}

const QUICK_SUGGESTIONS = [
  { label: '📋 查看视频列表', value: '帮我查看最近处理的视频列表' },
  { label: '📤 提交新视频', value: '我想提交一个 YouTube 或抖音视频到处理队列' },
  { label: '📊 检查任务状态', value: '帮我查看当前正在处理中的任务' },
  { label: '🔗 绑定账号', value: '帮我查看账号绑定状态' },
];

// ── 机器人图标 ─────────────────────────────────────────────────────────────────
function RobotIcon({ className = '' }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect x="16" y="20" width="32" height="28" rx="6" fill="currentColor" opacity="0.9" />
      <rect x="22" y="27" width="8" height="8" rx="2" fill="white" opacity="0.9" />
      <rect x="34" y="27" width="8" height="8" rx="2" fill="white" opacity="0.9" />
      <rect x="24" y="38" width="16" height="3" rx="1.5" fill="white" opacity="0.7" />
      <rect x="29" y="14" width="6" height="8" rx="3" fill="currentColor" opacity="0.9" />
      <circle cx="32" cy="13" r="3" fill="currentColor" opacity="0.9" />
      <rect x="8" y="28" width="6" height="12" rx="3" fill="currentColor" opacity="0.7" />
      <rect x="50" y="28" width="6" height="12" rx="3" fill="currentColor" opacity="0.7" />
    </svg>
  );
}

// ── Provider 组件 ──────────────────────────────────────────────────────────────
export default function PageAgentProvider() {
  const { currentUser } = useAuth();
  const abortControllerRef = useRef<AbortController | null>(null);

  const [isOpen, setIsOpen] = useState(false);
  const [status, setStatus] = useState<'idle' | 'running' | 'completed' | 'error'>('idle');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [activity, setActivity] = useState<ActivityState | null>(null);
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const isRunning = status === 'running';
  const userId = currentUser?.id ?? '';
  const { settings } = useUserSettings(userId);
  const playlistSubmissionConfig = parsePlaylistSubmissionConfig(settings[USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG]) ?? DEFAULT_PLAYLIST_SUBMISSION_CONFIG;

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, activity, scrollToBottom]);

  const sendMessage = useCallback(async (text: string) => {
    const trimmed = text.trim();
    if (!trimmed || isRunning) return;

    const userMsg: ChatMessage = {
      id: Date.now().toString(),
      role: 'user',
      content: trimmed,
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, userMsg]);
    setInput('');
    setStatus('running');
    setActivity({ type: 'thinking', message: '正在分析请求…' });

    try {
      const inspection = inspectVideoSubmissionUrl(trimmed, playlistSubmissionConfig);
      let content = '';

      if (inspection.isSupported) {
        setActivity({ type: 'executing', tool: 'submit_video_pipeline', message: '正在提交视频到处理队列…' });
        const result = await submitVideoToQueue(trimmed, userId, undefined, undefined, undefined, playlistSubmissionConfig);
        if (result.submissionMode === 'playlist') {
          content = `✅ 已成功将 YouTube 播放列表加入处理队列，共 ${result.submittedCount} 条。\n首个任务 ID：${result.videoId}\n请前往 /dashboard/tasks 查看批量任务进度。`;
        } else {
          const platformLabel = result.platform === 'douyin' ? '抖音' : 'YouTube';
          content = `✅ 已成功将${platformLabel}视频加入处理队列，视频 ID：${result.videoId}\n请前往 /dashboard/tasks 查看实时进度。`;
        }
      } else {
        setActivity({ type: 'executing', tool: 'query_backend_agent', message: '正在请求后端 AI 助手…' });
        const controller = new AbortController();
        abortControllerRef.current = controller;
        const result = await agentApi.run(`${SYSTEM_INSTRUCTIONS}\n\n用户问题：${trimmed}`, undefined, controller.signal);
        content = result.result || '✅ 任务完成';
      }

      const assistantMsg: ChatMessage = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content,
        timestamp: Date.now(),
      };
      setMessages((prev) => [...prev, assistantMsg]);
      setStatus('completed');
    } catch (err) {
      const errMsg: ChatMessage = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
	        content: err instanceof DOMException && err.name === 'AbortError'
	          ? '⏹️ 当前请求已停止。'
	          : `❌ 发生错误：${err instanceof Error ? err.message : String(err)}`,
        timestamp: Date.now(),
      };
      setMessages((prev) => [...prev, errMsg]);
	      setStatus('error');
	    } finally {
	      abortControllerRef.current = null;
	      setActivity(null);
    }
  }, [isRunning, playlistSubmissionConfig, userId]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage(input);
      }
    },
    [input, sendMessage],
  );

  const activityLabel: Record<string, string> = {
    thinking: '思考中…',
    executing: '执行操作…',
    executed: '操作完成',
    retrying: '重试中…',
    error: '发生错误',
  };

  return (
    <>
      {/* ── 悬浮机器人按钮 ── */}
      <button
        onClick={() => setIsOpen((v) => !v)}
        aria-label="打开 AI 助手"
        className="fixed right-6 bottom-8 z-50 flex h-14 w-14 items-center justify-center rounded-full shadow-2xl transition-all duration-300 hover:scale-110 active:scale-95"
        style={{
          background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #a855f7 100%)',
          boxShadow: '0 8px 32px rgba(99, 102, 241, 0.45)',
        }}
      >
        <RobotIcon className="h-8 w-8 text-white" />
        {isRunning && (
          <span className="absolute -top-1 -right-1 h-4 w-4 rounded-full bg-green-400 border-2 border-white animate-pulse" />
        )}
      </button>

      {/* ── 聊天面板 ── */}
      <div
        className={`fixed right-6 bottom-24 z-50 flex flex-col overflow-hidden rounded-2xl shadow-2xl transition-all duration-300 ${
          isOpen ? 'opacity-100 translate-y-0 pointer-events-auto' : 'opacity-0 translate-y-4 pointer-events-none'
        }`}
        style={{ width: 380, maxHeight: 'calc(100vh - 120px)' }}
      >
        {/* 渐变头部 */}
        <div
          className="flex items-center gap-3 px-5 py-4 flex-shrink-0"
          style={{ background: 'linear-gradient(135deg, #4f46e5 0%, #7c3aed 60%, #9333ea 100%)' }}
        >
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-white/20 flex-shrink-0">
            <RobotIcon className="h-7 w-7 text-white" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-white font-semibold text-sm leading-tight">ytb2bili AI 助手</p>
            <p className="text-indigo-200 text-xs mt-0.5">
              {isRunning ? (
                <span className="flex items-center gap-1">
                  <span className="h-1.5 w-1.5 rounded-full bg-green-300 animate-pulse inline-block" />
                  处理中…
                </span>
              ) : (
                '随时为您服务'
              )}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {isRunning && (
              <button
                onClick={() => abortControllerRef.current?.abort()}
                className="text-white/70 hover:text-white transition-colors text-xs px-2 py-1 rounded-lg bg-white/10 hover:bg-white/20"
              >
                停止
              </button>
            )}
            <button
              onClick={() => setIsOpen(false)}
              className="text-white/70 hover:text-white transition-colors p-1 rounded-lg hover:bg-white/10"
              aria-label="关闭"
            >
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* 消息区 */}
        <div className="flex-1 overflow-y-auto bg-gray-50 dark:bg-gray-900 px-4 py-3 space-y-3 min-h-0">
          {messages.length === 0 && !isRunning && (
            <>
              {/* 欢迎语 */}
              <div className="text-center py-4">
                <div
                  className="inline-flex h-16 w-16 items-center justify-center rounded-2xl mb-3"
                  style={{ background: 'linear-gradient(135deg, #6366f1 0%, #a855f7 100%)' }}
                >
                  <RobotIcon className="h-10 w-10 text-white" />
                </div>
                <p className="text-gray-700 dark:text-gray-200 font-medium text-sm">你好，我是 AI 助手</p>
                <p className="text-gray-400 text-xs mt-1">有什么可以帮您？</p>
              </div>
              {/* 快捷建议 */}
              <div className="grid grid-cols-2 gap-2">
                {QUICK_SUGGESTIONS.map((s) => (
                  <button
                    key={s.value}
                    onClick={() => sendMessage(s.value)}
                    className="text-left rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2.5 text-xs text-gray-600 dark:text-gray-300 hover:border-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 transition-colors leading-snug"
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            </>
          )}

          {/* 历史消息 */}
          {messages.map((msg) => (
            <div
              key={msg.id}
              className={`flex gap-2 ${msg.role === 'user' ? 'flex-row-reverse' : 'flex-row'}`}
            >
              {msg.role !== 'user' && (
                <div
                  className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full mt-1"
                  style={{ background: 'linear-gradient(135deg, #6366f1, #a855f7)' }}
                >
                  <RobotIcon className="h-4 w-4 text-white" />
                </div>
              )}
              <div
                className={`max-w-[80%] rounded-2xl px-3.5 py-2.5 text-sm leading-relaxed whitespace-pre-wrap break-words ${
                  msg.role === 'user'
                    ? 'bg-indigo-600 text-white rounded-tr-sm'
                    : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 shadow-sm border border-gray-100 dark:border-gray-700 rounded-tl-sm'
                }`}
              >
                {msg.content}
              </div>
            </div>
          ))}

          {/* 活动状态气泡 */}
          {isRunning && activity && (
            <div className="flex gap-2 flex-row">
              <div
                className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full mt-1"
                style={{ background: 'linear-gradient(135deg, #6366f1, #a855f7)' }}
              >
                <RobotIcon className="h-4 w-4 text-white" />
              </div>
              <div className="max-w-[80%] rounded-2xl rounded-tl-sm bg-white dark:bg-gray-800 shadow-sm border border-gray-100 dark:border-gray-700 px-3.5 py-2.5">
                <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <span className="flex gap-0.5">
                    {[0, 1, 2].map((i) => (
                      <span
                        key={i}
                        className="h-1.5 w-1.5 rounded-full bg-indigo-400 animate-bounce"
                        style={{ animationDelay: `${i * 150}ms` }}
                      />
                    ))}
                  </span>
                  <span>{activityLabel[activity.type] ?? activity.type}</span>
                  {activity.tool && (
                    <span className="text-indigo-500 font-medium">· {activity.tool}</span>
                  )}
                </div>
                {activity.message && (
                  <p className="mt-1 text-xs text-gray-600 dark:text-gray-300 leading-relaxed">{activity.message}</p>
                )}
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* 输入区 */}
        <div className="flex-shrink-0 bg-white dark:bg-gray-900 border-t border-gray-100 dark:border-gray-800 p-3">
          <div className="flex items-end gap-2 rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 focus-within:border-indigo-400 transition-colors">
            <textarea
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={isRunning ? 'AI 正在处理中…' : '输入您的需求，Enter 发送'}
              disabled={isRunning}
              rows={1}
              className="flex-1 resize-none bg-transparent text-sm text-gray-700 dark:text-gray-200 placeholder-gray-400 outline-none min-h-[24px] max-h-[96px] leading-6 disabled:opacity-50"
              style={{ overflowY: input.split('\n').length > 1 ? 'auto' : 'hidden' }}
            />
            <button
              onClick={() => sendMessage(input)}
              disabled={isRunning || !input.trim()}
              className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg transition-all disabled:opacity-40 disabled:cursor-not-allowed"
              style={{
                background: input.trim() && !isRunning
                  ? 'linear-gradient(135deg, #6366f1, #a855f7)'
                  : undefined,
                backgroundColor: !input.trim() || isRunning ? '#e5e7eb' : undefined,
              }}
              aria-label="发送"
            >
              <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19V5m0 0l-7 7m7-7l7 7" />
              </svg>
            </button>
          </div>
          <p className="mt-1.5 text-center text-[10px] text-gray-400">
            由 AI 驱动 · 可自动操作页面完成任务
          </p>
        </div>
      </div>
    </>
  );
}
