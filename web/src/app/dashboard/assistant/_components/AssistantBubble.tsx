'use client';

import { useState } from 'react';
import { ChevronDown, ChevronRight, CheckCircle, Trash2, XCircle, Wrench } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';
import { AgentStep } from '@/lib/api/agent';
import InlineTaskTracker from './InlineTaskTracker';

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  steps?: AgentStep[];
  execution_ms?: number;
  success?: boolean;
  isError?: boolean;
  videoId?: string;
  biliBvid?: string;
  trackingStatus?: 'pending' | 'tracking' | 'completed' | 'failed';
}

function StaticTrackingSummary({ message }: { message: Message }) {
  const { t } = useI18n();

  if (message.trackingStatus !== 'completed' && message.trackingStatus !== 'failed') {
    return null;
  }

  const isCompleted = message.trackingStatus === 'completed';
  return (
    <div className={`rounded-lg border px-3 py-2 text-xs ${isCompleted ? 'border-green-200 bg-green-50 text-green-700' : 'border-red-200 bg-red-50 text-red-600'}`}>
      <div className="flex items-center gap-2">
        {isCompleted ? <CheckCircle className="h-3.5 w-3.5 shrink-0" /> : <XCircle className="h-3.5 w-3.5 shrink-0" />}
        <span className="font-medium">{isCompleted ? t('This historical task has completed') : t('This historical task has ended or failed')}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-3">
        {message.videoId ? (
          <a href="/dashboard/tasks" className="hover:underline">
            {t('View task queue')} →
          </a>
        ) : null}
        {message.biliBvid ? (
          <a
            href={`https://www.bilibili.com/video/${message.biliBvid}`}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:underline"
          >
            {t('Uploaded to Bilibili: {bvid}', { bvid: message.biliBvid })} ↗
          </a>
        ) : null}
      </div>
    </div>
  );
}

function StepItem({ step, index }: { step: AgentStep; index: number }) {
  const [open, setOpen] = useState(false);
  const hasError = Boolean(step.error);
  const { t } = useI18n();

  return (
    <div className="border border-border rounded-lg overflow-hidden text-xs">
      <button
        onClick={() => setOpen(v => !v)}
        className="w-full flex items-center gap-2 px-3 py-2 bg-muted/50 hover:bg-muted transition-colors text-left"
      >
        {hasError
          ? <XCircle className="h-3.5 w-3.5 text-destructive shrink-0" />
          : <CheckCircle className="h-3.5 w-3.5 text-green-500 shrink-0" />}
        <Wrench className="h-3 w-3 text-muted-foreground shrink-0" />
        <span className="font-mono font-medium">{step.tool}</span>
        <span className="text-muted-foreground ml-auto">{t('Step {index}', { index: index + 1 })}</span>
        {open ? <ChevronDown className="h-3 w-3 shrink-0" /> : <ChevronRight className="h-3 w-3 shrink-0" />}
      </button>
      {open && (
        <div className="px-3 py-2 space-y-2 bg-background">
          {step.arguments && (
            <div>
              <p className="text-muted-foreground mb-1">{t('Arguments')}</p>
              <pre className="bg-muted rounded p-2 overflow-x-auto whitespace-pre-wrap break-all">{step.arguments}</pre>
            </div>
          )}
          {step.output && (
            <div>
              <p className="text-muted-foreground mb-1">{t('Output')}</p>
              <pre className="bg-muted rounded p-2 overflow-x-auto whitespace-pre-wrap break-all">{step.output}</pre>
            </div>
          )}
          {step.error && (
            <div>
              <p className="text-destructive mb-1">{t('Error')}</p>
              <pre className="bg-destructive/10 text-destructive rounded p-2 overflow-x-auto whitespace-pre-wrap break-all">{step.error}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function AssistantBubble({
  message,
  onTrackingStatusChange,
  onDelete,
  canDelete = true,
}: {
  message: Message;
  onTrackingStatusChange?: (messageId: string, payload: { status: 'tracking' | 'completed' | 'failed'; biliBvid?: string }) => void;
  onDelete?: (messageId: string) => void;
  canDelete?: boolean;
}) {
  const [stepsOpen, setStepsOpen] = useState(false);
  const hasSteps = message.steps && message.steps.length > 0;
  const { locale, t } = useI18n();

  return (
    <div className="group relative bg-muted rounded-2xl px-4 py-3 max-w-[80%] space-y-2">
      {canDelete && onDelete ? (
        <button
          type="button"
          onClick={() => onDelete(message.id)}
          className="absolute right-3 top-3 rounded-md p-1 text-muted-foreground opacity-0 transition hover:bg-background/80 hover:text-destructive group-hover:opacity-100"
          title={t('Delete this history message')}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      ) : null}

      <p className="whitespace-pre-wrap">{message.content}</p>

      {message.trackingStatus === 'pending' && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <div className="w-3 h-3 rounded-full border-2 border-blue-400 border-t-transparent animate-spin" />
          <span>{t('Submitting to the processing queue...')}</span>
        </div>
      )}

      <StaticTrackingSummary message={message} />

      {message.videoId && message.trackingStatus === 'tracking' && (
        <InlineTaskTracker
          videoId={message.videoId}
          onStatusChange={(payload) => onTrackingStatusChange?.(message.id, payload)}
        />
      )}

      {hasSteps && (
        <div className="pt-1 border-t border-border">
          <button
            onClick={() => setStepsOpen(v => !v)}
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            {stepsOpen ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
            <span>{t('Tool calls ({count} steps)', { count: message.steps!.length })}</span>
            {message.execution_ms !== undefined && (
              <span className="ml-2 text-muted-foreground/70">{message.execution_ms} ms</span>
            )}
          </button>
          {stepsOpen && (
            <div className="mt-2 space-y-1.5">
              {message.steps!.map((step, i) => (
                <StepItem key={i} step={step} index={i} />
              ))}
            </div>
          )}
        </div>
      )}

      <p className="text-xs text-muted-foreground">
        {message.timestamp.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })}
      </p>
    </div>
  );
}