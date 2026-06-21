import { useEffect, useMemo, useRef, useState } from 'react';
import { useI18n } from '@/contexts/I18nContext';

export interface SlashCommand {
  id: string;
  icon: string;
  label: string;
  desc: string;
  template: string;
  category: string;
}

interface SlashCommandDefinition {
  id: string;
  icon: string;
  labelKey: string;
  descKey: string;
  templateKey: string;
  categoryKey: string;
}

const SLASH_COMMAND_DEFINITIONS: SlashCommandDefinition[] = [
  { id: 'pipeline', icon: '🚀', labelKey: 'Full processing pipeline', categoryKey: 'Video processing', descKey: 'Download → transcribe → translate → generate metadata in one flow', templateKey: 'Submit this video to the full pipeline: ' },
  { id: 'download', icon: '⬇️', labelKey: 'Download video', categoryKey: 'Video processing', descKey: 'Download a YouTube or Douyin video locally', templateKey: 'Download this video: ' },
  { id: 'audio', icon: '🎵', labelKey: 'Extract audio', categoryKey: 'Video processing', descKey: 'Extract the audio track from a local video file', templateKey: 'Extract the audio from this video, video path: ' },
  { id: 'thumbnail', icon: '🖼️', labelKey: 'Download cover', categoryKey: 'Video processing', descKey: 'Download the high-resolution video cover image', templateKey: 'Download the cover for this video: ' },
  { id: 'query_all', icon: '🔍', labelKey: 'Query video library', categoryKey: 'Video library', descKey: 'List all local videos', templateKey: 'Query my video library' },
  { id: 'query_fail', icon: '❌', labelKey: 'View failed tasks', categoryKey: 'Video library', descKey: 'List videos that failed processing', templateKey: 'Show the most recently failed videos' },
  { id: 'query_done', icon: '✅', labelKey: 'View completed tasks', categoryKey: 'Video library', descKey: 'List videos that finished processing', templateKey: 'Show the most recently completed videos' },
  { id: 'query_bili', icon: '📤', labelKey: 'Uploaded to Bilibili', categoryKey: 'Video library', descKey: 'List videos already uploaded to Bilibili', templateKey: 'Show videos uploaded to Bilibili' },
  { id: 'rewrite', icon: '✏️', labelKey: 'Rewrite Bilibili title/description', categoryKey: 'AI content', descKey: 'Regenerate metadata with AI and save it', templateKey: 'Rewrite the Bilibili title for video [VIDEO_ID], ' },
  { id: 'summarize', icon: '📝', labelKey: 'Summarize video content', categoryKey: 'AI content', descKey: 'Read subtitles and generate a key-point summary', templateKey: 'Summarize the content of video [VIDEO_ID]' },
  { id: 'translate', icon: '🌐', labelKey: 'Translate subtitles', categoryKey: 'AI content', descKey: 'Translate subtitles into the target language', templateKey: 'Translate the subtitles of video [VIDEO_ID] into Chinese' },
  { id: 'sub_list', icon: '📺', labelKey: 'List subscribed channels', categoryKey: 'Subscription management', descKey: 'View all subscribed channels', templateKey: 'List all channels I am subscribed to' },
  { id: 'sub_add', icon: '➕', labelKey: 'Add channel subscription', categoryKey: 'Subscription management', descKey: 'Subscribe to a YouTube channel', templateKey: 'Subscribe to channel [CHANNEL_ID_OR_NAME]' },
  { id: 'sub_remove', icon: '➖', labelKey: 'Remove channel subscription', categoryKey: 'Subscription management', descKey: 'Unsubscribe from a specific channel', templateKey: 'Unsubscribe from channel [CHANNEL_ID]' },
];

export function useSlashCommands(
  input: string,
  setInput: (v: string) => void,
  inputRef: React.RefObject<HTMLTextAreaElement | null>,
) {
  const { t } = useI18n();
  const [slashOpen, setSlashOpen]   = useState(false);
  const [slashQuery, setSlashQuery] = useState('');
  const [slashIndex, setSlashIndex] = useState(0);
  const slashStartRef = useRef(-1);
  const slashMenuRef  = useRef<HTMLDivElement>(null);

  const slashCommands = useMemo<SlashCommand[]>(() => (
    SLASH_COMMAND_DEFINITIONS.map((definition) => ({
      id: definition.id,
      icon: definition.icon,
      label: t(definition.labelKey),
      desc: t(definition.descKey),
      template: t(definition.templateKey),
      category: t(definition.categoryKey),
    }))
  ), [t]);

  const filteredSlash = slashCommands.filter(cmd =>
    !slashQuery ||
    cmd.label.toLowerCase().includes(slashQuery) ||
    cmd.desc.toLowerCase().includes(slashQuery) ||
    cmd.id.includes(slashQuery),
  );

  const selectCommand = (cmd: SlashCommand) => {
    const start = slashStartRef.current;
    if (start < 0) return;
    const before = input.slice(0, start);
    const after  = input.slice(start + 1 + slashQuery.length);
    setInput(before + cmd.template + after);
    setSlashOpen(false);
    slashStartRef.current = -1;
    setTimeout(() => {
      const pos = before.length + cmd.template.length;
      inputRef.current?.focus();
      inputRef.current?.setSelectionRange(pos, pos);
    }, 0);
  };

  const handleInputChange = (val: string, cursor: number) => {
    const textBefore = val.slice(0, cursor);
    const lastSlash  = textBefore.lastIndexOf('/');
    if (lastSlash >= 0) {
      const afterSlash = textBefore.slice(lastSlash + 1);
      const beforeChar = lastSlash === 0 ? '' : val[lastSlash - 1];
      const validStart = lastSlash === 0 || /\s/.test(beforeChar);
      if (validStart && !/\s/.test(afterSlash)) {
        slashStartRef.current = lastSlash;
        setSlashQuery(afterSlash.toLowerCase());
        setSlashIndex(0);
        setSlashOpen(true);
        return;
      }
    }
    setSlashOpen(false);
    slashStartRef.current = -1;
  };

  // Close on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (slashMenuRef.current && !slashMenuRef.current.contains(e.target as Node)) {
        setSlashOpen(false);
      }
    };
    if (slashOpen) document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [slashOpen]);

  // Scroll selected item into view
  useEffect(() => {
    if (!slashOpen) return;
    slashMenuRef.current
      ?.querySelector<HTMLElement>('[data-selected="true"]')
      ?.scrollIntoView({ block: 'nearest' });
  }, [slashIndex, slashOpen]);

  return {
    slashOpen, setSlashOpen,
    slashIndex, setSlashIndex,
    filteredSlash,
    selectCommand,
    handleInputChange,
    slashMenuRef,
  };
}
