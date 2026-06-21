'use client';

import { useEffect, useRef, useState } from 'react';
import { CheckCircle, XCircle } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';

interface TrackStep {
  step_name: string;
  step_order: number;
  status: string;
  error_msg?: string;
  duration?: number;
  progress_percent?: number;
  progress_text?: string;
}

interface DoneEvent {
  type: 'done';
  video_status: string;
  bili_bvid?: string;
}

type SSEPayload = TrackStep | DoneEvent;

const STEP_LABEL_KEYS: Record<string, string> = {
  Initialize: 'Initialize',
  DownloadVideo: 'Download video',
  DownloadThumbnail: 'Download thumbnail',
  ExtractAudio: 'Extract audio',
  Transcribe: 'Transcribe subtitles',
  LLMTranslate: 'AI translation',
  SynthesizeSubtitleAudio: 'Subtitle voiceover',
  SaveDatabase: 'Save results',
};

function StepIcon({ status }: { status: string }) {
  if (status === 'completed') return <CheckCircle className="w-3.5 h-3.5 text-green-500 shrink-0" />;
  if (status === 'failed')    return <XCircle    className="w-3.5 h-3.5 text-red-500 shrink-0" />;
  if (status === 'running')   return <div className="w-3.5 h-3.5 shrink-0 rounded-full border-2 border-blue-500 border-t-transparent animate-spin" />;
  if (status === 'skipped')   return <div className="w-3.5 h-3.5 shrink-0 rounded-full bg-gray-300" />;
  return <div className="w-3.5 h-3.5 shrink-0 rounded-full border-2 border-gray-300" />;
}

function upsertStep(prev: TrackStep[], next: TrackStep): TrackStep[] {
  const idx = prev.findIndex(s => s.step_name === next.step_name);
  if (idx === -1) return [...prev, next].sort((a, b) => a.step_order - b.step_order);
  const updated = [...prev];
  updated[idx] = next;
  return updated;
}

interface Props {
  videoId: string;
  onStatusChange?: (payload: { status: 'tracking' | 'completed' | 'failed'; biliBvid?: string }) => void;
}

