'use client';

import { useState, useRef, useCallback, useEffect } from 'react';
import { useI18n } from '@/contexts/I18nContext';
import { useMembership } from '@/hooks/useMembership';
import CreditShortageCard from '@/components/membership/CreditShortageCard';
import UpgradePromptCard from '@/components/membership/UpgradePromptCard';
import {
  Upload,
  File,
  CheckCircle,
  XCircle,
  Loader2,
  AlertCircle,
  Play,
  Clock,
  ChevronDown,
  ChevronRight,
  ExternalLink,
} from 'lucide-react';

interface TaskStep {
  step_name: string;
  step_order: number;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
  duration: number;
  error_msg: string;
  can_retry: boolean;
}

interface VideoRecord {
  id: number;
  video_id: string;
  title: string;
  status: string;
  task_steps?: TaskStep[];
}

type Phase = 'idle' | 'uploading' | 'processing' | 'done' | 'error';

interface LocalVideoUploadProps {
  userId?: string;
}

const ALLOWED_EXTS = ['.mp4', '.avi', '.mov', '.mkv', '.flv', '.wmv', '.webm'];
const FULL_PIPELINE_REQUIRED_CREDITS = 50;
const STEP_LABEL_KEYS: Record<string, string> = {
  Initialize: 'Initialize',
  DownloadVideo: 'Download video',
  DownloadThumbnail: 'Download cover',
  ExtractAudio: 'Extract audio',
  Transcribe: 'Transcribe subtitles',
  LLMTranslate: 'AI translation',
  SynthesizeSubtitleAudio: 'Synthesize voice',
  SaveDatabase: 'Save database',
};
const STEP_STATUS_COLOR: Record<string, string> = {
  completed: 'text-green-600 bg-green-50 border-green-200',
  failed: 'text-red-600 bg-red-50 border-red-200',
  running: 'text-blue-600 bg-blue-50 border-blue-200',
  skipped: 'text-gray-400 bg-gray-50 border-gray-200',
  pending: 'text-gray-500 bg-gray-50 border-gray-200',
};

