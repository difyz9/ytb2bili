'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useI18n } from '@/contexts/I18nContext';
import { useAuth } from '@/contexts/AuthContext';
import { useMembership } from '@/hooks/useMembership';
import { useAiModelCatalog } from '@/hooks/useAiModelCatalog';
import { Sparkles, Upload, Settings, AlertCircle, X, Crown, Check, ChevronsUpDown } from 'lucide-react';
import { agentApi, AgentInfo } from '@/lib/api/agent';
import { TIER_META, highestTier } from '@/lib/agent-models';
import { useUserSettings } from '@/hooks/useUserSettings';
import {
  DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
  parsePlaylistSubmissionConfig,
  USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG,
} from '@/lib/video-submission';
import LocalVideoUpload from '../../../components/LocalVideoUpload';
import ChatMessages from './_components/ChatMessages';
import ChatInput from './_components/ChatInput';
import { useChatMessages } from './_components/hooks/useChatMessages';


const ASSISTANT_MODEL_STORAGE_KEY = 'ytb2bili:assistant:selected-model';

function buildAssistantModelStorageKey(userId?: string): string {
  return `${ASSISTANT_MODEL_STORAGE_KEY}:${userId || 'anonymous'}`;
}

function readAssistantModel(userId?: string): string | null {
  if (typeof window === 'undefined') return null;

  try {
    return localStorage.getItem(buildAssistantModelStorageKey(userId));
  } catch {
    return null;
  }
}

function writeAssistantModel(userId: string | undefined, value: string) {
  if (typeof window === 'undefined' || !value) return;

  try {
    localStorage.setItem(buildAssistantModelStorageKey(userId), value);
  } catch {
    // Ignore local persistence failures and keep the in-memory selection.
  }
}

type SelectableModel = {
  id: string;
  label: string;
  description: string;
  minTier: string;
};