export default function InlineTaskTracker({ videoId, onStatusChange }: Props) {
  const [steps, setSteps]           = useState<TrackStep[]>([]);
  const [videoStatus, setVideoStatus] = useState('');
  const [biliBvid, setBiliBvid]     = useState('');
  const [connError, setConnError]   = useState('');
  const esRef = useRef<EventSource | null>(null);
  const { t } = useI18n();

  useEffect(() => {
    if (!videoId) return;

    onStatusChange?.({ status: 'tracking' });

    let mounted = true;
    const es = new EventSource(`/api/v1/videos/${videoId}/events`);
    esRef.current = es;

    es.onmessage = (e: MessageEvent<string>) => {
      if (!mounted) return;
      try {
        const payload = JSON.parse(e.data) as SSEPayload;
        if ('type' in payload && payload.type === 'done') {
          setVideoStatus(payload.video_status);
          if (payload.bili_bvid) setBiliBvid(payload.bili_bvid);
          onStatusChange?.({
            status: payload.video_status === '004' || payload.video_status === 'failed' ? 'failed' : 'completed',
            biliBvid: payload.bili_bvid,
          });
          es.close();
          return;
        }
        setSteps(prev => upsertStep(prev, payload as TrackStep));
      } catch {
        // ignore malformed events
      }
    };

    es.onerror = () => {
      es.close();
      if (mounted) {
        setConnError('Connection failed. Refresh and try again.');
        onStatusChange?.({ status: 'failed' });
      }
    };

    return () => {
      mounted = false;
      es.close();
    };
  }, [onStatusChange, videoId]);

  const allStepsFinished = steps.length > 0 && steps.every(
    (step) => step.status === 'completed' || step.status === 'skipped',
  );
  const isDone   = videoStatus === '003' || videoStatus === 'completed' || allStepsFinished;
  const isFailed = videoStatus === '004' || videoStatus === 'failed';
  const completedCount = steps.filter(s => s.status === 'completed' || s.status === 'skipped').length;
  const runningStep = steps.find(step => step.status === 'running');
  const progress = steps.length > 0
    ? Math.round((((completedCount) + ((runningStep?.progress_percent ?? 0) / 100)) / steps.length) * 100)
    : 0;

  useEffect(() => {
    if (isDone) {
      onStatusChange?.({ status: 'completed', biliBvid });
      return;
    }

    if (isFailed) {
      onStatusChange?.({ status: 'failed', biliBvid });
    }
  }, [biliBvid, isDone, isFailed, onStatusChange]);

  if (steps.length === 0) {
    if (connError) {
      return (
        <div className="mt-2 flex items-center gap-2 text-xs text-red-500">
          <XCircle className="w-3 h-3 shrink-0" />
          <span>{t(connError)}</span>
        </div>
      );
    }
    return (
      <div className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
        <div className="w-3 h-3 rounded-full border-2 border-blue-400 border-t-transparent animate-spin" />
        <span>{t('Initializing task...')}</span>
      </div>
    );
  }

  return (
    <div className="mt-3 rounded-lg border border-border bg-background/60 overflow-hidden text-xs">
      {/* Header */}
      <div className="px-3 py-2 flex items-center gap-2 border-b border-border">
        {isDone ? (
          <><CheckCircle className="w-3.5 h-3.5 text-green-500" /><span className="font-medium text-green-700">{t('Processing completed')}</span></>
        ) : isFailed ? (
          <><XCircle className="w-3.5 h-3.5 text-red-500" /><span className="font-medium text-red-700">{t('Processing failed')}</span></>
        ) : (
          <><div className="w-3 h-3 rounded-full border-2 border-blue-500 border-t-transparent animate-spin" /><span className="font-medium text-blue-700">{t('Processing')}</span></>
        )}
        <span className="ml-auto text-muted-foreground">{completedCount}/{steps.length}</span>
      </div>
      {/* Progress bar */}
      <div className="h-1 bg-muted">
        <div
          className={`h-1 transition-all duration-500 ${
            isDone ? 'bg-green-500' : isFailed ? 'bg-red-400' : 'bg-blue-500'
          }`}
          style={{ width: `${progress}%` }}
        />
      </div>
      {/* Steps */}
      <div className="px-3 py-2 space-y-1.5">
        {steps.map(step => (
          <div key={step.step_name} className="space-y-1">
            <div className="flex items-center gap-2">
              <StepIcon status={step.status} />
              <span className={step.status === 'pending' ? 'text-muted-foreground' : 'text-foreground'}>
                {t(STEP_LABEL_KEYS[step.step_name] ?? step.step_name)}
              </span>
              {step.status === 'running' && typeof step.progress_percent === 'number' && (
                <span className="ml-auto text-blue-600">{step.progress_percent}%</span>
              )}
              {step.status === 'completed' && step.duration != null && step.duration > 0 && (
                <span className="ml-auto text-muted-foreground">{(step.duration / 1000).toFixed(1)}s</span>
              )}
              {step.status === 'failed' && step.error_msg && (
                <span className="ml-auto text-red-500 truncate max-w-[160px]" title={step.error_msg}>{step.error_msg}</span>
              )}
            </div>
            {step.status === 'running' && step.progress_text && (
              <p className="pl-5 text-[11px] text-blue-700">{step.progress_text}</p>
            )}
          </div>
        ))}
      </div>
      {/* Footer links */}
      <div className="px-3 py-2 border-t border-border flex items-center gap-4">
        <a href="/dashboard/tasks" className="text-blue-500 hover:underline">{t('View task queue')} →</a>
        {biliBvid && (
          <a
            href={`https://www.bilibili.com/video/${biliBvid}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-pink-500 hover:underline"
          >
            {t('View Bilibili video')} ↗
          </a>
        )}
      </div>
    </div>
  );
}