function formatDuration(ms: number) {
  if (!ms) return '';
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m${Math.round((ms % 60000) / 1000)}s`;
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
}

function StepList({ steps }: { steps: TaskStep[] }) {
  const { t } = useI18n();
  const sorted = [...steps].sort((a, b) => a.step_order - b.step_order);
  return (
    <ul className="mt-3 space-y-1.5">
      {sorted.map((step) => {
        const label = t(STEP_LABEL_KEYS[step.step_name] ?? step.step_name);
        const colorClass = STEP_STATUS_COLOR[step.status] ?? STEP_STATUS_COLOR.pending;
        return (
          <li key={step.step_name} className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm ${colorClass}`}>
            {step.status === 'completed' && <CheckCircle className="h-3.5 w-3.5 shrink-0" />}
            {step.status === 'failed' && <XCircle className="h-3.5 w-3.5 shrink-0" />}
            {step.status === 'running' && <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />}
            {step.status === 'skipped' && <span className="h-3.5 w-3.5 shrink-0 text-center text-xs">-</span>}
            {step.status === 'pending' && <Clock className="h-3.5 w-3.5 shrink-0" />}
            <span className="flex-1 font-medium">{label}</span>
            {step.duration > 0 ? <span className="text-xs opacity-70">{formatDuration(step.duration)}</span> : null}
            {step.error_msg ? (
              <span className="max-w-[140px] truncate text-xs opacity-80" title={step.error_msg}>
                {step.error_msg}
              </span>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}

export default function LocalVideoUpload({ userId }: LocalVideoUploadProps) {
  const { t } = useI18n();
  const { tier, credits } = useMembership();
  const [phase, setPhase] = useState<Phase>('idle');
  const [file, setFile] = useState<File | null>(null);
  const [uploadPct, setUploadPct] = useState(0);
  const [errorMsg, setErrorMsg] = useState('');
  const [insufficientCredits, setInsuffCredits] = useState(false);
  const [showUpgradePromptCard, setShowUpgradePromptCard] = useState(true);
  const [steps, setSteps] = useState<TaskStep[]>([]);
  const [stepsOpen, setStepsOpen] = useState(true);
  const [resultData, setResultData] = useState<{ audioPath: string; transcriptPath: string } | null>(null);

  const dropRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const startPolling = useCallback((vid: string) => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      try {
        const res = await fetch('/api/v1/videos?source_type=manual&page=1&size=20');
        const data = await res.json();
        if ((data.code === 0 || data.code === 200) && data.data?.videos) {
          const found: VideoRecord | undefined = data.data.videos.find((item: VideoRecord) => item.video_id === vid);
          if (found?.task_steps) setSteps(found.task_steps);
        }
      } catch {}
    }, 2000);
  }, []);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  useEffect(() => () => stopPolling(), [stopPolling]);

  const handleFileSelect = (selectedFile: File) => {
    const ext = `.${selectedFile.name.split('.').pop()?.toLowerCase()}`;
    if (!ALLOWED_EXTS.includes(ext)) {
      setErrorMsg(t('Unsupported format {ext}. Please upload one of: {exts}', { ext, exts: ALLOWED_EXTS.join(' / ') }));
      setPhase('error');
      return;
    }

    setFile(selectedFile);
    setPhase('idle');
    setErrorMsg('');
    setSteps([]);
    setResultData(null);
  };

  const onInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFile = e.target.files?.[0];
    if (selectedFile) handleFileSelect(selectedFile);
    e.target.value = '';
  };

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const selectedFile = e.dataTransfer.files?.[0];
    if (selectedFile) handleFileSelect(selectedFile);
  };

  const onDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  const handleProcess = async () => {
    if (!file) return;
    setPhase('uploading');
    setUploadPct(0);
    setErrorMsg('');
    setSteps([]);

    let videoPath = '';
    let uploadedVideoId = '';

    try {
      const result = await new Promise<{ videoPath: string; videoId: string }>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        const form = new FormData();
        form.append('file', file);

        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) setUploadPct(Math.round((e.loaded / e.total) * 100));
        };
        xhr.onload = () => {
          const data = JSON.parse(xhr.responseText);
          if (xhr.status === 200 && data.success) {
            resolve({ videoPath: data.video_path as string, videoId: (data.video_id as string) ?? '' });
          } else {
            reject(new Error(data.message || t('Upload failed')));
          }
        };
        xhr.onerror = () => reject(new Error(t('Network error')));
        xhr.open('POST', '/api/v1/video-process/upload');
        xhr.send(form);
      });
      videoPath = result.videoPath;
      uploadedVideoId = result.videoId;
    } catch (error: unknown) {
      setErrorMsg((error as Error).message ?? t('File upload failed'));
      setPhase('error');
      return;
    }

    setPhase('processing');
    const fname = videoPath.split('/').pop() ?? '';
    const derived = uploadedVideoId || (fname.includes('.') ? fname.substring(0, fname.lastIndexOf('.')) : fname);
    startPolling(derived);

    try {
      const res = await fetch('/api/v1/video-process/submit-video', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          video_path: videoPath,
          user_id: userId ?? '',
          title: file.name.replace(/\.[^/.]+$/, ''),
        }),
      });
      const data = await res.json();
      stopPolling();

      if (res.status === 402 || data.error === 'insufficient_credits') {
        setInsuffCredits(true);
        setErrorMsg(data.message || t('Insufficient credits. Please top up and try again.'));
        setPhase('error');
      } else if (data.success) {
        setShowUpgradePromptCard(true);
        setResultData({ audioPath: data.data?.audio_path ?? '', transcriptPath: data.data?.transcript?.srt_path ?? '' });
        setPhase('done');
        try {
          const refresh = await fetch('/api/v1/videos?source_type=manual&page=1&size=20');
          const refreshData = await refresh.json();
          const found = refreshData.data?.videos?.find((item: VideoRecord) => item.video_id === derived);
          if (found?.task_steps) setSteps(found.task_steps);
        } catch {}
      } else {
        setErrorMsg(data.message || t('Processing failed'));
        setPhase('error');
      }
    } catch (error: unknown) {
      stopPolling();
      setErrorMsg((error as Error).message ?? t('Processing request failed'));
      setPhase('error');
    }
  };

  const reset = () => {
    stopPolling();
    setFile(null);
    setPhase('idle');
    setUploadPct(0);
    setErrorMsg('');
    setInsuffCredits(false);
    setShowUpgradePromptCard(true);
    setSteps([]);
    setResultData(null);
  };

  const completedSteps = steps.filter((step) => step.status === 'completed').length;
  const totalSteps = steps.length;
  const progressPct = totalSteps > 0 ? Math.round((completedSteps / totalSteps) * 100) : 0;
  const shortageCredits = Math.max(FULL_PIPELINE_REQUIRED_CREDITS - credits.balance, 0);
  const successRecommendedPlanId = tier === 'free' ? 'vip_7d' : 'vip_month';

  return (
    <div className="space-y-4">
      {(phase === 'idle' || phase === 'error') ? (
        <div
          ref={dropRef}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onClick={() => inputRef.current?.click()}
          className="cursor-pointer rounded-xl border-2 border-dashed border-border p-8 text-center transition-colors hover:border-primary hover:bg-accent/40"
        >
          <input ref={inputRef} type="file" accept={ALLOWED_EXTS.join(',')} className="hidden" onChange={onInputChange} />
          <Upload className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
          {file ? (
            <div className="space-y-1">
              <p className="font-medium text-foreground">{file.name}</p>
              <p className="text-sm text-muted-foreground">{formatSize(file.size)}</p>
              <p className="mt-1 text-xs text-muted-foreground">{t('Click or drag to replace the file')}</p>
            </div>
          ) : (
            <div className="space-y-1">
              <p className="font-medium">{t('Drag a video file here')}</p>
              <p className="text-sm text-muted-foreground">{t('Or click to choose a file')}</p>
              <p className="mt-1 text-xs text-muted-foreground">{t('Supported: {exts}', { exts: ALLOWED_EXTS.join(' / ') })}</p>
            </div>
          )}
        </div>
      ) : null}

      {phase === 'error' && errorMsg ? (
        <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="flex-1">
            <span>{errorMsg}</span>
            {insufficientCredits ? (
              <CreditShortageCard
                className="mt-3"
                availableCredits={credits.balance}
                requiredCredits={FULL_PIPELINE_REQUIRED_CREDITS}
                serviceLabel="视频完整处理"
                recommendedPlanId={shortageCredits <= 20 ? 'vip_7d' : 'vip_month'}
              />
            ) : null}
          </div>
        </div>
      ) : null}

      {phase === 'uploading' ? (
        <div className="space-y-2 rounded-xl border border-border bg-muted/50 p-4">
          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span>{t('Uploading: {name}', { name: file?.name ?? '' })}</span>
            </div>
            <span className="font-mono text-xs">{uploadPct}%</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary transition-all duration-300" style={{ width: `${uploadPct}%` }} />
          </div>
        </div>
      ) : null}

      {(phase === 'processing' || phase === 'done') ? (
        <div className="overflow-hidden rounded-xl border border-border bg-muted/30">
          <div className="flex items-center gap-3 border-b border-border px-4 py-3">
            {phase === 'processing' ? (
              <Loader2 className="h-4 w-4 shrink-0 animate-spin text-blue-500" />
            ) : (
              <CheckCircle className="h-4 w-4 shrink-0 text-green-500" />
            )}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{file?.name}</p>
              <p className="text-xs text-muted-foreground">
                {phase === 'processing'
                  ? (totalSteps > 0
                    ? t('Task chain in progress... ({completed}/{total})', { completed: completedSteps, total: totalSteps })
                    : t('Task chain in progress...'))
                  : t('Processing completed')}
              </p>
            </div>
            {totalSteps > 0 ? <span className="shrink-0 font-mono text-xs text-muted-foreground">{progressPct}%</span> : null}
          </div>

          {totalSteps > 0 ? (
            <div className="h-1 bg-muted">
              <div className={`h-full transition-all duration-500 ${phase === 'done' ? 'bg-green-500' : 'bg-blue-500'}`} style={{ width: `${progressPct}%` }} />
            </div>
          ) : null}

          <button className="flex w-full items-center gap-2 px-4 py-2 text-xs text-muted-foreground transition-colors hover:text-foreground" onClick={() => setStepsOpen((open) => !open)}>
            {stepsOpen ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
            <span>{t('Task step details')}</span>
          </button>

          {stepsOpen && steps.length > 0 ? (
            <div className="px-4 pb-4">
              <StepList steps={steps} />
            </div>
          ) : null}

          {stepsOpen && steps.length === 0 && phase === 'processing' ? (
            <p className="px-4 pb-4 text-xs text-muted-foreground">{t('Waiting for the server to initialize task step records...')}</p>
          ) : null}
        </div>
      ) : null}

      {phase === 'done' && resultData ? (
        <div className="space-y-3">
          <div className="space-y-2 rounded-xl border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-950/20">
            <div className="flex items-center gap-2 font-medium text-green-700 dark:text-green-400">
              <CheckCircle className="h-4 w-4" />
              <span>{t('Processing completed')}</span>
            </div>
            {resultData.audioPath ? (
              <p className="text-xs text-muted-foreground">
                <span className="font-medium">{t('Audio:')}</span>
                {resultData.audioPath}
              </p>
            ) : null}
            {resultData.transcriptPath ? (
              <p className="text-xs text-muted-foreground">
                <span className="font-medium">{t('Subtitles:')}</span>
                {resultData.transcriptPath}
              </p>
            ) : null}
            <a href="/dashboard/tasks" className="mt-1 inline-flex items-center gap-1.5 text-xs text-blue-600 hover:underline">
              <ExternalLink className="h-3 w-3" />
              {t('View the full steps in the task queue')}
            </a>
          </div>

          {showUpgradePromptCard ? (
            <UpgradePromptCard
              badge={t('Keep scaling output')}
              title={tier === 'free' ? t('The first trial already worked. Upgrading now is the natural next step.') : t('Before the next batch, prepare enough credits and permissions.')}
              description={tier === 'free'
                ? t('Membership unlocks AI metadata, subtitle translation, batch tasks, and higher limits together, so the next run is less likely to be interrupted by credits or permissions.')
                : t('The current account has {balance} credits available. The next full video run may need about {required} credits, so topping up early helps avoid interruptions.', { balance: credits.balance, required: FULL_PIPELINE_REQUIRED_CREDITS })}
              highlights={tier === 'free'
                ? [t('AI metadata generation'), t('Automatic AI subtitle translation'), t('Higher task limits')]
                : [t('{balance} credits available', { balance: credits.balance }), t('Batch tasks are more stable'), t('Credits arrive immediately after payment')]}
              primaryHref={`/membership?source=video-success&recommended=${successRecommendedPlanId}`}
              primaryLabel={tier === 'free' ? t('Upgrade and continue') : t('Top up credits')}
              secondaryHref="/membership"
              secondaryLabel={t('View all plans')}
              dismissible
              onDismiss={() => setShowUpgradePromptCard(false)}
            />
          ) : null}
        </div>
      ) : null}

      <div className="flex items-center gap-3">
        {(phase === 'idle' || phase === 'error') && file ? (
          <button onClick={handleProcess} className="flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90">
            <Play className="h-4 w-4" />
            {t('Start processing')}
          </button>
        ) : null}
        {(phase === 'done' || phase === 'error') ? (
          <button onClick={reset} className="rounded-lg border border-border px-4 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground">
            {t('Upload another file')}
          </button>
        ) : null}
        {file && (phase === 'idle' || phase === 'error') ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <File className="h-4 w-4" />
            <span>{file.name}</span>
            <span className="text-xs">({formatSize(file.size)})</span>
          </div>
        ) : null}
      </div>

      <p className="text-xs text-muted-foreground">
        {t('After upload, the local task chain continues with the current user configuration: audio extraction, subtitle transcription, AI translation, subtitle voiceover, and saving results. Disabled optional steps are skipped automatically. A full run can use up to about {credits} credits, and total processing time depends on the video length.', { credits: FULL_PIPELINE_REQUIRED_CREDITS })}
      </p>
    </div>
  );
}