function AssistantModelSelector({
  models,
  selectedModel,
  onChange,
  className,
}: {
  models: SelectableModel[];
  selectedModel: string;
  onChange: (value: string) => void;
  className?: string;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const selected = useMemo(
    () => models.find((item) => item.id === selectedModel),
    [models, selectedModel],
  );

  useEffect(() => {
    if (!open) return;

    const handlePointerDown = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleEscape);

    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [open]);

  return (
    <div ref={containerRef} className={className ? `relative ${className}` : 'relative'}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="flex h-9 w-full min-w-[11.75rem] items-center justify-between rounded-xl border border-slate-200 bg-white px-3 text-left text-sm font-semibold text-slate-900 shadow-sm transition-colors hover:border-slate-300 hover:bg-slate-50"
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate">{selected?.label ?? t('Select model')}</span>
        <ChevronsUpDown className="ml-3 h-4 w-4 shrink-0 text-slate-500" />
      </button>

      {open && (
        <div className="absolute right-0 top-full z-30 mt-1.5 w-72 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_18px_40px_rgba(15,23,42,0.12)]">
          <div className="max-h-80 overflow-y-auto p-1.5">
            {models.map((item) => {
              const active = item.id === selectedModel;

              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => {
                    onChange(item.id);
                    setOpen(false);
                  }}
                  className={`flex w-full items-start gap-2.5 rounded-xl px-3 py-2 text-left transition-colors ${active ? 'bg-slate-100 text-slate-900' : 'bg-white text-slate-900 hover:bg-slate-50'}`}
                  role="option"
                  aria-selected={active}
                >
                  <span className="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center">
                    {active ? <Check className="h-4 w-4 text-amber-500" /> : null}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{item.label}</span>
                    <span className="mt-0.5 block text-[11px] leading-snug text-slate-500">{item.description}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

export default function AssistantPage() {
  const { locale, t } = useI18n();
  const { user, currentUser } = useAuth();
  const assistantUserId = currentUser?.id ?? user?.uid ?? '';
  const { tier, credits, refresh: refreshMembership } = useMembership();
  const { models: modelCatalog, resolvedTier: workerTier, refresh: refreshModelCatalog } = useAiModelCatalog({
    onlyAvailable: true,
    mergeFallback: false,
  });
  const { settings, updateSettings } = useUserSettings(assistantUserId);
  const playlistSubmissionConfig = useMemo(
    () => parsePlaylistSubmissionConfig(settings[USER_SETTING_KEY_PLAYLIST_SUBMISSION_CONFIG]) ?? DEFAULT_PLAYLIST_SUBMISSION_CONFIG,
    [settings],
  );
  const { messages, isLoading, sendMessage, updateTrackingStatus, deleteMessage } = useChatMessages(
    assistantUserId,
    playlistSubmissionConfig,
  );
  const [agentInfo, setAgentInfo] = useState<AgentInfo | null>(null);
  const [showUploadPanel, setShowUploadPanel] = useState(false);
  const [selectedModel, setSelectedModel] = useState('gpt-4o');
  const pendingSelectedModelRef = useRef<string | null>(null);

  const refreshAgentInfo = useCallback(() => {
    agentApi.getInfo().then(setAgentInfo).catch(() => setAgentInfo({ available: false, message: t('Unable to connect to the server') }));
  }, [t]);

  useEffect(() => {
    refreshAgentInfo();
  }, [refreshAgentInfo]);

  useEffect(() => {
    const syncAssistantAccess = () => {
      refreshMembership();
      refreshModelCatalog();
      refreshAgentInfo();
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        syncAssistantAccess();
      }
    };

    window.addEventListener('focus', syncAssistantAccess);
    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      window.removeEventListener('focus', syncAssistantAccess);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [refreshAgentInfo, refreshMembership, refreshModelCatalog]);

  const agentNotAvailable = agentInfo !== null && !agentInfo.available;
  const effectiveTier = highestTier(tier, workerTier, agentInfo?.membership_tier);
  const tierMeta = TIER_META[effectiveTier];
  const assistantPromptHref = `/membership?source=assistant-upsell&recommended=${effectiveTier === 'free' ? 'vip_month' : 'vip_7d'}`;
  const selectableModels = useMemo<SelectableModel[]>(() => modelCatalog.map((item) => ({
    id: item.id,
    label: item.label,
    description: item.description,
    minTier: item.minTier,
  })), [modelCatalog]);
  const selectableModelMap = useMemo(
    () => new Map(selectableModels.map((item) => [item.id, item])),
    [selectableModels],
  );
  const isSelectableModel = useCallback(
    (value: string) => selectableModelMap.has(value),
    [selectableModelMap],
  );

  const persistSelectedModel = useCallback(async (value: string) => {
    const selected = selectableModelMap.get(value);
    if (!selected) return;

    await updateSettings({
      preferred_ai_model: selected.id,
      preferred_ai_model_name: selected.label,
    });
  }, [selectableModelMap, updateSettings]);

  const isRemoteModelSynced = useCallback((value: string) => {
    const selected = selectableModelMap.get(value);
    if (!selected) return false;

    return settings.preferred_ai_model === selected.id && settings.preferred_ai_model_name === selected.label;
  }, [selectableModelMap, settings.preferred_ai_model, settings.preferred_ai_model_name]);

  const handleModelPersistError = useCallback((failedValue: string) => {
    if (pendingSelectedModelRef.current === failedValue) {
      pendingSelectedModelRef.current = null;
    }
    toast.error(t('Failed to save the default model'), { id: 'assistant-model-save-failed' });
  }, [t]);

  useEffect(() => {
    const pendingSelectedModel = pendingSelectedModelRef.current;
    if (pendingSelectedModel) {
      if (isRemoteModelSynced(pendingSelectedModel)) {
        pendingSelectedModelRef.current = null;
      } else {
        return;
      }
    }

    const remoteModel = settings.preferred_ai_model;
    const localModel = readAssistantModel(assistantUserId);
    const fallbackModel = isSelectableModel('gpt-4o') ? 'gpt-4o' : selectableModels[0]?.id ?? 'gpt-4o';
    const nextModel = remoteModel && isSelectableModel(remoteModel)
      ? remoteModel
      : localModel && isSelectableModel(localModel)
        ? localModel
        : fallbackModel;

    setSelectedModel((currentValue) => (currentValue === nextModel ? currentValue : nextModel));
  }, [assistantUserId, isRemoteModelSynced, isSelectableModel, selectableModels, settings.preferred_ai_model]);

  const handleModelChange = useCallback((value: string) => {
    if (!isSelectableModel(value) || value === selectedModel) return;

    pendingSelectedModelRef.current = value;
    setSelectedModel(value);
    writeAssistantModel(assistantUserId, value);

    void persistSelectedModel(value).catch(() => {
      handleModelPersistError(value);
    });
  }, [assistantUserId, handleModelPersistError, isSelectableModel, persistSelectedModel, selectedModel]);

  const selectedModelMeta = selectableModelMap.get(selectedModel);

  const modelLabel = (model: { label: string; description: string } | undefined) => model ? `${model.label} · ${model.description}` : t('Default model');

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)] max-w-4xl mx-auto">
      <div className="flex items-center justify-between p-6 border-b border-border">
        <div className="flex items-center space-x-3">
          <div className="p-2 bg-primary/10 rounded-lg"><Sparkles className="h-6 w-6 text-primary" /></div>
          <div>
            <h1 className="text-2xl font-bold">{t('AI Assistant')}</h1>
            <p className="text-sm text-muted-foreground">
              {t('Welcome, {name}', { name: currentUser?.display_name || user?.displayName || t('User') })}
              {agentInfo?.available && agentInfo.name && (
                <span className="ml-2 text-green-500 text-xs">● {agentInfo.name}</span>
              )}
            </p>
          </div>
        </div>
        <div className="flex items-center space-x-2">
          {selectableModels.length > 0 && (
            <div className="hidden md:flex items-center gap-2 px-1 py-0.5">
              <div className="flex items-center gap-1 text-xs font-medium text-slate-500">
                <Crown className="h-3.5 w-3.5 text-amber-500" />
                <span>{effectiveTier.toUpperCase()}</span>
              </div>
              <AssistantModelSelector
                models={selectableModels}
                selectedModel={selectedModel}
                onChange={handleModelChange}
                className="w-52"
              />
            </div>
          )}
          <button onClick={() => setShowUploadPanel(v => !v)} title={t('Upload local video')} className={`p-2 rounded-lg transition-colors ${showUploadPanel ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}`}><Upload className="h-5 w-5" /></button>
          <button className="p-2 hover:bg-accent rounded-lg transition-colors"><Settings className="h-5 w-5" /></button>
        </div>
      </div>
      {selectableModels.length > 0 && (
        <div className="mx-6 mt-4 rounded-2xl border border-slate-200 bg-white px-4 py-2.5 shadow-sm md:hidden">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-xs text-slate-500">{t('Current membership tier')}</p>
              <p className="text-sm font-semibold text-slate-900">{tierMeta.label}</p>
            </div>
            <AssistantModelSelector
              models={selectableModels}
              selectedModel={selectedModel}
              onChange={handleModelChange}
              className="w-52"
            />
          </div>
          <p className="mt-2 text-xs text-muted-foreground">{modelLabel(selectedModelMeta)}</p>
        </div>
      )}
      {showUploadPanel && (
        <div className="mx-6 mt-4 rounded-xl border border-border bg-card">
          <div className="flex items-center justify-between px-4 py-3 border-b border-border">
            <div className="flex items-center gap-2">
              <Upload className="h-4 w-4 text-primary" /><span className="font-semibold text-sm">{t('Upload local video')}</span><span className="text-xs text-muted-foreground">{t('Extract audio · Transcribe subtitles · AI translation')}</span>
            </div>
            <button onClick={() => setShowUploadPanel(false)} className="p-1 hover:bg-accent rounded transition-colors" title={t('Close')}><X className="h-4 w-4 text-muted-foreground" /></button>
          </div>
          <div className="p-4"><LocalVideoUpload userId={user?.uid} /></div>
        </div>
      )}
      {agentNotAvailable && (
        <div className="mx-6 mt-4 flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 px-4 py-3 text-sm text-amber-800 dark:text-amber-300">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
          <span>{agentInfo?.message ?? t('AI Assistant is currently unavailable. Please try again later.')}</span>
        </div>
      )}
      <ChatMessages messages={messages} isLoading={isLoading} onSuggestedPrompt={(prompt) => sendMessage(prompt, selectedModel)} disabled={agentNotAvailable} onTrackingStatusChange={updateTrackingStatus} onDeleteMessage={deleteMessage} />
   
      <ChatInput onSend={(text) => sendMessage(text, selectedModel)} disabled={agentNotAvailable} placeholder={agentNotAvailable ? t('AI Assistant is not configured. Check config.toml ...') : undefined} />
    </div>
  );
}
